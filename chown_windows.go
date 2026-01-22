// chown_windows.go 提供了Windows系统下的文件所有者变更功能的空实现。
// 由于Windows系统的权限管理机制与Unix系统不同，该文件中的chown函数
// 为空操作，确保代码在Windows平台上能够正常编译和运行。
//go:build windows
// +build windows

package logrotatex

// import (
// 	"os"
// )

// // chown 在 Windows 系统下为空操作，始终返回 nil。
// // Windows 系统的权限管理机制与 Unix 系统不同。
// func chown(_ string, _ os.FileInfo) error {
// 	return nil
// }
