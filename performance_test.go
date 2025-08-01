package logrotatex

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFileScanning_Performance 测试文件扫描性能优化效果
func TestFileScanning_Performance(t *testing.T) {
	tempDir := t.TempDir()

	// 创建大量测试文件
	fileCount := 1000
	prefix := "app"
	ext := ".log"

	t.Logf("创建 %d 个测试文件...", fileCount)

	// 创建测试文件
	for i := 0; i < fileCount; i++ {
		timestamp := time.Now().Add(-time.Duration(i) * time.Hour)
		filename := fmt.Sprintf("%s_%s%s", prefix, timestamp.Format(backupTimeFormat), ext)
		filePath := filepath.Join(tempDir, filename)

		file, err := os.Create(filePath)
		if err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
		if _, err := fmt.Fprintf(file, "测试数据 %d", i); err != nil {
			if closeErr := file.Close(); closeErr != nil {
				t.Logf("关闭文件时出错: %v", closeErr)
			}
			t.Fatalf("写入测试文件失败: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("关闭测试文件失败: %v", err)
		}
	}

	// 创建LogRotateX实例
	logPath := filepath.Join(tempDir, "app.log")
	logger := &LogRotateX{
		Filename:   logPath,
		MaxSize:    1,
		MaxBackups: 10,
		MaxAge:     30,
	}

	// 性能测试
	t.Run("优化后的文件扫描性能", func(t *testing.T) {
		start := time.Now()

		// 执行多次扫描以获得平均性能
		iterations := 10
		for i := 0; i < iterations; i++ {
			files, err := logger.oldLogFiles()
			if err != nil {
				t.Fatalf("文件扫描失败: %v", err)
			}

			if i == 0 {
				t.Logf("扫描到 %d 个日志文件", len(files))
			}
		}

		elapsed := time.Since(start)
		avgTime := elapsed / time.Duration(iterations)

		t.Logf("总耗时: %v", elapsed)
		t.Logf("平均单次扫描耗时: %v", avgTime)
		t.Logf("处理 %d 个文件的平均性能: %.2f 文件/毫秒",
			fileCount, float64(fileCount)/float64(avgTime.Nanoseconds())*1000000)

		// 性能断言：平均单次扫描应该在合理时间内完成
		if avgTime > 100*time.Millisecond {
			t.Errorf("文件扫描性能不达标，期望 < 100ms，实际: %v", avgTime)
		}
	})
}

// TestGoroutineLifecycle_Management 测试Goroutine生命周期管理
func TestGoroutineLifecycle_Management(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "lifecycle_test.log")

	t.Run("Goroutine正确启动和关闭", func(t *testing.T) {
		logger := NewLogRotateX(logPath)
		logger.MaxSize = 1 // 1MB
		logger.Compress = true

		// 写入数据触发mill goroutine启动
		data := make([]byte, 1024)
		for i := range data {
			data[i] = 'A'
		}

		_, err := logger.Write(data)
		if err != nil {
			t.Fatalf("写入失败: %v", err)
		}

		// 验证mill goroutine已启动
		if !logger.millStarted.Load() {
			t.Error("mill goroutine未正确启动")
		}

		// 测试关闭操作
		start := time.Now()
		err = logger.Close()
		closeTime := time.Since(start)

		if err != nil {
			t.Fatalf("关闭失败: %v", err)
		}

		t.Logf("关闭耗时: %v", closeTime)

		// 验证mill goroutine已停止
		if logger.millStarted.Load() {
			t.Error("mill goroutine未正确停止")
		}

		// 关闭时间应该在合理范围内
		if closeTime > 6*time.Second {
			t.Errorf("关闭耗时过长，期望 < 6s，实际: %v", closeTime)
		}
	})

	t.Run("多次关闭操作安全性", func(t *testing.T) {
		logger := NewLogRotateX(logPath)

		// 写入一些数据
		if _, err := logger.Write([]byte("测试数据")); err != nil {
			t.Fatalf("写入数据失败: %v", err)
		}

		// 多次调用Close应该是安全的
		for i := 0; i < 3; i++ {
			err := logger.Close()
			if err != nil {
				t.Fatalf("第 %d 次关闭失败: %v", i+1, err)
			}
		}
	})

	t.Run("并发关闭操作安全性", func(t *testing.T) {
		logger := NewLogRotateX(logPath)
		if _, err := logger.Write([]byte("测试数据")); err != nil {
			t.Fatalf("写入数据失败: %v", err)
		}

		// 并发调用Close
		done := make(chan error, 5)
		for i := 0; i < 5; i++ {
			go func() {
				done <- logger.Close()
			}()
		}

		// 等待所有goroutine完成
		for i := 0; i < 5; i++ {
			err := <-done
			if err != nil {
				t.Errorf("并发关闭失败: %v", err)
			}
		}
	})
}
