// Package logrotatex 提供了一个日志轮转功能的实现, 用于管理日志文件的大小和数量。
// 它是一个轻量级的组件, 可以与任何支持 io.Writer 接口的日志库配合使用。
//
// 主要功能:
// - 自动轮转日志文件, 防止单个文件过大
// - 支持设置最大文件大小、保留文件数量和保留天数
// - 支持日志文件压缩
// - 线程安全的设计, 适用于并发环境
//
// 注意事项:
// - 假设只有一个进程在向输出文件写入日志
// - 多个进程使用相同的配置可能导致异常行为
package logrotatex

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gitee.com/MM-Q/comprx"
)

// LogRotateX 是一个 io.WriteCloser, 它写入指定的文件名。
// 编译时接口实现检查, 确保 LogRotateX 实现了 io.WriteCloser 接口
var _ io.WriteCloser = (*LogRotateX)(nil)

var (
	// currentTime 是一个函数, 用于返回当前时间。它是一个变量, 这样测试时可以对其进行模拟。
	currentTime = time.Now

	// megabyte 是用于将MB转换为字节的常量。
	megabyte = 1024 * 1024
)

// LogRotateX 是一个 io.WriteCloser, 它会将日志写入指定的文件名。
//
// 首次调用 Write 方法时, LogRotateX 会打开或创建日志文件。如果文件已存在且大小小于 MaxSize 兆字节,
// logrotatex 会打开该文件并追加写入。如果文件已存在且大小大于或等于 MaxSize 兆字节,
// 该文件会被重命名, 重命名时会在文件名的扩展名之前 (如果没有扩展名, 则在文件名末尾) 插入当前时间戳。
// 然后会使用原始文件名创建一个新的日志文件。
//
// 每当写入操作会导致当前日志文件大小超过 MaxSize 兆字节时, 当前文件会被关闭、重命名,
// 并使用原始文件名创建一个新的日志文件。因此, 你提供给 LogRotateX 的文件名始终是"当前"的日志文件。
//
// 按天轮转功能:
//   - 当 RotateByDay 为 true 时, 每天自动轮转一次 (跨天时触发)
//   - 可以同时设置按大小轮转, 满足任一条件即轮转
//   - 支持通过 LocalTime 配置使用本地时间或 UTC 时间
//
// 日志文件命名规则:
//   - 默认格式: name_timestamp.ext, 其中 name 是不带扩展名的文件名, timestamp 是日志轮转时的时间, 格式为 `20060102150405`
//   - 如果启用 DateDirLayout, 轮转后的日志会存放在 YYYY-MM-DD/ 目录下
//   - 例如, 如果你的 LogRotateX.LogFilePath 是 `/var/log/foo/server.log`,
//     在 2016 年 11 月 11 日下午 6:30 创建的备份文件名将是 `/var/log/foo/server_20161104183000.log`
//     如果启用日期目录, 则为 `/var/log/foo/2016-11-11/server_20161111183000.log`
//
// 压缩功能:
//   - 当 Compress 为 true 时, 轮转后的日志文件会被压缩
//   - 支持多种压缩格式, 通过 CompressType 字段指定 (默认为 zip)
//   - 支持的压缩格式: zip, tar, tgz, tar.gz, gz, bz2, bzip2, zlib
//
// 清理旧日志文件, 清理规则支持三种场景，根据 MaxFiles 和 MaxAge 的组合决定:
//
//  1. 数量+天数组合 (MaxFiles>0, MaxAge>0):
//
//     - 先按天数筛选，只保留 MaxAge 天内的文件
//
//     - 然后对每天的文件按时间排序，每天最多保留 MaxFiles 个文件
//
//  2. 只按数量保留 (MaxFiles>0, MaxAge=0):
//
//     - 按时间戳从新到旧排序，保留最新的 MaxFiles 个文件
//
//     - MaxFiles 包括当前正在使用的日志文件
//
//  3. 只按天数保留 (MaxFiles=0, MaxAge>0):
//
//     - 删除所有时间戳早于 MaxAge 天的文件
//
//     - 如果 MaxFiles 和 MaxAge 都为 0, 则不会删除任何旧日志文件。
//
//     - 当 Async 为 true 时, 清理操作会异步执行, 不阻塞写入操作。
//
// 注意:
//   - MaxFiles 指定的是总文件数量（包括当前文件），不是仅备份文件数量
type LogRotateX struct {
	// LogFilePath 是写入日志的文件路径。备份日志文件将保留在同一目录中。
	// 如果该值为空, 则使用 os.TempDir() 下的 <程序名>_logrotatex.log。
	LogFilePath string `json:"logfilepath" yaml:"logfilepath"`

	// 是否启用异步清理 (单协程、合并触发)
	Async bool `json:"async" yaml:"async"`

	// MaxSize 是单个日志文件的最大大小 (以 MB 为单位) 。默认值为 10 MB。
	// 超过此大小的日志文件将被轮转。
	MaxSize int `json:"maxsize" yaml:"maxsize"`

	// MaxAge 是保留日志文件的天数, 超过此天数的文件将被删除。
	// 默认值为 0, 表示不按时间删除旧日志文件。
	MaxAge int `json:"maxage" yaml:"maxage"`

	// MaxFiles 是最大保留的历史日志文件数量, 超过此数量的旧文件将被删除。
	// 默认值为 0, 表示不限制文件数量。
	MaxFiles int `json:"maxfiles" yaml:"maxfiles"`

	// LocalTime 决定是否使用本地时间记录日志文件的轮转时间。
	// 默认使用 UTC 时间。
	LocalTime bool `json:"localtime" yaml:"localtime"`

	// Compress 决定轮转后的日志文件是否进行压缩。
	// 默认不进行压缩。
	Compress bool `json:"compress" yaml:"compress"`

	// DateDirLayout 决定是否启用按日期目录存放轮转后的日志。
	// true: 轮转后的日志存放在 YYYY-MM-DD/ 目录下
	// false: 轮转后的日志存放在当前目录下 (默认)
	DateDirLayout bool `json:"datedirlayout" yaml:"datedirlayout"`

	// RotateByDay 决定是否启用按天轮转。
	// true: 每天自动轮转一次 (跨天时触发)
	// false: 只按文件大小轮转 (默认)
	RotateByDay bool `json:"rotatebyday" yaml:"rotatebyday"`

	// CompressType 压缩类型, 默认为: comprx.CompressTypeZip
	//
	// 支持的压缩格式：
	//   - comprx.CompressTypeZip: zip 压缩格式
	//   - comprx.CompressTypeTar: tar 压缩格式
	//   - comprx.CompressTypeTgz: tgz 压缩格式
	//   - comprx.CompressTypeTarGz: tar.gz 压缩格式
	//   - comprx.CompressTypeGz: gz 压缩格式
	//   - comprx.CompressTypeBz2: bz2 压缩格式
	//   - comprx.CompressTypeBzip2: bzip2 压缩格式
	//   - comprx.CompressTypeZlib: zlib 压缩格式
	CompressType comprx.CompressType `json:"compress_type" yaml:"compress_type"`

	// 内部状态
	filePerm         os.FileMode    // filePerm 是日志文件的权限模式。默认值为 0600
	size             int64          // size 是当前日志文件的大小 (以字节为单位)
	file             *os.File       // file 是当前打开的日志文件
	mu               sync.Mutex     // mu 是互斥锁, 用于保护文件操作
	closed           atomic.Bool    // closed 标志: true 表示已关闭；关闭后 Write/Sync 直接拒绝
	cleanupRunning   atomic.Bool    // 清理协程运行标志: false=未运行, true=运行中
	rerunNeeded      atomic.Bool    // 重跑需求标志: false=不需要重跑, true=需要在本轮后再跑一次
	wg               sync.WaitGroup // wg 是等待组, 用于等待清理协程退出
	lastRotationDate time.Time      // lastRotationDate 上次轮转的日期 (只记录日期, 不记录时间)
	initialized      bool           // initialized 标志: true 表示已初始化, 避免重复初始化
}

// Default 返回一个默认的 LogRotateX 实例, 日志文件路径为 "logs/app.log"。
//
// 默认配置:
//   - Async: false (默认同步)
//   - MaxSize: 10MB (默认值)
//   - MaxAge: 0 (默认不清理历史文件)
//   - MaxFiles: 0 (默认不清理备份文件)
//   - LocalTime: true (默认使用本地时间)
//   - Compress: false (默认不压缩)
//   - DateDirLayout: true (默认按日期目录存放)
//   - RotateByDay: true (默认按天轮转)
//   - CompressType: comprx.CompressTypeZip (默认压缩类型为 zip)
func Default() *LogRotateX {
	return NewLogRotateX("logs/app.log")
}

// NewLRX 是 NewLogRotateX 的简写形式, 用于创建新的 LogRotateX 实例。
var NewLRX = NewLogRotateX

// NewLogRotateX 创建一个新的 LogRotateX 实例, 使用默认配置。
//
// 参数:
//   - logFilePath: 日志文件路径
//
// 返回值:
//   - *LogRotateX: 配置好的实例
//
// 默认配置:
//   - Async: false (默认同步)
//   - MaxSize: 10MB (默认值)
//   - MaxAge: 0 (默认不清理历史文件)
//   - MaxFiles: 0 (默认不清理备份文件)
//   - LocalTime: true (默认使用本地时间)
//   - Compress: false (默认不压缩)
//   - DateDirLayout: true (默认按日期目录存放)
//   - RotateByDay: true (默认按天轮转)
//   - CompressType: comprx.CompressTypeZip (默认压缩类型为 zip)
func NewLogRotateX(logFilePath string) *LogRotateX {
	// 清理文件路径
	logFilePath = filepath.Clean(logFilePath)

	// 去除左右空格
	logFilePath = strings.TrimSpace(logFilePath)

	// 验证文件路径
	if logFilePath == "" {
		panic("logrotatex: log file path cannot be empty")
	}

	// 创建 LogRotateX 实例并设置默认值 (显式初始化所有内部字段)
	logger := &LogRotateX{
		LogFilePath:   logFilePath,            // 日志文件路径
		Async:         false,                  // 是否异步清理 (默认同步)
		MaxSize:       10,                     // 10MB (默认值)
		MaxAge:        0,                      // 0天 (默认不清理历史文件)
		MaxFiles:      0,                      // 0个备份文件 (默认不清理备份文件)
		LocalTime:     true,                   // 使用本地时间
		Compress:      false,                  // 禁用压缩
		DateDirLayout: true,                   // 启用日期目录 (默认行为)
		RotateByDay:   true,                   // 启用按天轮转 (默认行为)
		CompressType:  comprx.CompressTypeZip, // 默认压缩类型为 zip
	}

	return logger
}

// initDefaults 初始化 LogRotateX 实例的默认值。
// 该方法确保无论是通过构造函数创建还是直接通过结构体字面量创建，
// 都能获得一致的初始化行为。
// 注意：该方法只会执行一次，避免重复初始化。
//
// 返回值:
//   - error: 初始化失败时返回错误，否则返回 nil
func (l *LogRotateX) initDefaults() error {
	// 如果已经初始化过，直接返回
	if l.initialized {
		return nil
	}

	// 如果 LogFilePath 为空，设置默认值
	if l.LogFilePath == "" {
		l.LogFilePath = getDefaultLogFilePath()
	}

	// 确保 LogFilePath 是干净的路径
	l.LogFilePath = filepath.Clean(l.LogFilePath)
	l.LogFilePath = strings.TrimSpace(l.LogFilePath)

	// 验证文件路径
	if l.LogFilePath == "" {
		return fmt.Errorf("logrotatex: log file path cannot be empty")
	}

	// 确保目录存在
	dir := filepath.Dir(l.LogFilePath)
	if err := os.MkdirAll(dir, defaultDirPerm); err != nil {
		return fmt.Errorf("logrotatex: failed to create log directory: %w", err)
	}

	// 初始化最大文件大小
	if l.MaxSize <= 0 {
		l.MaxSize = defaultMaxSize
	}

	// 初始化最大保留时间
	if l.MaxAge < 0 {
		l.MaxAge = 0
	}

	// 初始化最大备份文件数
	if l.MaxFiles < 0 {
		l.MaxFiles = 0
	}

	// 初始化内部文件权限
	if l.filePerm == 0 {
		l.filePerm = defaultFilePerm
	}

	// 初始化内部文件大小
	if l.size == 0 {
		l.size = 0
	}

	// 显式设置原子布尔的初始值为 false
	l.closed.Store(false)
	l.cleanupRunning.Store(false)
	l.rerunNeeded.Store(false)

	// 初始化压缩类型, 如果为空, 则设置为默认值 zip
	if l.CompressType.String() == "" {
		l.CompressType = comprx.CompressTypeZip
	}

	// 标记为已初始化
	l.initialized = true

	return nil
}

// Write 实现 io.Writer 接口, 向日志文件写入数据。
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

	// 初始化默认值（确保直接通过结构体字面量创建的实例也能正确初始化）
	if !l.initialized {
		if err := l.initDefaults(); err != nil {
			return 0, err
		}
	}

	// 关闭后快速短路, 避免继续 open/rotate/write
	if l.closed.Load() {
		return 0, errors.New("logrotatex: write on closed")
	}

	// 计算要写入的数据长度
	writeLen := int64(len(p))

	// 检查文件是否已打开, 如果未打开则尝试打开或创建文件
	if l.file == nil {
		if err = l.openExistingOrNew(len(p)); err != nil {
			return 0, err
		}
	}

	// 检查当前写入是否会导致文件大小达到或超过限制, 如果是则触发轮转
	if l.size+writeLen >= l.max() {
		if rotateErr := l.rotate(); rotateErr != nil {
			return 0, fmt.Errorf("logrotatex: failed to rotate file: %w", rotateErr)
		}
	}

	// 检查是否跨天, 触发按天轮转 (仅在启用时)
	if l.RotateByDay && l.shouldRotateByDay() {
		if rotateErr := l.rotate(); rotateErr != nil {
			return 0, fmt.Errorf("logrotatex: failed to rotate file: %w", rotateErr)
		}
	}

	// 再次检查文件是否已打开
	if l.file == nil {
		return 0, errors.New("logrotatex: file handle is nil after attempting to open or rotate")
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
//   - error: 关闭失败时返回错误, 否则返回 nil
func (l *LogRotateX) Close() error {
	// 使用原子 CAS 保证关闭逻辑只执行一次
	if !l.closed.CompareAndSwap(false, true) {
		// 已关闭: 幂等返回
		return nil
	}
	// 执行具体的关闭操作
	if err := l.close(); err != nil {
		return err
	}
	// 若启用异步清理, 等待后台协程收敛
	if l.Async {
		l.wg.Wait()
	}
	return nil
}

// Sync 强制将缓冲区数据同步到磁盘。
//
// 返回值:
//   - error: 同步失败时返回错误, 否则返回 nil
func (l *LogRotateX) Sync() error {
	// 加锁以确保并发安全, 防止在同步过程中文件被其他操作修改
	l.mu.Lock()
	defer l.mu.Unlock()

	// 初始化默认值（确保直接通过结构体字面量创建的实例也能正确初始化）
	if !l.initialized {
		if err := l.initDefaults(); err != nil {
			return err
		}
	}

	// 二次检查, 避免与关闭并发竞态
	if l.closed.Load() {
		return errors.New("logrotatex: sync on closed")
	}

	// 检查文件是否已打开, 如果已打开则执行同步操作
	if l.file != nil {
		return l.file.Sync()
	}
	// 如果文件未打开, 则无需同步, 直接返回 nil
	return nil
}
