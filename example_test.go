// example_test.go 包含了logrotatex包的示例代码和用法演示。
// 该文件提供了如何使用logrotatex进行日志轮转的具体示例，
// 包括基本配置、高级选项设置等，帮助用户快速上手使用该库。

package logrotatex

import (
	"log"
)

// To use logrotatex with the standard library's log package, just pass it into
// the SetOutput function when your application starts.
// 此函数展示了如何将 logrotatex 与标准库的 log 包结合使用。
// 在应用启动时，将 LogRotateX 实例传递给 log.SetOutput 函数。
func Example() {
	// 设置标准日志库的输出为 LogRotateX 实例
	// 日志文件路径为 /var/log/myapp/foo.log
	// 日志文件最大大小为 500MB
	// 最多保留 3 个备份文件
	// 日志文件最多保留 28 天
	// 启用日志文件压缩功能
	log.SetOutput(&LogRotateX{
		Filename:   "/var/log/myapp/foo.log",
		MaxSize:    500, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   // days
		Compress:   true, // disabled by default
	})
}
