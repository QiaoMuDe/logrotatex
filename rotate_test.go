//go:build linux
// +build linux

package logrotatex

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// Example of how to rotate in response to SIGHUP.
// 此函数展示了如何响应 SIGHUP 信号进行日志轮转。
func ExampleLogRotateX_Rotate() {
	// 创建一个 LogRotateX 实例
	l := &LogRotateX{}
	// 将标准日志输出设置为 LogRotateX 实例
	log.SetOutput(l)
	// 创建一个信号通道，缓冲区大小为 1
	c := make(chan os.Signal, 1)
	// 通知信号通道监听 SIGHUP 信号
	signal.Notify(c, syscall.SIGHUP)

	// 启动一个 goroutine 来处理接收到的信号
	go func() {
		for {
			// 阻塞等待信号
			<-c
			// 当接收到 SIGHUP 信号时，调用 LogRotateX 实例的 Rotate 方法进行日志轮转
			l.Rotate()
		}
	}()
}
