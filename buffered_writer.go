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
	_ io.Writer = (*BufferedWriter)(nil)
	_ io.Closer = (*BufferedWriter)(nil)
)

// BufferedWriter 带缓冲批量写入器
// 可以包装任何写入器和关闭器，提供批量写入功能
type BufferedWriter struct {
	writer io.Writer     // 底层写入器（必需）
	closer io.Closer     // 底层关闭器（必需）
	buffer *bytes.Buffer // 缓冲区
	mutex  sync.RWMutex  // 保护缓冲区和状态（读写锁）

	// 三重刷新条件
	maxBufferSize int           // 最大缓冲区大小（字节）
	maxLogCount   int           // 最大日志条数
	flushInterval time.Duration // 刷新间隔

	// 状态跟踪
	currentCount int       // 当前日志条数
	lastFlush    time.Time // 上次刷新时间
	closed       bool      // 是否已关闭
}

// BufCfg 缓冲写入器配置
type BufCfg struct {
	MaxBufferSize int           // 最大缓冲区大小，默认64KB
	MaxLogCount   int           // 最大日志条数，默认500条
	FlushInterval time.Duration // 刷新间隔，默认1秒
}

// DefBufCfg 默认缓冲写入器配置
//
// 注意:
//   - 默认缓冲区大小为64KB，最大日志条数为500条，刷新间隔为1秒
func DefBufCfg() *BufCfg {
	return &BufCfg{
		MaxBufferSize: 64 * 1024,       // 64KB缓冲区
		MaxLogCount:   500,             // 500条日志
		FlushInterval: 1 * time.Second, // 1秒刷新间隔
	}
}

// NewBW 是 NewBufferedWriter 的简写形式，用于创建新的 BufferedWriter 实例。
var NewBW = NewBufferedWriter

// NewBufferedWriter 创建新的带缓冲批量写入器
func NewBufferedWriter(writer io.Writer, closer io.Closer, config *BufCfg) *BufferedWriter {
	if writer == nil {
		panic("buffered writer: writer cannot be nil")
	}
	if closer == nil {
		panic("buffered writer: closer cannot be nil")
	}
	if config == nil {
		config = DefBufCfg()
	}

	return &BufferedWriter{
		writer:        writer,                                                 // 底层写入器（必需）
		closer:        closer,                                                 // 底层关闭器（必需）
		buffer:        bytes.NewBuffer(make([]byte, 0, config.MaxBufferSize)), // 初始化缓冲区
		maxBufferSize: config.MaxBufferSize,                                   // 最大缓冲区大小（字节）
		maxLogCount:   config.MaxLogCount,                                     // 最大日志条数
		flushInterval: config.FlushInterval,                                   // 刷新间隔
		lastFlush:     time.Now(),                                             // 初始化为当前时间
	}
}

// NewBFL 是 NewBufFromL 的简写形式，用于从 LogRotateX 创建缓冲写入器。
var NewBFL = NewBufFromL

// NewBufFromL 从 LogRotateX 创建缓冲写入器的便捷方法
//
// 参数:
//   - logger: LogRotateX 实例
//   - config: 缓冲写入器配置（可选）
//
// 返回值:
//   - *BufferedWriter: 配置好的缓冲写入器
func NewBufFromL(logger *LogRotateX, config *BufCfg) *BufferedWriter {
	return NewBufferedWriter(logger, logger, config)
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

	// 2. 增加日志计数
	bw.currentCount++

	// 3. 检查是否需要刷新（三重条件触发）
	if bw.shouldFlush() {
		return n, bw.flushLocked()
	}

	return n, nil
}

// shouldFlush 检查是否应该刷新缓冲区
// 三重条件：缓冲区大小 OR 日志条数 OR 刷新间隔
func (bw *BufferedWriter) shouldFlush() bool {
	// 先检查大小和数量条件，避免不必要的时间计算
	if bw.buffer.Len() >= bw.maxBufferSize || bw.currentCount >= bw.maxLogCount {
		return true
	}
	// 只有在前两个条件都不满足时才检查时间
	return time.Since(bw.lastFlush) >= bw.flushInterval
}

// flushLocked 刷新缓冲区
func (bw *BufferedWriter) flushLocked() error {
	if bw.buffer.Len() == 0 {
		return nil
	}

	// 一次性写入所有数据到底层写入器
	_, err := bw.writer.Write(bw.buffer.Bytes())
	if err != nil {
		return err
	}

	// 重置缓冲区和计数器
	bw.buffer.Reset()
	bw.currentCount = 0
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
		return nil
	}

	bw.closed = true

	// 关闭前最后一次刷新，确保数据不丢失
	err := bw.flushLocked()

	// 关闭底层写入器
	if bw.closer != nil {
		if closeErr := bw.closer.Close(); closeErr != nil && err == nil {
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

// LogCount 返回当前缓冲区中的日志条数
func (bw *BufferedWriter) LogCount() int {
	bw.mutex.RLock()
	defer bw.mutex.RUnlock()
	return bw.currentCount
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
