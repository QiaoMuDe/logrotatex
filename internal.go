// internal.go 包含了logrotatex包的内部实现细节和辅助函数。
// 该文件提供了日志轮转过程中需要的内部工具函数、常量定义
// 和私有方法，支持核心功能的实现但不对外暴露接口。

package logrotatex

import (
	"os"
	"path/filepath"
	"time"
)

const (
	// backupTimeFormat 是备份文件的时间戳格式, 用于在文件名中嵌入时间信息。
	backupTimeFormat = "2006-01-02T15-04-05.000"

	// compressSuffix 是压缩文件的后缀, 用于标识已压缩的日志文件。
	compressSuffix = ".zip"

	// defaultMaxSize 是日志文件的最大默认大小(单位: MB), 在未明确设置时使用此值。
	defaultMaxSize = 10

	// 4KB - 最小缓冲区
	minBufferSize = 4 * 1024

	// 128KB - 最大缓冲区，避免过度内存使用
	maxBufferSize = 128 * 1024

	// defaultLogSuffix 是默认日志文件的后缀名
	defaultLogSuffix = "_logrotatex.log"

	// defaultFilePerm 是日志文件的默认权限模式
	defaultFilePerm = 0600

	// defaultDirPerm 是日志目录的默认权限模式
	defaultDirPerm = 0700
)

// logInfo 是一个便捷结构体，用于返回文件名及其嵌入的时间戳。
// 它包含了日志文件的时间戳信息和文件系统信息，用于日志轮转时的文件管理。
type logInfo struct {
	// timestamp 是从文件名中解析出的时间戳
	timestamp time.Time
	// FileInfo 包含文件的基本信息（大小、修改时间等）
	os.FileInfo
}

// byFormatTime 是一个自定义的排序类型, 用于按文件名中格式化的时间对日志文件进行排序。
// 它实现了 sort.Interface 接口，可以被 sort.Sort 函数使用。
type byFormatTime []logInfo

// Less 比较两个日志文件的时间戳，按时间从新到旧排序。
//
// 参数:
//   - i, j: 要比较的元素索引
//
// 返回值:
//   - bool: 如果 i 的时间戳晚于 j 则返回 true
func (b byFormatTime) Less(i, j int) bool {
	return b[i].timestamp.After(b[j].timestamp)
}

// Swap 交换两个日志文件的位置。
func (b byFormatTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

// Len 返回日志文件列表的长度。
func (b byFormatTime) Len() int {
	return len(b)
}

// filename 获取当前日志文件的完整路径。
// 如果未指定 Filename，则使用默认文件名：程序名_logrotatex.log
//
// 返回值:
//   - string: 日志文件的完整路径
func (l *LogRotateX) filename() string {
	// 如果已经指定了日志文件名, 则直接返回
	if l.Filename != "" {
		return l.Filename
	}
	// 生成默认的日志文件名, 格式为: 程序名_logrotatex.log
	name := filepath.Base(os.Args[0]) + defaultLogSuffix

	// 将日志文件存储在系统的临时目录中
	return filepath.Join(os.TempDir(), name)
}

// max 返回日志文件轮转的大小阈值。
// 如果未设置 MaxSize，则使用默认值。
//
// 返回值:
//   - int64: 日志文件的最大允许大小，单位为字节
func (l *LogRotateX) max() int64 {
	// 如果未设置最大大小, 则使用默认值
	if l.MaxSize == 0 {
		return int64(defaultMaxSize * megabyte)
	}
	// 将最大大小从 MB 转换为字节
	return int64(l.MaxSize) * int64(megabyte)
}

// dir 获取日志文件所在的目录路径。
//
// 返回值:
//   - string: 日志文件所在的目录路径
func (l *LogRotateX) dir() string {
	return filepath.Dir(l.filename())
}

// prefixAndExt 解析日志文件名，分离前缀和扩展名。
// 如果没有前缀，使用程序名作为默认前缀。
//
// 返回值:
//   - prefix: 文件名前缀
//   - ext: 文件扩展名（包含点号）
func (l *LogRotateX) prefixAndExt() (prefix, ext string) {
	filename := filepath.Base(l.filename())    // 获取日志文件的基本名称
	ext = filepath.Ext(filename)               // 提取文件的扩展名
	prefix = filename[:len(filename)-len(ext)] // 提取文件名部分并添加分隔符

	// 如果文件名没有前缀，则使用程序名作为前缀
	if prefix == "" {
		prefix = filepath.Base(os.Args[0])
	}

	return prefix, ext
}

// getBufferSize 根据文件大小计算最优缓冲区大小。
// 使用自适应算法，确保缓冲区大小在 4KB-128KB 范围内。
//
// 参数:
//   - fileSize: 文件大小，单位为字节
//
// 返回值:
//   - int: 建议的缓冲区大小
func getBufferSize(fileSize int64) int {
	// 对于空文件或极小文件，使用最小缓冲区
	if fileSize <= 0 {
		return minBufferSize
	}

	if fileSize <= minBufferSize {
		return minBufferSize // 确保不会返回小于最小缓冲区的值
	}

	// 自适应算法：缓冲区大小基于文件大小，但限制在合理范围内
	bufferSize := int(fileSize / 16) // 文件大小的1/16作为基础

	// 确保缓冲区大小是4KB的倍数，提高I/O效率
	bufferSize = (bufferSize + 4095) & ^4095

	// 限制在合理范围内，确保不会返回0或负数
	if bufferSize < minBufferSize {
		return minBufferSize
	}
	if bufferSize > maxBufferSize {
		return maxBufferSize
	}

	return bufferSize
}
