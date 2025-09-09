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

// Less 方法用于比较两个日志文件的时间戳, 按时间从新到旧排序。
// 这意味着索引较小的元素时间戳较新。
// 参数:
//   - i: 第一个元素的索引
//   - j: 第二个元素的索引
//
// 返回值:
//   - bool: 如果第一个元素的时间戳晚于第二个元素的时间戳，则返回 true
func (b byFormatTime) Less(i, j int) bool {
	return b[i].timestamp.After(b[j].timestamp)
}

// Swap 方法用于交换两个日志文件的位置。
// 参数:
//   - i: 第一个元素的索引
//   - j: 第二个元素的索引
func (b byFormatTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

// Len 方法返回日志文件列表的长度。
// 返回值:
//   - int: 日志文件列表的长度
func (b byFormatTime) Len() int {
	return len(b)
}

// filename 获取当前日志文件的完整路径名称
//
// 该方法实现了日志文件名的智能生成策略：
// 1. 优先使用用户显式指定的 Filename 字段
// 2. 如果未指定，则自动生成默认文件名：程序名 + "_logrotatex.log"
// 3. 默认文件存储在系统临时目录中，确保跨平台兼容性
//
// 返回值:
//   - string: 日志文件的完整路径，格式为绝对路径或相对路径
//
// 示例:
//   - 指定文件名: "/var/log/app.log"
//   - 默认文件名: "/tmp/myapp_logrotatex.log" (Unix) 或 "C:\Temp\myapp_logrotatex.log" (Windows)
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

// max 计算并返回日志文件轮转的大小阈值
//
// 该方法负责确定日志文件何时需要进行轮转操作：
// 1. 如果用户设置了 MaxSize 字段（非零值），则使用该值
// 2. 如果未设置（为0），则使用系统默认值 defaultMaxSize
// 3. 所有大小值都会从 MB 单位转换为字节单位进行内部计算
//
// 参数:
//   - 无（使用接收者的 MaxSize 字段）
//
// 返回值:
//   - int64: 日志文件的最大允许大小，单位为字节
//
// 注意:
//   - 默认值通常为 100MB (104,857,600 字节)
//   - 返回值用于与当前文件大小进行比较，决定是否触发轮转
func (l *LogRotateX) max() int64 {
	// 如果未设置最大大小, 则使用默认值
	if l.MaxSize == 0 {
		return int64(defaultMaxSize * megabyte)
	}
	// 将最大大小从 MB 转换为字节
	return int64(l.MaxSize) * int64(megabyte)
}

// dir 获取日志文件所在的目录路径
//
// 该方法从完整的日志文件路径中提取目录部分，用于：
// 1. 创建日志目录（如果不存在）
// 2. 扫描同目录下的历史日志文件
// 3. 执行文件清理和轮转操作
//
// 实现细节:
//   - 使用 filepath.Dir() 确保跨平台路径处理的正确性
//   - 自动处理绝对路径和相对路径的情况
//
// 返回值:
//   - string: 日志文件所在的目录路径，不包含文件名部分
//
// 示例:
//   - 输入: "/var/log/app.log" -> 输出: "/var/log"
//   - 输入: "logs/app.log" -> 输出: "logs"
//   - 输入: "app.log" -> 输出: "."
func (l *LogRotateX) dir() string {
	return filepath.Dir(l.filename())
}

// prefixAndExt 解析日志文件名，分离前缀和扩展名
//
// 该方法用于生成轮转后的日志文件名，通过分离原始文件名的组成部分：
// 1. prefix: 文件名主体部分，用作轮转文件的基础名称
// 2. ext: 文件扩展名，保持轮转文件的类型一致性
//
// 处理逻辑:
//   - 提取文件的基本名称（去除路径部分）
//   - 分离扩展名（如 .log, .txt 等）
//   - 如果没有前缀，使用程序名作为默认前缀
//
// 返回值:
//   - prefix: 文件名前缀，用于构建轮转文件名
//   - ext: 文件扩展名，包含点号（如 ".log"）
//
// 示例:
//   - 输入: "app.log" -> 输出: prefix="app", ext=".log"
//   - 输入: "service" -> 输出: prefix="service", ext=""
//   - 输入: "" -> 输出: prefix="程序名", ext=""
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

// getBufferSize 计算文件操作的最优缓冲区大小
//
// 该函数实现了自适应缓冲区大小算法，根据文件大小动态调整缓冲区：
// 1. 对于小文件（≤4KB），使用最小缓冲区避免内存浪费
// 2. 对于大文件，使用文件大小的1/16作为基础缓冲区
// 3. 确保缓冲区大小是4KB的倍数，提高I/O效率
// 4. 限制在合理范围内（4KB - 1MB），平衡性能和内存使用
//
// 算法优势:
//   - 小文件快速处理，减少内存开销
//   - 大文件高效传输，减少系统调用次数
//   - 4KB对齐优化，匹配操作系统页面大小
//
// 参数:
//   - fileSize: 文件大小，单位为字节
//
// 返回值:
//   - int: 建议的缓冲区大小，范围在 [4KB, 1MB] 之间
//
// 示例:
//   - fileSize=1KB -> 返回4KB
//   - fileSize=64KB -> 返回4KB
//   - fileSize=1MB -> 返回64KB
//   - fileSize=16MB -> 返回1MB
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
