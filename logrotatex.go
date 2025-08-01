// Package logrotatex 提供了一个日志轮转功能的实现，用于管理日志文件的大小和数量。
// 它是一个轻量级的组件，可以与任何支持 io.Writer 接口的日志库配合使用。
//
// 主要功能：
// - 自动轮转日志文件，防止单个文件过大
// - 支持设置最大文件大小、保留文件数量和保留天数
// - 支持日志文件压缩
// - 线程安全的设计，适用于并发环境
//
// 注意事项：
// - 假设只有一个进程在向输出文件写入日志
// - 多个进程使用相同的配置可能导致异常行为
package logrotatex

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// LogRotateX 是一个 io.WriteCloser, 它写入指定的文件名。
// 编译时接口实现检查，确保 LogRotateX 实现了 io.WriteCloser 接口
var _ io.WriteCloser = (*LogRotateX)(nil)

var (
	// currentTime 是一个函数, 用于返回当前时间。它是一个变量, 这样测试时可以对其进行模拟。
	currentTime = time.Now

	// megabyte 是用于将MB转换为字节的常量。
	megabyte = 1024 * 1024
)

// LogRotateX 是一个 io.WriteCloser，它会将日志写入指定的文件名。
//
// 首次调用 Write 方法时，LogRotateX 会打开或创建日志文件。如果文件已存在且大小小于 MaxSize 兆字节，
// logrotatex 会打开该文件并追加写入。如果文件已存在且大小大于或等于 MaxSize 兆字节，
// 该文件会被重命名，重命名时会在文件名的扩展名之前（如果没有扩展名，则在文件名末尾）插入当前时间戳。
// 然后会使用原始文件名创建一个新的日志文件。
//
// 每当写入操作会导致当前日志文件大小超过 MaxSize 兆字节时，当前文件会被关闭、重命名，
// 并使用原始文件名创建一个新的日志文件。因此，你提供给 LogRotateX 的文件名始终是"当前"的日志文件。
//
// 备份文件使用提供给 LogRotateX 的日志文件名，格式为 `name-timestamp.ext`，
// 其中 name 是不带扩展名的文件名，timestamp 是日志轮转时的时间，格式为 `2006-01-02T15-04-05.000`，
// ext 是原始扩展名。例如，如果你的 LogRotateX.Filename 是 `/var/log/foo/server.log`，
// 在 2016 年 11 月 11 日下午 6:30 创建的备份文件名将是 `/var/log/foo/server-2016-11-04T18-30-00.000.log`。
//
// # 清理旧日志文件
//
// 每当创建新的日志文件时，可能会删除旧的日志文件。根据编码的时间戳，最近的文件会被保留，
// 最多保留数量等于 MaxBackups（如果 MaxBackups 为 0，则保留所有文件）。
// 任何编码时间戳早于 MaxAge 天的文件都会被删除，无论 MaxBackups 的设置如何。
// 请注意，时间戳中编码的时间是轮转时间，可能与该文件最后一次写入的时间不同。
//
// 如果 MaxBackups 和 MaxAge 都为 0，则不会删除任何旧日志文件。
type LogRotateX struct {
	/* ========== 配置字段 ========== */
	// Filename 是写入日志的文件。备份日志文件将保留在同一目录中。如果该值为空, 则使用 os.TempDir() 下的 <程序名>_logrotatex.log。
	Filename string `json:"filename" yaml:"filename"`

	// MaxSize 最大单个日志文件的大小（以 MB 为单位）。默认值为 10 MB。
	MaxSize int `json:"maxsize" yaml:"maxsize"`

	// MaxAge 最大保留日志文件的天数。默认情况下, 不会删除旧日志文件。
	MaxAge int `json:"maxage" yaml:"maxage"`

	// MaxBackups 最大保留日志文件的数量。默认情况下, 不会删除旧日志文件。
	MaxBackups int `json:"maxbackups" yaml:"maxbackups"`

	// ========== 行为选项 ==========
	// LocalTime 决定是否使用本地时间记录日志文件的轮转时间。默认使用 UTC 时间。
	LocalTime bool `json:"localtime" yaml:"localtime"`

	// Compress 决定轮转后的日志文件是否应使用 zip 进行压缩。默认不进行压缩。
	Compress bool `json:"compress" yaml:"compress"`

	// FilePerm 是日志文件的权限模式。默认值为 0600。
	FilePerm os.FileMode `json:"fileperm" yaml:"fileperm"`

	/* ========== 运行时状态 ========== */
	// size 是当前日志文件的大小（以字节为单位）。
	size int64

	// file 是当前打开的日志文件。
	file *os.File

	/* ========== 并发控制 ========== */
	// mu 是互斥锁, 用于保护文件操作。
	mu sync.Mutex

	/* ========== 后台处理 ========== */
	// millCh 是一个通道, 用于通知 LogRotateX 进行压缩和删除旧日志文件。
	millCh chan bool

	// millDone 是一个通道, 用于通知mill goroutine退出
	millDone chan struct{}

	/* ========== 生命周期控制 ========== */
	// startMill 是一个 sync.Once, 用于确保只启动一次压缩和删除旧日志文件的 goroutine。
	startMill sync.Once

	// closeOnce 是一个 sync.Once, 用于确保只执行一次关闭操作
	closeOnce sync.Once

	// millStarted 标记mill goroutine是否已启动 (使用原子操作)
	millStarted atomic.Bool
}

// NewLogRotateX 创建一个新的 LogRotateX 实例，使用指定的文件路径和合理的默认配置。
// 该构造函数会验证和清理文件路径，确保路径安全性，并设置推荐的默认值。
// 如果路径不安全或创建失败，此函数会立即 panic，确保问题能够快速被发现。
//
// 参数:
//   - filename string: 日志文件的路径，会进行安全验证和清理
//
// 返回值:
//   - *LogRotateX: 配置好的 LogRotateX 实例
//
// 默认配置:
//   - MaxSize: 10MB (单个日志文件最大大小)
//   - MaxAge: 0天 (日志文件最大保留时间, 0表示不清理历史文件)
//   - MaxBackups: 0个 (最大备份文件数量, 0表示不清理备份文件)
//   - LocalTime: true (使用本地时间)
//   - Compress: false (禁用压缩)
//   - FilePerm: 0600 (文件权限，所有者读写，组和其他用户只读)
//
// 注意: 如果文件路径不安全或创建失败，此函数会 panic
func NewLogRotateX(filename string) *LogRotateX {
	// 验证和清理文件路径
	safePath, err := sanitizePath(filename)
	if err != nil {
		panic(fmt.Sprintf("logrotatex: 创建 LogRotateX 失败，文件路径不安全: %v", err))
	}

	// 确保目录存在
	dir := filepath.Dir(safePath)
	if err := os.MkdirAll(dir, defaultDirPerm); err != nil {
		panic(fmt.Sprintf("logrotatex: 创建日志目录失败: %v", err))
	}

	// 创建 LogRotateX 实例并设置默认值
	logger := &LogRotateX{
		Filename:   safePath,        // 日志文件路径
		MaxSize:    10,              // 10MB
		MaxAge:     0,               // 0天 (默认不清理历史文件)
		MaxBackups: 0,               // 0个备份文件 (默认不清理备份文件)
		LocalTime:  true,            // 使用本地时间
		Compress:   false,           // 禁用压缩
		FilePerm:   defaultFilePerm, // 文件权限：所有者读写，组和其他用户只读
	}

	return logger
}

// Write 实现了 io.Writer 接口，用于向日志文件写入数据。
// 该方法会处理日志轮转逻辑，确保单个日志文件不会超过设定的最大大小。
//
// 参数:
//   - p []byte: 要写入的日志数据
//
// 返回值:
//   - n int: 实际写入的字节数
//   - err error: 如果写入过程中发生错误，则返回该错误；否则返回 nil
func (l *LogRotateX) Write(p []byte) (n int, err error) {
	// 加锁以确保并发安全, 防止多个 goroutine 同时操作文件
	l.mu.Lock()
	// 函数返回时解锁, 保证锁一定会被释放
	defer l.mu.Unlock()

	// 计算要写入的数据长度
	writeLen := int64(len(p))

	// 确保文件已正确打开
	if err = l.openExistingOrNew(len(p)); err != nil {
		return 0, err
	}

	// 检查是否需要轮转: 当前文件大小+写入数据长度 > 超过最大大小
	if l.size+writeLen > l.max() {
		// 执行日志轮转操作
		if err := l.rotate(); err != nil {
			return 0, err
		}

		// 轮转后必须重新确保文件打开
		if err = l.openExistingOrNew(len(p)); err != nil {
			return 0, err
		}
	}

	// 双重检查确保文件句柄有效，如果为nil则尝试重新打开
	if l.file == nil {
		if err = l.openExistingOrNew(len(p)); err != nil {
			return 0, fmt.Errorf("logrotatex: 无法打开文件进行写入: %w", err)
		}
		// 如果重新打开后仍然为nil，则返回错误
		if l.file == nil {
			return 0, fmt.Errorf("logrotatex: 文件句柄仍然无效，无法写入数据")
		}
	}

	// 安全地将所有数据写入文件
	n, err = l.file.Write(p)
	if err != nil {
		return n, fmt.Errorf("logrotatex: 写入文件失败: %w", err)
	}
	l.size += int64(n) // 更新当前文件大小

	return n, nil
}

// Close 是 LogRotateX 类型的 Close 方法, 用于关闭日志记录器。
// 该方法会关闭当前打开的日志文件，释放相关资源，并停止后台goroutine。
// 此操作是线程安全的，使用 sync.Once 防止重复调用，并通过上下文控制超时。
// 在异常情况下确保文件句柄正确关闭，防止资源泄漏。
//
// 返回值:
//   - error: 如果在关闭文件时发生错误，则返回该错误；否则返回 nil。
func (l *LogRotateX) Close() error {
	var closeErr error

	// 使用 sync.Once 确保整个关闭操作只执行一次
	l.closeOnce.Do(func() {
		// 创建一个带5秒超时的上下文
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		done := make(chan error, 1)

		go func() {
			defer func() {
				// 使用defer确保即使在panic情况下也能发送结果
				if r := recover(); r != nil {
					done <- fmt.Errorf("关闭操作发生panic: %v", r)
				}
			}()

			// 停止mill goroutine
			if l.millStarted.Load() && l.millDone != nil {
				// 使用select避免在通道已关闭时阻塞
				select {
				case <-l.millDone:
					// 通道已关闭
				default:
					close(l.millDone)
				}
				l.millStarted.Store(false)
			}

			// 关闭mill通道
			if l.millCh != nil {
				// 使用select避免在通道已关闭时阻塞
				select {
				case <-l.millCh:
					// 通道已关闭
				default:
					close(l.millCh)
				}
				l.millCh = nil
			}

			// 调用 LogRotateX 的 close 方法, 执行具体的关闭操作
			done <- l.close()
		}()

		// 等待关闭完成或上下文取消
		select {
		case err := <-done:
			closeErr = err
		case <-ctx.Done():
			// 即使超时也要尝试强制关闭文件句柄
			if l.file != nil {
				l.mu.Lock()
				if l.file != nil {
					_ = l.file.Close() // 强制关闭，忽略错误
					l.file = nil
				}
				l.mu.Unlock()
			}
			closeErr = fmt.Errorf("关闭操作超时: %w", ctx.Err())
		}
	})

	return closeErr
}

// Rotate 是 LogRotateX 类型的一个方法, 用于执行日志文件的轮转操作。
// 该方法会关闭当前日志文件，将其重命名为带有时间戳的备份文件，
// 然后创建一个新的日志文件用于后续写入。
// 此操作是线程安全的，使用互斥锁保护。
//
// 返回值:
//   - error: 如果在执行轮转操作时发生错误，则返回该错误；否则返回 nil。
func (l *LogRotateX) Rotate() error {
	// 使用互斥锁来确保日志轮转操作的线程安全。
	l.mu.Lock()
	// 在函数结束时自动解锁, 以确保即使在发生错误时也能正确释放锁。
	defer l.mu.Unlock()
	// 调用具体的日志轮转实现方法 rotate, 并返回其结果。
	return l.rotate()
}
