//go:build !linux
// +build !linux

package logrotatex

import (
	"os"
)

// chown 函数用于处理文件的权限变更操作。
// 在非 Linux 系统下，该函数直接返回 nil，不进行实际的权限变更。
// 参数 path 为文件的路径，在非 Linux 系统下该参数会被忽略。
// 参数 info 为文件的信息，在非 Linux 系统下该参数会被忽略。
// 返回值为错误信息，在非 Linux 系统下始终返回 nil。
func chown(_ string, _ os.FileInfo) error {
	return nil
}
