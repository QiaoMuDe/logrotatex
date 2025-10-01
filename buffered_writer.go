/*
buffered_writer.go - 带缓冲批量写入器
实现简洁高效的批量写入优化，通过三重条件触发减少系统调用开销。
*/
package logrotatex

import (
	"bytes"
	"io"
	"sync"
	"time"
)

// 编译时接口实现检查
var (
	_ io.WriteCloser = (*BufferedWriter)(nil)
)

// BufferedWriter 带缓冲批量写入器
// 可以包装任何写入器和关闭器，提供批量写入功能
type BufferedWriter struct {
	wc     io.WriteCloser // 底层写入+关闭器（必需）
	buffer *bytes.Buffer  // 缓冲区
	mutex  sync.RWMutex   // 保护缓冲区和状态（读写锁）

	// 三重刷新条件
	maxBufferSize int           // 最大缓冲区大小（字节），默认64KB (0 表示禁用缓冲区大小触发条件)
	maxWriteCount int           // 最大写入次数，默认500次 (0 表示禁用写入次数触发条件)
	flushInterval time.Duration // 刷新间隔，默认1秒 (0 表示禁用刷新间隔触发条件)

	// 状态跟踪
	writeCount int       // 当前写入次数
	lastFlush  time.Time // 上次刷新时间
	closed     bool      // 是否已关闭
}

// BufCfg 缓冲写入器配置
type BufCfg struct {
	MaxBufferSize int           // 最大缓冲区大小，默认64KB (0 表示禁用缓冲区大小触发条件)
	MaxWriteCount int           // 最大写入次数，默认500次 (0 表示禁用写入次数触发条件)
	FlushInterval time.Duration // 刷新间隔，默认1秒 (0 表示禁用刷新间隔触发条件)
}

// DefBufCfg 默认缓冲写入器配置
//
// 注意:
//   - 默认缓冲区大小为64KB，最大写入次数为500次，刷新间隔为1秒
func DefBufCfg() *BufCfg {
	return &BufCfg{
		MaxBufferSize: 64 * 1024,       // 64KB缓冲区
		MaxWriteCount: 500,             // 500次写入
		FlushInterval: 1 * time.Second, // 1秒刷新间隔
	}
}

// NewBW 是 NewBufferedWriter 的简写形式，用于创建新的 BufferedWriter 实例。
var NewBW = NewBufferedWriter

// NewBufferedWriter 创建新的带缓冲批量写入器
//
// 参数:
//   - wc: 底层写入+关闭器（必需）
//   - config: 配置（可选，如果为空，使用默认值）
//
// 返回值:
//   - *BufferedWriter: 新的带缓冲批量写入器实例
func NewBufferedWriter(wc io.WriteCloser, config *BufCfg) *BufferedWriter {
	// 校验参数：WriteCloser 不能为空
	if wc == nil {
		panic("logrotatex: WriteCloser cannot be nil")
	}
	if config == nil {
		config = DefBufCfg() // 配置如果为空，使用默认值
	}

	// 严格校验：非法值直接 panic，快速失败
	if config.MaxBufferSize < 0 {
		panic("logrotatex: MaxBufferSize must be >= 0")
	}
	if config.MaxWriteCount < 0 {
		panic("logrotatex: MaxWriteCount must be >= 0")
	}
	if config.FlushInterval < 0 {
		panic("logrotatex: FlushInterval must be >= 0")
	}

	return &BufferedWriter{
		wc:            wc,                                                     // 底层写入+关闭器（必需）
		buffer:        bytes.NewBuffer(make([]byte, 0, config.MaxBufferSize)), // 初始化缓冲区
		maxBufferSize: config.MaxBufferSize,                                   // 最大缓冲区大小（字节）
		maxWriteCount: config.MaxWriteCount,                                   // 最大写入次数
		flushInterval: config.FlushInterval,                                   // 刷新间隔
		lastFlush:     time.Now(),                                             // 初始化为当前时间
	}
}

// Write 实现 io.Writer 接口
// 将数据写入缓冲区，达到刷新条件时自动批量写入
//
// 参数:
//   - p: 要写入的数据
//
// 返回值:
//   - n: 实际写入的字节数
//   - err: 写入错误（如果有）
func (bw *BufferedWriter) Write(p []byte) (n int, err error) {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	if bw.closed {
		return 0, io.ErrClosedPipe
	}

	// 1. 写入缓冲区
	n, err = bw.buffer.Write(p)
	if err != nil {
		return n, err
	}

	// 2. 增加写入计数
	bw.writeCount++

	// 3. 检查是否需要刷新（三重条件触发）
	if bw.shouldFlush() {
		return n, bw.flushLocked()
	}

	return n, nil
}

// shouldFlush 检查是否应该刷新缓冲区
//
// 返回值:
//   - bool: 是否应该刷新缓冲区
//
// 注意:
//   - 三重条件：缓冲区大小 OR 写入次数 OR 刷新间隔
//   - 如果满足任意一个条件，则刷新缓冲区
//   - 0 表示禁用对应触发条件
func (bw *BufferedWriter) shouldFlush() bool {
	// 检查是否满足缓冲区更新条件
	if bw.maxBufferSize > 0 && bw.buffer.Len() >= bw.maxBufferSize {
		return true
	}

	// 检查是否满足写入次数更新条件
	if bw.maxWriteCount > 0 && bw.writeCount >= bw.maxWriteCount {
		return true
	}

	// 检查是否满足刷新间隔条件
	if bw.flushInterval > 0 && time.Since(bw.lastFlush) >= bw.flushInterval {
		return true
	}
	return false
}

// flushLocked 刷新缓冲区
func (bw *BufferedWriter) flushLocked() error {
	if bw.buffer.Len() == 0 {
		return nil
	}

	// 使用 bytes.Buffer.WriteTo 来处理部分写入与循环写入
	if _, err := bw.buffer.WriteTo(bw.wc); err != nil {
		// 出错时，WriteTo 已消耗掉已写出的前缀，剩余数据仍保留在缓冲区
		return err
	}

	// 写入成功后，缓冲区已被消费为空，这里 Reset 以确保状态干净
	bw.buffer.Reset()
	bw.writeCount = 0         // 重置写入次数
	bw.lastFlush = time.Now() // 更新刷新时间
	return nil
}

// Flush 手动刷新缓冲区
func (bw *BufferedWriter) Flush() error {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()
	return bw.flushLocked()
}

// Close 关闭缓冲写入器
func (bw *BufferedWriter) Close() error {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	if bw.closed {
		return nil // 已关闭，无需重复操作
	}
	bw.closed = true // 标记为已关闭

	// 关闭前最后一次刷新，确保数据不丢失
	err := bw.flushLocked()

	// 关闭底层写入器
	if bw.wc != nil {
		if closeErr := bw.wc.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}

	return err
}

// BufferSize 返回当前缓冲区中的字节数
func (bw *BufferedWriter) BufferSize() int {
	bw.mutex.RLock()
	defer bw.mutex.RUnlock()
	return bw.buffer.Len()
}

// WriteCount 返回当前缓冲区中的写入次数
func (bw *BufferedWriter) WriteCount() int {
	bw.mutex.RLock()
	defer bw.mutex.RUnlock()
	return bw.writeCount
}

// IsClosed 返回缓冲写入器是否已关闭
func (bw *BufferedWriter) IsClosed() bool {
	bw.mutex.RLock()
	defer bw.mutex.RUnlock()
	return bw.closed
}

// TimeSinceLastFlush 返回距离上次刷新的时间
func (bw *BufferedWriter) TimeSinceLastFlush() time.Duration {
	bw.mutex.RLock()
	defer bw.mutex.RUnlock()
	return time.Since(bw.lastFlush)
}
