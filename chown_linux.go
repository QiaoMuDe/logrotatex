package logrotatex

import (
	"os"
	"syscall"
)

// osChown 是一个变量，这样我们可以在测试期间模拟它。
var osChown = os.Chown

// chown 函数用于更改指定文件的所有者和所属组。
// 它会先尝试以创建、只写和截断模式打开文件，如果文件不存在则创建它。
// 然后获取文件信息中的系统状态，从中提取用户 ID 和组 ID。
// 最后使用 osChown 函数更改文件的所有者和所属组。
// 参数 name 是要更改所有者和所属组的文件路径。
// 参数 info 是包含文件信息的 os.FileInfo 类型变量。
// 返回值为 error 类型，如果操作过程中出现错误则返回相应的错误信息，否则返回 nil。
func chown(name string, info os.FileInfo) error {
	// 以创建、只写和截断模式打开文件，如果文件不存在则创建它
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		// 打开文件失败，返回错误信息
		return err
	}
	// 关闭文件
	f.Close()
	// 获取文件信息中的系统状态
	stat := info.Sys().(*syscall.Stat_t)
	// 更改文件的所有者和所属组
	return osChown(name, int(stat.Uid), int(stat.Gid))
}
