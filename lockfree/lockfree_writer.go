// lockfree_writer.go - 使用环形缓冲区的高性能写入器
// 提供无锁并发写入能力，适用于高并发日志场景
package lockfree

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// LockFreeBufferedWriter 使用环形缓冲区的高性能写入器
type LockFreeBufferedWriter struct {
	// 底层写入器
	writer io.WriteCloser

	// 环形缓冲区
	ringBuffer *RingBuffer

	// 后台刷新协程控制
	flushChan   chan struct{}
	closeChan   chan struct{}
	flushTicker *time.Ticker
	wg          sync.WaitGroup

	// 配置参数
	flushInterval time.Duration
	batchSize     int
	maxRetries    int

	// 状态标志
	closed      atomic.Bool
	flushActive atomic.Bool
}

// LockFreeWriterConfig 配置结构
type LockFreeWriterConfig struct {
	BufferSize    int           // 环形缓冲区大小
	FlushInterval time.Duration // 刷新间隔
	BatchSize     int           // 批量刷新大小
	MaxRetries    int           // 最大重试次数
}

// DefaultLockFreeWriterConfig 返回默认配置
func DefaultLockFreeWriterConfig() *LockFreeWriterConfig {
	return &LockFreeWriterConfig{
		BufferSize:    1024 * 1024, // 1MB缓冲区
		FlushInterval: time.Second, // 1秒刷新间隔
		BatchSize:     4096,        // 4KB批量大小
		MaxRetries:    3,           // 最大重试3次
	}
}

// NewLockFreeBufferedWriter 创建新的无锁缓冲写入器
func NewLockFreeBufferedWriter(writer io.WriteCloser, config *LockFreeWriterConfig) *LockFreeBufferedWriter {
	if config == nil {
		config = DefaultLockFreeWriterConfig()
	}

	// 确保缓冲区大小是2的幂次方
	bufferSize := RoundUpToPowerOfTwo(config.BufferSize)

	return &LockFreeBufferedWriter{
		writer:        writer,
		ringBuffer:    NewRingBuffer(bufferSize),
		flushChan:     make(chan struct{}, 1),
		closeChan:     make(chan struct{}),
		flushTicker:   time.NewTicker(config.FlushInterval),
		flushInterval: config.FlushInterval,
		batchSize:     config.BatchSize,
		maxRetries:    config.MaxRetries,
	}
}

// Start 启动后台刷新协程
func (lfw *LockFreeBufferedWriter) Start() {
	if !lfw.flushActive.CompareAndSwap(false, true) {
		return // 已经启动
	}

	lfw.wg.Add(1)
	go lfw.flushWorker()
}

// Write 实现 io.Writer 接口
func (lfw *LockFreeBufferedWriter) Write(p []byte) (n int, err error) {
	if lfw.closed.Load() {
		return 0, fmt.Errorf("writer is closed")
	}

	// 尝试写入环形缓冲区
	n, err = lfw.ringBuffer.Write(p)
	if err != nil {
		// 缓冲区满，触发立即刷新
		lfw.triggerFlush()

		// 重试写入
		n, err = lfw.ringBuffer.Write(p)
		if err != nil {
			// 仍然失败，直接写入底层
			return lfw.writer.Write(p)
		}
	}

	// 如果缓冲区使用量超过阈值，触发刷新
	if lfw.ringBuffer.Used() > lfw.batchSize {
		lfw.triggerFlush()
	}

	return n, nil
}

// WriteString 写入字符串，避免内存分配
func (lfw *LockFreeBufferedWriter) WriteString(s string) (int, error) {
	return lfw.Write([]byte(s))
}

// triggerFlush 触发刷新，非阻塞
func (lfw *LockFreeBufferedWriter) triggerFlush() {
	select {
	case lfw.flushChan <- struct{}{}:
	default:
		// 通道已满，说明已经有刷新请求在处理
	}
}

// flushWorker 后台刷新协程
func (lfw *LockFreeBufferedWriter) flushWorker() {
	defer lfw.wg.Done()
	defer lfw.flushActive.Store(false)

	// 临时缓冲区，大小与批量大小一致
	tempBuf := make([]byte, lfw.batchSize)

	for {
		select {
		case <-lfw.flushChan:
			// 立即刷新
			lfw.flushBuffer(tempBuf)

		case <-lfw.flushTicker.C:
			// 定时刷新
			if lfw.ringBuffer.Used() > 0 {
				lfw.flushBuffer(tempBuf)
			}

		case <-lfw.closeChan:
			// 关闭信号，最后刷新一次
			lfw.flushBuffer(tempBuf)
			return
		}
	}
}

// flushBuffer 刷新缓冲区数据到底层写入器
func (lfw *LockFreeBufferedWriter) flushBuffer(tempBuf []byte) {
	// 防止并发刷新
	if !lfw.flushActive.CompareAndSwap(false, true) {
		return
	}
	defer lfw.flushActive.Store(false)

	retryCount := 0

	for lfw.ringBuffer.Used() > 0 && retryCount < lfw.maxRetries {
		n, err := lfw.ringBuffer.Read(tempBuf)
		if err != nil || n == 0 {
			break
		}

		// 写入底层，带重试机制
		writeErr := lfw.writeWithRetry(tempBuf[:n])
		if writeErr != nil {
			// 写入失败，记录错误但继续尝试
			fmt.Printf("flush error: %v\n", writeErr)
			retryCount++
		} else {
			// 写入成功，重置重试计数
			retryCount = 0
		}
	}
}

// writeWithRetry 带重试的写入操作
func (lfw *LockFreeBufferedWriter) writeWithRetry(data []byte) error {
	var lastErr error

	for i := 0; i < lfw.maxRetries; i++ {
		_, err := lfw.writer.Write(data)
		if err == nil {
			return nil // 写入成功
		}

		lastErr = err

		// 短暂休眠后重试
		time.Sleep(time.Millisecond * time.Duration(i+1))
	}

	return lastErr
}

// Flush 手动刷新缓冲区
func (lfw *LockFreeBufferedWriter) Flush() error {
	if lfw.closed.Load() {
		return fmt.Errorf("writer is closed")
	}

	// 触发刷新并等待完成
	lfw.triggerFlush()

	// 等待缓冲区清空
	maxWait := time.Second * 5
	startTime := time.Now()

	for lfw.ringBuffer.Used() > 0 && time.Since(startTime) < maxWait {
		time.Sleep(time.Millisecond * 10)
	}

	// 如果底层写入器支持Flush，调用它
	if flusher, ok := lfw.writer.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}

	return nil
}

// Stats 返回写入器的统计信息
type WriterStats struct {
	BufferSize      int  // 缓冲区总大小
	BufferUsed      int  // 已使用的缓冲区空间
	BufferAvailable int  // 可用的缓冲区空间
	IsClosed        bool // 是否已关闭
}

// Stats 返回写入器的统计信息
func (lfw *LockFreeBufferedWriter) Stats() WriterStats {
	stats := lfw.ringBuffer.Stats()
	return WriterStats{
		BufferSize:      stats.Size,
		BufferUsed:      stats.Used,
		BufferAvailable: stats.Available,
		IsClosed:        lfw.closed.Load(),
	}
}

// Close 关闭写入器
func (lfw *LockFreeBufferedWriter) Close() error {
	if !lfw.closed.CompareAndSwap(false, true) {
		return nil // 已经关闭
	}

	// 停止定时器
	lfw.flushTicker.Stop()

	// 发送关闭信号
	close(lfw.closeChan)

	// 等待后台协程退出
	lfw.wg.Wait()

	// 最后一次刷新
	tempBuf := make([]byte, lfw.batchSize)
	lfw.flushBuffer(tempBuf)

	// 关闭底层写入器
	return lfw.writer.Close()
}

// SetFlushInterval 动态调整刷新间隔
func (lfw *LockFreeBufferedWriter) SetFlushInterval(interval time.Duration) {
	lfw.flushInterval = interval
	lfw.flushTicker.Stop()
	lfw.flushTicker = time.NewTicker(interval)
}

// SetBatchSize 动态调整批量大小
func (lfw *LockFreeBufferedWriter) SetBatchSize(size int) {
	lfw.batchSize = size
}

// AdaptiveBuffer 自适应缓冲区写入器
// 根据写入频率动态调整缓冲策略
type AdaptiveBuffer struct {
	*LockFreeBufferedWriter

	// 自适应参数
	writeInterval  time.Duration // 平均写入间隔
	lastWriteTime  time.Time     // 上次写入时间
	writeCount     int64         // 写入次数统计
	adjustInterval time.Duration // 调整间隔
	lastAdjustTime time.Time     // 上次调整时间
}

// NewAdaptiveBuffer 创建自适应缓冲写入器
func NewAdaptiveBuffer(writer io.WriteCloser, config *LockFreeWriterConfig) *AdaptiveBuffer {
	lfw := NewLockFreeBufferedWriter(writer, config)
	return &AdaptiveBuffer{
		LockFreeBufferedWriter: lfw,
		writeInterval:          time.Millisecond * 100, // 初始值
		adjustInterval:         time.Second * 10,       // 每10秒调整一次
	}
}

// Write 自适应写入
func (ab *AdaptiveBuffer) Write(p []byte) (int, error) {
	now := time.Now()

	// 更新写入统计
	if !ab.lastWriteTime.IsZero() {
		interval := now.Sub(ab.lastWriteTime)
		// 使用指数移动平均计算平均间隔
		ab.writeInterval = time.Duration(float64(ab.writeInterval)*0.9 + float64(interval)*0.1)
	}
	ab.lastWriteTime = now
	atomic.AddInt64(&ab.writeCount, 1)

	// 检查是否需要调整策略
	if now.Sub(ab.lastAdjustTime) > ab.adjustInterval {
		ab.adjustStrategy()
		ab.lastAdjustTime = now
	}

	// 调用底层写入
	return ab.LockFreeBufferedWriter.Write(p)
}

// adjustStrategy 根据写入模式调整缓冲策略
func (ab *AdaptiveBuffer) adjustStrategy() {
	// 根据写入频率调整缓冲区大小和刷新间隔
	if ab.writeInterval < time.Millisecond*10 {
		// 高频写入：增加缓冲区大小，减少刷新频率
		ab.SetBatchSize(8192)                // 8KB批量
		ab.SetFlushInterval(time.Second * 5) // 5秒刷新
	} else if ab.writeInterval > time.Second {
		// 低频写入：减少缓冲区大小，增加刷新频率
		ab.SetBatchSize(1024)                       // 1KB批量
		ab.SetFlushInterval(time.Millisecond * 500) // 500ms刷新
	} else {
		// 中频写入：使用默认值
		ab.SetBatchSize(4096)            // 4KB批量
		ab.SetFlushInterval(time.Second) // 1秒刷新
	}
}

// GetWriteInterval 获取平均写入间隔
func (ab *AdaptiveBuffer) GetWriteInterval() time.Duration {
	return ab.writeInterval
}

// GetWriteCount 获取写入次数
func (ab *AdaptiveBuffer) GetWriteCount() int64 {
	return atomic.LoadInt64(&ab.writeCount)
}
