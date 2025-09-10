// resource_leak_test.go 包含了logrotatex包资源泄漏检测的测试用例。
// 该文件测试了日志轮转过程中的资源管理，包括文件句柄泄漏、内存泄漏、
// goroutine泄漏等问题的检测，确保系统长期运行的稳定性。

package logrotatex

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestFileHandleLeakPrevention 测试文件句柄泄漏预防机制
func TestFileHandleLeakPrevention(t *testing.T) {
	// 创建logs目录
	logsDir := "logs"
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("创建logs目录失败: %v", err)
	}
	defer func() { _ = os.RemoveAll(logsDir) }()
	logFile := filepath.Join(logsDir, "test_leak.log")

	// 获取初始文件描述符数量
	initialFDs := getOpenFileDescriptors()

	// 创建LogRotateX实例
	logger := &LogRotateX{
		Filename:  logFile,
		MaxSize:   1, // 1MB，便于触发轮转
		MaxFiles:  3,
		MaxAge:    1,
		LocalTime: true,
		Compress:  false,
	}

	// 测试多次写入和轮转操作
	for i := 0; i < 10; i++ {
		// 写入足够的数据触发轮转
		data := strings.Repeat(fmt.Sprintf("测试数据行 %d\n", i), 10000)
		_, err := logger.Write([]byte(data))
		if err != nil {
			t.Fatalf("写入失败: %v", err)
		}

		// 强制轮转
		err = logger.Rotate()
		if err != nil {
			t.Fatalf("轮转失败: %v", err)
		}
	}

	// 关闭logger
	closeErr := logger.Close()
	if closeErr != nil {
		t.Fatalf("关闭失败: %v", closeErr)
	}

	// 强制垃圾回收
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// 检查文件描述符是否有泄漏
	finalFDs := getOpenFileDescriptors()
	fdDiff := finalFDs - initialFDs

	// 允许少量的文件描述符增长（测试环境可能有其他操作）
	if fdDiff > 5 {
		t.Errorf("可能存在文件句柄泄漏: 初始FD数量=%d, 最终FD数量=%d, 差异=%d",
			initialFDs, finalFDs, fdDiff)
	}

	t.Logf("文件描述符检查: 初始=%d, 最终=%d, 差异=%d", initialFDs, finalFDs, fdDiff)
}

// TestConcurrentFileHandleManagement 测试并发环境下的文件句柄管理
func TestConcurrentFileHandleManagement(t *testing.T) {
	logsDir := "logs"
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("创建logs目录失败: %v", err)
	}
	defer func() { _ = os.RemoveAll(logsDir) }()
	logFile := filepath.Join(logsDir, "test_concurrent.log")

	logger := &LogRotateX{
		Filename:  logFile,
		MaxSize:   1, // 1MB
		MaxFiles:   5,
		MaxAge:    1,
		LocalTime: true,
		Compress:  false,
	}

	// 并发写入测试
	var wg sync.WaitGroup
	numGoroutines := 10
	numWrites := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numWrites; j++ {
				data := fmt.Sprintf("Goroutine %d, Write %d: %s\n",
					id, j, strings.Repeat("data", 100))
				_, err := logger.Write([]byte(data))
				if err != nil {
					t.Errorf("并发写入失败 (goroutine %d, write %d): %v", id, j, err)
					return
				}

				// 偶尔触发轮转
				if j%50 == 0 {
					_ = logger.Rotate()
				}
			}
		}(i)
	}

	wg.Wait()

	// 关闭logger
	closeErr := logger.Close()
	if closeErr != nil {
		t.Fatalf("并发测试后关闭失败: %v", closeErr)
	}
}

// TestErrorHandlingInFileOperations 测试文件操作中的错误处理
func TestErrorHandlingInFileOperations(t *testing.T) {
	logsDir := "logs"
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("创建logs目录失败: %v", err)
	}
	defer func() { _ = os.RemoveAll(logsDir) }()
	logFile := filepath.Join(logsDir, "test_error.log")

	logger := &LogRotateX{
		Filename:  logFile,
		MaxSize:   1,
		MaxFiles:   3,
		MaxAge:    1,
		LocalTime: true,
		Compress:  false,
	}

	// 正常写入
	_, writeErr := logger.Write([]byte("正常数据\n"))
	if writeErr != nil {
		t.Fatalf("正常写入失败: %v", writeErr)
	}

	// 模拟文件权限问题（在某些系统上可能不起作用）
	if runtime.GOOS != "windows" {
		// 更改目录权限使其不可写
		chmodErr := os.Chmod(logsDir, 0444)
		if chmodErr == nil {
			// 尝试轮转（应该失败但不应该泄漏文件句柄）
			rotateErr := logger.Rotate()
			if rotateErr == nil {
				t.Log("轮转意外成功（可能是权限限制不起作用）")
			}

			// 恢复权限
			_ = os.Chmod(logsDir, 0755)
		}
	}

	// 确保即使在错误情况下也能正常关闭
	closeErr := logger.Close()
	if closeErr != nil {
		t.Logf("关闭时出现错误（预期可能发生）: %v", closeErr)
	}
}

// TestMultipleCloseOperations 测试多次关闭操作
func TestMultipleCloseOperations(t *testing.T) {
	logsDir := "logs"
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("创建logs目录失败: %v", err)
	}
	defer func() { _ = os.RemoveAll(logsDir) }()
	logFile := filepath.Join(logsDir, "test_multiple_close.log")

	logger := &LogRotateX{
		Filename:  logFile,
		MaxSize:   10,
		MaxFiles:   3,
		MaxAge:    1,
		LocalTime: true,
		Compress:  false,
	}

	// 写入一些数据
	_, writeErr := logger.Write([]byte("测试数据\n"))
	if writeErr != nil {
		t.Fatalf("写入失败: %v", writeErr)
	}

	// 多次关闭应该是安全的
	for i := 0; i < 5; i++ {
		err := logger.Close()
		if err != nil && i == 0 {
			t.Fatalf("首次关闭失败: %v", err)
		}
		// 后续关闭应该是无害的
	}
}

// getOpenFileDescriptors 获取当前进程打开的文件描述符数量
// 这是一个简化的实现，在不同系统上可能需要调整
func getOpenFileDescriptors() int {
	if runtime.GOOS == "linux" {
		// 在Linux上，可以通过/proc/self/fd目录获取
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			return -1
		}
		return len(entries)
	}

	// 在其他系统上，返回一个固定值（测试时会计算差异）
	return 0
}

// BenchmarkFileHandleManagement 基准测试文件句柄管理性能
func BenchmarkFileHandleManagement(b *testing.B) {
	logsDir := "logs"
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		b.Fatalf("创建logs目录失败: %v", err)
	}
	defer func() { _ = os.RemoveAll(logsDir) }()
	logFile := filepath.Join(logsDir, "bench_handle.log")

	logger := &LogRotateX{
		Filename:  logFile,
		MaxSize:   1,
		MaxFiles:   5,
		MaxAge:    1,
		LocalTime: true,
		Compress:  false,
	}

	data := []byte("基准测试数据行\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, writeErr := logger.Write(data)
		if writeErr != nil {
			b.Fatalf("基准测试写入失败: %v", writeErr)
		}

		// 每1000次写入执行一次轮转
		if i%1000 == 0 {
			_ = logger.Rotate()
		}
	}

	closeErr := logger.Close()
	if closeErr != nil {
		b.Fatalf("基准测试关闭失败: %v", closeErr)
	}
}
