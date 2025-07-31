//go:build linux || darwin
// +build linux darwin

package logrotatex

import (
	"fmt"
	"os"
	"syscall"
)

// osChown 是一个变量，这样我们可以在测试期间模拟它。
var osChown = os.Chown

// chown 函数用于更改指定文件的所有者和所属组。
// 它会从源文件信息中提取用户 ID 和组 ID，然后应用到目标文件。
//
// 参数：
// - name: 目标文件名
// - info: 源文件信息，用于获取所有者和所属组信息
//
// 返回值：
// - error: 如果发生错误，则返回错误信息；否则返回 nil。
func chown(name string, info os.FileInfo) error {
	// 安全地获取系统状态信息
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("logrotatex: 无法获取文件系统状态信息")
	}

	// 获取源文件的用户 ID 和组 ID
	uid := int(stat.Uid)
	gid := int(stat.Gid)

	// 直接更改目标文件的所有者和所属组
	if err := os.Chown(name, uid, gid); err != nil {
		return fmt.Errorf("logrotatex: 无法设置文件所有者: %w", err)
	}

	return nil
}
