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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
// 日志文件使用提供给 LogRotateX 的日志文件名，格式为 `name_timestamp.ext`，
// 其中 name 是不带扩展名的文件名，timestamp 是日志轮转时的时间，格式为 `20060102150405`，
// ext 是原始扩展名。例如，如果你的 LogRotateX.LogFilePath 是 `/var/log/foo/server.log`，
// 在 2016 年 11 月 11 日下午 6:30 创建的备份文件名将是 `/var/log/foo/server_20161104183000.log`。
//
// 清理旧日志文件:
//   - 每当创建新的日志文件时，可能会删除旧的日志文件。根据编码的时间戳，最近的文件会被保留，
//     最多保留数量等于 MaxFiles（如果 MaxFiles 为 0，则保留所有文件）。
//     任何编码时间戳早于 MaxAge 天的文件都会被删除，无论 MaxFiles 的设置如何。
//     请注意，时间戳中编码的时间是轮转时间，可能与该文件最后一次写入的时间不同。
//   - 如果 MaxFiles 和 MaxAge 都为 0，则不会删除任何旧日志文件。
type LogRotateX struct {
	// LogFilePath 是写入日志的文件路径。备份日志文件将保留在同一目录中。
	// 如果该值为空，则使用 os.TempDir() 下的 <程序名>_logrotatex.log。
	LogFilePath string `json:"logfilepath" yaml:"logfilepath"`

	// 是否启用异步清理 (单协程、合并触发)
	Async bool `json:"async" yaml:"async"`

	// MaxSize 是单个日志文件的最大大小（以 MB 为单位）。默认值为 10 MB。
	// 超过此大小的日志文件将被轮转。
	MaxSize int `json:"maxsize" yaml:"maxsize"`

	// MaxAge 是保留日志文件的天数，超过此天数的文件将被删除。
	// 默认值为 0，表示不按时间删除旧日志文件。
	MaxAge int `json:"maxage" yaml:"maxage"`

	// MaxFiles 是最大保留的历史日志文件数量，超过此数量的旧文件将被删除。
	// 默认值为 0，表示不限制文件数量。
	MaxFiles int `json:"maxfiles" yaml:"maxfiles"`

	// LocalTime 决定是否使用本地时间记录日志文件的轮转时间。
	// 默认使用 UTC 时间。
	LocalTime bool `json:"localtime" yaml:"localtime"`

	// Compress 决定轮转后的日志文件是否应使用 zip 进行压缩。
	// 默认不进行压缩。
	Compress bool `json:"compress" yaml:"compress"`

	// 内部状态
	filePerm       os.FileMode    // filePerm 是日志文件的权限模式。默认值为 0600
	size           int64          // size 是当前日志文件的大小（以字节为单位）
	file           *os.File       // file 是当前打开的日志文件
	mu             sync.Mutex     // mu 是互斥锁，用于保护文件操作
	closed         atomic.Bool    // closed 标志：true 表示已关闭；关闭后 Write/Sync 直接拒绝
	cleanupRunning atomic.Bool    // 清理协程运行标志: false=未运行，true=运行中
	rerunNeeded    atomic.Bool    // 重跑需求标志: false=不需要重跑，true=需要在本轮后再跑一次
	wg             sync.WaitGroup // wg 是等待组，用于等待清理协程退出
}

// NewLRX 是 NewLogRotateX 的简写形式，用于创建新的 LogRotateX 实例。
var NewLRX = NewLogRotateX

// NewLogRotateX 创建一个新的 LogRotateX 实例，使用默认配置。
//
// 参数:
//   - logFilePath: 日志文件路径
//
// 返回值:
//   - *LogRotateX: 配置好的实例
//
// 默认配置: MaxSize=10MB, MaxAge=0, MaxSize=0, LocalTime=true, Compress=false
func NewLogRotateX(logFilePath string) *LogRotateX {
	// 清理文件路径
	logFilePath = filepath.Clean(logFilePath)

	// 去除左右空格
	logFilePath = strings.TrimSpace(logFilePath)

	// 验证文件路径
	if logFilePath == "" {
		panic("logrotatex: log file path cannot be empty")
	}

	// 确保目录存在
	dir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(dir, defaultDirPerm); err != nil {
		panic(fmt.Sprintf("logrotatex: failed to create log directory: %v", err))
	}

	// 创建 LogRotateX 实例并设置默认值
	logger := &LogRotateX{
		LogFilePath: logFilePath,     // 日志文件路径
		MaxSize:     10,              // 10MB
		MaxAge:      0,               // 0天 (默认不清理历史文件)
		MaxFiles:    0,               // 0个备份文件 (默认不清理备份文件)
		LocalTime:   true,            // 使用本地时间
		Compress:    false,           // 禁用压缩
		filePerm:    defaultFilePerm, // 文件权限
	}

	return logger
}

// Write 实现 io.Writer 接口，向日志文件写入数据。
// 当文件大小超过限制时自动执行轮转。
//
// 参数:
//   - p: 要写入的数据
//
// 返回值:
//   - n: 实际写入的字节数
//   - err: 写入失败时返回错误
func (l *LogRotateX) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 关闭后快速短路，避免继续 open/rotate/write
	if l.closed.Load() {
		return 0, fmt.Errorf("logrotatex: write on closed")
	}

	// 计算要写入的数据长度
	writeLen := int64(len(p))

	// 检查文件是否已打开，如果未打开则尝试打开或创建文件
	if l.file == nil {
		if err = l.openExistingOrNew(len(p)); err != nil {
			return 0, err
		}
	}

	// 检查当前写入是否会导致文件大小超过限制，如果是则触发轮转
	if l.size+writeLen > l.max() {
		if rotateErr := l.rotate(); rotateErr != nil {
			return 0, fmt.Errorf("logrotatex: failed to rotate file: %w", rotateErr)
		}
	}

	// 再次检查文件是否已打开
	if l.file == nil {
		return 0, fmt.Errorf("logrotatex: file handle is nil after attempting to open or rotate")
	}

	// 安全地将所有数据写入文件
	n, err = l.file.Write(p)
	if err != nil {
		return n, fmt.Errorf("logrotatex: failed to write to file: %w", err)
	}
	l.size += int64(n) // 更新当前文件大小

	return n, nil
}

// Close 关闭日志文件
//
// 返回值:
//   - error: 关闭失败时返回错误，否则返回 nil
func (l *LogRotateX) Close() error {
	// 使用原子 CAS 保证关闭逻辑只执行一次
	if !l.closed.CompareAndSwap(false, true) {
		// 已关闭：幂等返回
		return nil
	}
	// 执行具体的关闭操作
	if err := l.close(); err != nil {
		return err
	}
	// 若启用异步清理，等待后台协程收敛
	if l.Async {
		l.wg.Wait()
	}
	return nil
}

// Sync 强制将缓冲区数据同步到磁盘。
//
// 返回值:
//   - error: 同步失败时返回错误，否则返回 nil
func (l *LogRotateX) Sync() error {
	// 加锁以确保并发安全，防止在同步过程中文件被其他操作修改
	l.mu.Lock()
	defer l.mu.Unlock()

	// 二次检查，避免与关闭并发竞态
	if l.closed.Load() {
		return fmt.Errorf("logrotatex: sync on closed")
	}

	// 检查文件是否已打开，如果已打开则执行同步操作
	if l.file != nil {
		return l.file.Sync()
	}
	// 如果文件未打开，则无需同步，直接返回 nil
	return nil
}
