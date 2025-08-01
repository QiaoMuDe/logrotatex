// internal.go 包含了logrotatex包的内部实现细节和辅助函数。
// 该文件提供了日志轮转过程中需要的内部工具函数、常量定义
// 和私有方法，支持核心功能的实现但不对外暴露接口。

package logrotatex

import (
	"os"
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

// 系统敏感目录映射表，用于快速检查路径安全性
// 使用 map 结构提供 O(1) 查找性能，避免线性搜索
var dangerousPathsMap = map[string]string{
	// === 核心系统目录（绝对不能碰）===
	"/etc":      "系统配置目录",
	"/boot":     "系统启动目录",
	"/usr/bin":  "系统可执行文件目录",
	"/usr/sbin": "系统管理员可执行文件目录",
	"/sbin":     "系统二进制文件目录",
	"/bin":      "基本二进制文件目录",
	"/proc":     "进程虚拟文件系统",
	"/sys":      "系统虚拟文件系统",
	"/dev":      "设备文件目录",
	"/root":     "超级用户主目录",

	// === 系统库目录（高风险）===
	"/lib":         "系统库目录",
	"/lib64":       "64位系统库目录",
	"/usr/lib":     "用户系统库目录",
	"/usr/lib64":   "64位用户系统库目录",
	"/lib/modules": "内核模块目录",

	// === 系统运行时（高风险）===
	"/run":      "运行时文件目录",
	"/var/run":  "运行时变量目录",
	"/var/lock": "锁文件目录",

	// === 特别危险的系统日志 ===
	"/var/log/kern.log": "内核日志文件",
	"/var/log/auth.log": "认证日志文件",
	"/var/log/secure":   "安全日志文件",
	"/var/log/messages": "系统消息日志文件",
	"/var/log/syslog":   "系统日志文件",

	// === 关键系统文件 ===
	"/etc/passwd":  "用户账户信息文件",
	"/etc/shadow":  "用户密码文件",
	"/etc/group":   "用户组信息文件",
	"/etc/sudoers": "sudo权限配置文件",
	"/etc/ssh":     "SSH配置目录",
}

// dangerousPatternsMap 定义了路径遍历攻击的危险模式
// 使用map进行O(1)查找，提高性能，避免slice遍历的O(n)复杂度
var dangerousPatternsMap = map[string]bool{
	"..":      true,
	"..\\":    true,
	"../":     true,
	".\\..\\": true,
	"./..":    true,
	"%2e%2e":  true,
	"%2E%2E":  true,
}

// windowsReservedNamesMap 定义了Windows系统保留的文件名
// 使用map进行O(1)查找，提高性能，避免slice遍历的O(n)复杂度
var windowsReservedNamesMap = map[string]bool{
	"CON":  true,
	"PRN":  true,
	"AUX":  true,
	"NUL":  true,
	"COM1": true,
	"COM2": true,
	"COM3": true,
	"COM4": true,
	"COM5": true,
	"COM6": true,
	"COM7": true,
	"COM8": true,
	"COM9": true,
	"LPT1": true,
	"LPT2": true,
	"LPT3": true,
	"LPT4": true,
	"LPT5": true,
	"LPT6": true,
	"LPT7": true,
	"LPT8": true,
	"LPT9": true,
}

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
