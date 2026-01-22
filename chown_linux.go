// chown_linux.go 实现了Linux和Darwin系统下的文件所有者变更功能。
// 该文件通过系统调用获取源文件的用户ID和组ID，并将其应用到目标文件上，
// 确保轮转后的日志文件保持与原文件相同的所有者权限。
//go:build linux || darwin
// +build linux darwin

package logrotatex

// import (
// 	"fmt"
// 	"os"
// 	"syscall"
// )

// // osChown 是一个变量，这样我们可以在测试期间模拟它。
// var osChown = os.Chown

// // chown 更改指定文件的所有者和所属组。
// //
// // 参数:
// //   - name: 目标文件名
// //   - info: 源文件信息，用于获取所有者信息
// //
// // 返回值:
// //   - error: 设置失败时返回错误，否则返回 nil
// func chown(name string, info os.FileInfo) error {
// 	// 安全地获取系统状态信息
// 	stat, ok := info.Sys().(*syscall.Stat_t)
// 	if !ok {
// 		return fmt.Errorf("logrotatex: unable to get file system status information")
// 	}

// 	// 获取源文件的用户 ID 和组 ID
// 	uid := int(stat.Uid)
// 	gid := int(stat.Gid)

// 	// 直接更改目标文件的所有者和所属组
// 	if err := os.Chown(name, uid, gid); err != nil {
// 		return fmt.Errorf("logrotatex: unable to set file owner: %w", err)
// 	}

// 	return nil
// }
