/*
buffered_writer.go - 带缓冲批量写入器
实现简洁高效的批量写入优化, 通过三重条件触发减少系统调用开销。
*/
package logrotatex

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// 编译时接口实现检查
var (
	_ io.WriteCloser = (*BufferedWriter)(nil)
)

// 缓冲写入器默认配置常量
const (
	// DefaultMaxBufferSize 默认最大缓冲区大小 (256KB)
	DefaultMaxBufferSize = 256 * 1024
	// DefaultMaxWriteCount 默认最大写入次数 (1000次)
	DefaultMaxWriteCount = 1000
	// DefaultFlushInterval 默认刷新间隔 (1秒)
	DefaultFlushInterval = 1 * time.Second
	// MinFlushInterval 最小刷新间隔 (500ms), 防止过于频繁的刷新
	MinFlushInterval = 500 * time.Millisecond
)

// BufferedWriter 带缓冲批量写入器
// 可以包装任何写入器和关闭器, 提供批量写入功能
type BufferedWriter struct {
	// 内部状态
	wc          io.WriteCloser // 底层写入+关闭器 (必需)
	buffer      *bytes.Buffer  // 缓冲区
	mutex       sync.RWMutex   // 保护缓冲区和状态 (读写锁)
	flushTicker *time.Ticker   // 刷新定时器 (根据 flushInterval 触发)
	closeChan   chan struct{}  // 关闭信号通道, 用于通知定时器协程退出
	tickerWg    sync.WaitGroup // 用于跟踪定时器协程生命周期

	// 三重刷新条件
	maxBufferSize int           // 最大缓冲区大小 (字节), 默认256KB
	maxWriteCount int           // 最大写入次数, 默认1000次
	flushInterval time.Duration // 刷新间隔, 默认1秒 (最小500ms)

	// 状态跟踪
	writeCount  int         // 当前写入次数
	lastFlush   time.Time   // 上次刷新时间
	closed      atomic.Bool // 是否已关闭
	initialized bool        // 是否已初始化
}

// BufCfg 缓冲写入器配置
type BufCfg struct {
	MaxBufferSize int           // 最大缓冲区大小, 默认256KB
	MaxWriteCount int           // 最大写入次数, 默认1000次
	FlushInterval time.Duration // 刷新间隔, 默认1秒(最小500ms)
}

// DefBufCfg 默认缓冲写入器配置
//
// 注意:
//   - 默认缓冲区大小为256KB, 最大写入次数为1000次, 刷新间隔为1秒
func DefBufCfg() *BufCfg {
	return &BufCfg{
		MaxBufferSize: DefaultMaxBufferSize, // 默认256KB缓冲区
		MaxWriteCount: DefaultMaxWriteCount, // 默认1000次写入
		FlushInterval: DefaultFlushInterval, // 默认1秒刷新间隔
	}
}

/*
不可关闭的 WriteCloser 包装器, 用于避免关闭标准输出等不应被关闭的 Writer。
*/
type noCloseWC struct{ io.Writer }

func (noCloseWC) Close() error { return nil }

// WrapWriter 将 io.Writer 包装为不可关闭的 io.WriteCloser (具体类型为 noCloseWC)
//
// 参数:
//   - w: 要包装的 io.Writer
//
// 返回值:
//   - io.WriteCloser: 不可关闭的 WriteCloser 包装器
func WrapWriter(w io.Writer) io.WriteCloser {
	return noCloseWC{w}
}

// NewStdoutBW 创建面向标准输出的带缓冲写入器 (不会关闭 stdout)
// 仅接收配置结构体, 使用 os.Stdout 作为底层输出。
//
// 注意:
//   - 调用 Close() 不会关闭标准输出, 适合长期运行的场景。
func NewStdoutBW(config *BufCfg) *BufferedWriter {
	return NewBufferedWriter(WrapWriter(os.Stdout), config)
}

// NewBW 是 NewBufferedWriter 的简写形式, 用于创建新的 BufferedWriter 实例。
var NewBW = NewBufferedWriter

// DefaultBufferedWriter 返回一个使用默认配置的 BufferedWriter 实例
// 需要传入一个 io.WriteCloser 作为底层写入器
//
// 参数:
//   - wc: 底层写入+关闭器 (必需)
//
// 返回值:
//   - *BufferedWriter: 使用默认配置的带缓冲批量写入器实例
//
// 默认配置:
//   - 缓冲区大小: 256KB
//   - 最大写入次数: 1000次
//   - 刷新间隔: 1秒
func DefaultBufferedWriter(wc io.WriteCloser) *BufferedWriter {
	return NewBufferedWriter(wc, DefBufCfg())
}

// DefaultBuffered 返回一个使用默认配置的 BufferedWriter 实例
// 内部自动创建一个使用默认配置的 LogRotateX 作为底层写入器
//
// 返回值:
//   - *BufferedWriter: 使用默认配置的带缓冲批量写入器实例
//
// 默认配置:
//   - 缓冲区大小: 256KB
//   - 最大写入次数: 1000次
//   - 刷新间隔: 1秒
//   - 日志文件路径: logs/app.log
//   - 按天轮转: 启用
//   - 日期目录: 启用
func DefaultBuffered() *BufferedWriter {
	// 创建默认配置的 LogRotateX
	logger := Default()
	// 使用默认配置创建 BufferedWriter
	return NewBufferedWriter(logger, DefBufCfg())
}

// initDefaults 初始化 BufferedWriter 实例的默认值。
// 该方法确保无论是通过构造函数创建还是直接通过结构体字面量创建，
// 都能获得一致的初始化行为。
// 注意：该方法只会执行一次，避免重复初始化。
//
// 返回值:
//   - error: 初始化失败时返回错误，否则返回 nil
func (bw *BufferedWriter) initDefaults() error {
	// 如果已经初始化过，直接返回
	if bw.initialized {
		return nil
	}

	// 校验参数: WriteCloser 不能为空
	if bw.wc == nil {
		return errors.New("logrotatex: WriteCloser cannot be nil")
	}

	// 参数校验, 负值,零值统一设置为默认值
	if bw.maxBufferSize <= 0 {
		bw.maxBufferSize = DefaultMaxBufferSize // 默认256KB缓冲区
	}
	if bw.maxWriteCount <= 0 {
		bw.maxWriteCount = DefaultMaxWriteCount // 默认1000次写入
	}
	// 设置刷新间隔, 最小500ms, 默认1秒
	if bw.flushInterval > 0 && bw.flushInterval < MinFlushInterval {
		bw.flushInterval = MinFlushInterval // 最小500ms, 防止过于频繁的刷新
	} else if bw.flushInterval <= 0 {
		bw.flushInterval = DefaultFlushInterval // 默认1秒刷新间隔
	}

	// 初始化内部字段
	bw.buffer = bytes.NewBuffer(make([]byte, 0, bw.maxBufferSize)) // 缓冲区
	// 使用零值锁请勿手动初始化
	// bw.mutex = sync.RWMutex{}
	bw.writeCount = 0                  // 写入次数
	bw.lastFlush = time.Now()          // 上次刷新时间
	bw.closed.Store(false)             // 默认设置为未关闭
	bw.closeChan = make(chan struct{}) // 初始化关闭信号通道

	// 启动刷新定时器
	if bw.flushTicker == nil {
		bw.flushTicker = time.NewTicker(bw.flushInterval)

		// 启动定时器协程
		bw.tickerWg.Go(func() {
			// 防止定时器协程panic导致程序崩溃
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("logrotatex: BufferedWriter flush ticker panic: %v stack: %s\n", r, debug.Stack())
				}
			}()

			for {
				select {
				case <-bw.flushTicker.C:
					// 检查是否已关闭
					if bw.closed.Load() {
						return
					}
					// 刷新失败时打印错误日志
					if err := bw.Flush(); err != nil {
						fmt.Printf("logrotatex: ticker flush failed: %v\n", err)
					}
				case <-bw.closeChan:
					// 收到关闭信号，立即退出
					return
				}
			}
		})
	}

	// 标记为已初始化
	bw.initialized = true

	return nil
}

// NewBufferedWriter 创建新的带缓冲批量写入器
//
// 参数:
//   - wc: 底层写入+关闭器 (必需)
//   - config: 配置 (可选, 如果为空, 使用默认值)
//
// 返回值:
//   - *BufferedWriter: 新的带缓冲批量写入器实例
func NewBufferedWriter(wc io.WriteCloser, config *BufCfg) *BufferedWriter {
	// 校验参数: WriteCloser 不能为空
	if wc == nil {
		panic("logrotatex: WriteCloser cannot be nil")
	}

	if config == nil {
		config = DefBufCfg() // 配置如果为空, 使用默认值
	}

	bw := &BufferedWriter{
		wc:            wc,                   // 底层写入+关闭器 (必需)
		maxBufferSize: config.MaxBufferSize, // 最大缓冲区大小 (字节)
		maxWriteCount: config.MaxWriteCount, // 最大写入次数
		flushInterval: config.FlushInterval, // 刷新间隔
	}

	// 初始化默认值
	if err := bw.initDefaults(); err != nil {
		panic(err)
	}

	return bw
}

// Write 实现 io.Writer 接口
// 将数据写入缓冲区, 达到刷新条件时自动批量写入
//
// 参数:
//   - p: 要写入的数据
//
// 返回值:
//   - n: 实际写入的字节数
//   - err: 写入错误 (如果有)
func (bw *BufferedWriter) Write(p []byte) (n int, err error) {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	// 初始化默认值
	if !bw.initialized {
		if err := bw.initDefaults(); err != nil {
			return 0, err
		}
	}

	if bw.closed.Load() {
		return 0, errors.New("logrotatex: write on closed")
	}

	// 1. 写入缓冲区
	n, err = bw.buffer.Write(p)
	if err != nil {
		return n, err
	}

	// 2. 增加写入计数
	bw.writeCount++

	// 3. 检查是否需要刷新 (三重条件触发)
	if bw.shouldFlush() {
		flushErr := bw.flushLocked()
		if flushErr != nil {
			// 数据已写入缓冲区，但刷新失败
			// 返回实际写入的字节数，让调用者知道数据已接收
			return n, fmt.Errorf("logrotatex: write succeeded but flush failed: %w", flushErr)
		}
	}

	return n, nil
}

// shouldFlush 检查是否应该刷新缓冲区
//
// 返回值:
//   - bool: 是否应该刷新缓冲区
//
// 注意:
//   - 三重条件: 缓冲区大小 OR 写入次数 OR 刷新间隔
//   - 如果满足任意一个条件, 则刷新缓冲区
//   - 所有触发条件始终生效
func (bw *BufferedWriter) shouldFlush() bool {
	// 检查是否满足缓冲区更新条件
	if bw.buffer.Len() >= bw.maxBufferSize {
		return true
	}

	// 检查是否满足写入次数更新条件
	if bw.writeCount >= bw.maxWriteCount {
		return true
	}

	// 检查是否满足刷新间隔条件
	if time.Since(bw.lastFlush) >= bw.flushInterval {
		return true
	}
	return false
}

// flushLocked 刷新缓冲区
func (bw *BufferedWriter) flushLocked() error {
	// 如果缓冲区为空, 则无需刷新
	if bw.buffer.Len() == 0 {
		return nil
	}

	// 使用 bytes.Buffer.WriteTo 来处理部分写入与循环写入
	if _, err := bw.buffer.WriteTo(bw.wc); err != nil {
		// 出错时, WriteTo 已消耗掉已写出的前缀, 剩余数据仍保留在缓冲区
		return fmt.Errorf("logrotatex: flush failed: %w", err)
	}

	// 写入成功后, 缓冲区已被消费为空, 无需Reset, WriteTo会自动清空缓冲区
	bw.writeCount = 0         // 重置写入次数
	bw.lastFlush = time.Now() // 更新刷新时间
	return nil
}

// Flush 手动刷新缓冲区, 确保所有数据被写入底层写入器
//
// 返回值:
//   - error: 刷新错误 (如果有)
func (bw *BufferedWriter) Flush() error {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	// 外部触发刷新: 若已关闭则直接返回, 防止重复刷新
	if bw.closed.Load() {
		return errors.New("logrotatex: flush on closed")
	}

	// 初始化默认值
	if !bw.initialized {
		if err := bw.initDefaults(); err != nil {
			return err
		}
	}

	return bw.flushLocked()
}

// Close 关闭缓冲写入器
func (bw *BufferedWriter) Close() error {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	// 初始化默认值
	if !bw.initialized {
		if err := bw.initDefaults(); err != nil {
			return err
		}
	}

	// 原子设置关闭, 仅执行一次
	if !bw.closed.CompareAndSwap(false, true) {
		return nil // 已关闭, 无需重复操作
	}

	// 停止刷新定时器 (如果存在)
	if bw.flushTicker != nil {
		bw.flushTicker.Stop()
	}

	// 发送关闭信号，让定时器协程立即退出
	if bw.closeChan != nil {
		close(bw.closeChan)
	}

	// 等待所有定时器协程退出，确保资源完全释放
	bw.tickerWg.Wait()

	// 关闭前最后一次刷新, 确保数据不丢失
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
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	// 初始化默认值
	if !bw.initialized {
		if err := bw.initDefaults(); err != nil {
			return 0
		}
	}

	return bw.buffer.Len()
}

// WriteCount 返回当前缓冲区中的写入次数
func (bw *BufferedWriter) WriteCount() int {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	// 初始化默认值
	if !bw.initialized {
		if err := bw.initDefaults(); err != nil {
			return 0
		}
	}

	return bw.writeCount
}

// IsClosed 返回缓冲写入器是否已关闭
func (bw *BufferedWriter) IsClosed() bool {
	bw.mutex.Lock()
	defer bw.mutex.Unlock()

	// 初始化默认值
	if !bw.initialized {
		if err := bw.initDefaults(); err != nil {
			return false
		}
	}

	return bw.closed.Load()
}
