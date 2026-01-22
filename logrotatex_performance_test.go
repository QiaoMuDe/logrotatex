// logrotatex_performance_test.go - LogRotateX性能测试用例
// 测试LogRotateX在不同场景下的性能表现，包括写入性能、轮转性能、并发性能等
package logrotatex

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// BenchmarkLogRotateX_Write 测试基本写入性能
func BenchmarkLogRotateX_Write(b *testing.B) {
	dir := makeTempDir("BenchmarkLogRotateX_Write", b)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(filepath.Join(dir, "bench.log"))
	defer func() {
		if err := logger.Close(); err != nil {
			b.Errorf("Close failed: %v", err)
		}
	}()

	// 准备测试数据
	testData := []byte(strings.Repeat("This is a benchmark test line.\n", 10))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := logger.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

// BenchmarkLogRotateX_ConcurrentWrite 测试并发写入性能
func BenchmarkLogRotateX_ConcurrentWrite(b *testing.B) {
	dir := makeTempDir("BenchmarkLogRotateX_ConcurrentWrite", b)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(filepath.Join(dir, "bench.log"))
	defer func() {
		if err := logger.Close(); err != nil {
			b.Errorf("Close failed: %v", err)
		}
	}()

	// 准备测试数据
	testData := []byte(strings.Repeat("This is a concurrent benchmark test line.\n", 10))

	// 根据CPU核心数设置并发数
	numGoroutines := runtime.NumCPU()
	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < b.N/numGoroutines; j++ {
				_, err := logger.Write(testData)
				if err != nil {
					b.Errorf("Write failed: %v", err)
				}
			}
		}()
	}
	wg.Wait()
}

// BenchmarkLogRotateX_Rotation 测试轮转性能
func BenchmarkLogRotateX_Rotation(b *testing.B) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("BenchmarkLogRotateX_Rotation", b)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(filepath.Join(dir, "bench.log"))
	logger.MaxSize = 1 // 1MB，便于触发轮转
	defer func() {
		if err := logger.Close(); err != nil {
			b.Errorf("Close failed: %v", err)
		}
	}()

	// 准备较大的测试数据，便于触发轮转
	testData := []byte(strings.Repeat("This is a rotation benchmark test line.\n", 100))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := logger.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

// BenchmarkLogRotateX_RotationWithCompression 测试带压缩的轮转性能
func BenchmarkLogRotateX_RotationWithCompression(b *testing.B) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("BenchmarkLogRotateX_RotationWithCompression", b)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(filepath.Join(dir, "bench.log"))
	logger.MaxSize = 1 // 1MB，便于触发轮转
	logger.Compress = true
	defer func() {
		if err := logger.Close(); err != nil {
			b.Errorf("Close failed: %v", err)
		}
	}()

	// 准备较大的测试数据，便于触发轮转
	testData := []byte(strings.Repeat("This is a compression rotation benchmark test line.\n", 100))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := logger.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

// BenchmarkLogRotateX_AsyncCleanup 测试异步清理性能
func BenchmarkLogRotateX_AsyncCleanup(b *testing.B) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("BenchmarkLogRotateX_AsyncCleanup", b)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(filepath.Join(dir, "bench.log"))
	logger.MaxSize = 1  // 1MB，便于触发轮转
	logger.Async = true // 启用异步清理
	logger.MaxFiles = 5 // 限制文件数量，触发清理
	defer func() {
		if err := logger.Close(); err != nil {
			b.Errorf("Close failed: %v", err)
		}
	}()

	// 准备较大的测试数据，便于触发轮转
	testData := []byte(strings.Repeat("This is an async cleanup benchmark test line.\n", 100))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := logger.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

// BenchmarkLogRotateX_DailyRotation 测试按天轮转性能
func BenchmarkLogRotateX_DailyRotation(b *testing.B) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	dayCounter := 0
	currentTime = func() time.Time {
		// 每次调用增加一天，模拟按天轮转
		day := dayCounter % 30
		dayCounter++
		return time.Date(2023, 1, day+1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("BenchmarkLogRotateX_DailyRotation", b)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(filepath.Join(dir, "bench.log"))
	logger.RotateByDay = true
	defer func() {
		if err := logger.Close(); err != nil {
			b.Errorf("Close failed: %v", err)
		}
	}()

	// 准备测试数据
	testData := []byte(strings.Repeat("This is a daily rotation benchmark test line.\n", 10))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := logger.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

// BenchmarkLogRotateX_DateDirectoryLayout 测试日期目录布局性能
func BenchmarkLogRotateX_DateDirectoryLayout(b *testing.B) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("BenchmarkLogRotateX_DateDirectoryLayout", b)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(filepath.Join(dir, "bench.log"))
	logger.MaxSize = 1 // 1MB，便于触发轮转
	logger.DateDirLayout = true
	defer func() {
		if err := logger.Close(); err != nil {
			b.Errorf("Close failed: %v", err)
		}
	}()

	// 准备较大的测试数据，便于触发轮转
	testData := []byte(strings.Repeat("This is a date directory layout benchmark test line.\n", 100))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := logger.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

// TestLogRotateX_PerformanceComparison 性能对比测试
// 比较不同配置下的性能表现
func TestLogRotateX_PerformanceComparison(t *testing.T) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	// 准备测试数据
	testData := []byte(strings.Repeat("This is a performance comparison test line.\n", 100))
	numWrites := 10000

	// 测试场景1: 基本写入
	func() {
		dir := makeTempDir("TestLogRotateX_PerformanceComparison_Basic", t)
		defer func() { _ = os.RemoveAll(dir) }()

		logger := NewLogRotateX(filepath.Join(dir, "bench.log"))
		defer func() {
			if err := logger.Close(); err != nil {
				t.Errorf("Close failed: %v", err)
			}
		}()

		start := time.Now()
		for i := 0; i < numWrites; i++ {
			_, err := logger.Write(testData)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}
		duration := time.Since(start)
		t.Logf("Basic write: %d writes in %v (%.2f writes/sec)", numWrites, duration, float64(numWrites)/duration.Seconds())
	}()

	// 测试场景2: 带轮转
	func() {
		dir := makeTempDir("TestLogRotateX_PerformanceComparison_Rotation", t)
		defer func() { _ = os.RemoveAll(dir) }()

		logger := NewLogRotateX(filepath.Join(dir, "bench.log"))
		logger.MaxSize = 1 // 1MB，便于触发轮转
		defer func() {
			if err := logger.Close(); err != nil {
				t.Errorf("Close failed: %v", err)
			}
		}()

		start := time.Now()
		for i := 0; i < numWrites; i++ {
			_, err := logger.Write(testData)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}
		duration := time.Since(start)
		t.Logf("With rotation: %d writes in %v (%.2f writes/sec)", numWrites, duration, float64(numWrites)/duration.Seconds())
	}()

	// 测试场景3: 带压缩
	func() {
		dir := makeTempDir("TestLogRotateX_PerformanceComparison_Compression", t)
		defer func() { _ = os.RemoveAll(dir) }()

		logger := NewLogRotateX(filepath.Join(dir, "bench.log"))
		logger.MaxSize = 1 // 1MB，便于触发轮转
		logger.Compress = true
		defer func() {
			if err := logger.Close(); err != nil {
				t.Errorf("Close failed: %v", err)
			}
		}()

		start := time.Now()
		for i := 0; i < numWrites; i++ {
			_, err := logger.Write(testData)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}
		duration := time.Since(start)
		t.Logf("With compression: %d writes in %v (%.2f writes/sec)", numWrites, duration, float64(numWrites)/duration.Seconds())
	}()

	// 测试场景4: 异步清理
	func() {
		dir := makeTempDir("TestLogRotateX_PerformanceComparison_Async", t)
		defer func() { _ = os.RemoveAll(dir) }()

		logger := NewLogRotateX(filepath.Join(dir, "bench.log"))
		logger.MaxSize = 1 // 1MB，便于触发轮转
		logger.Async = true
		logger.MaxFiles = 5
		defer func() {
			if err := logger.Close(); err != nil {
				t.Errorf("Close failed: %v", err)
			}
		}()

		start := time.Now()
		for i := 0; i < numWrites; i++ {
			_, err := logger.Write(testData)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}
		duration := time.Since(start)
		t.Logf("With async cleanup: %d writes in %v (%.2f writes/sec)", numWrites, duration, float64(numWrites)/duration.Seconds())
	}()
}

// TestLogRotateX_MemoryUsage 内存使用测试
func TestLogRotateX_MemoryUsage(t *testing.T) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("TestLogRotateX_MemoryUsage", t)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(filepath.Join(dir, "bench.log"))
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 获取初始内存状态
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// 执行大量写入操作
	testData := []byte(strings.Repeat("This is a memory usage test line.\n", 100))
	numWrites := 10000

	for i := 0; i < numWrites; i++ {
		_, err := logger.Write(testData)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// 获取最终内存状态
	runtime.GC()
	runtime.ReadMemStats(&m2)

	// 计算内存使用情况
	allocDiff := m2.TotalAlloc - m1.TotalAlloc
	t.Logf("Memory usage for %d writes: %d bytes (%.2f KB per write)",
		numWrites, allocDiff, float64(allocDiff)/float64(numWrites)/1024)
}

// TestLogRotateX_ConcurrentPerformance 并发性能测试
func TestLogRotateX_ConcurrentPerformance(t *testing.T) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("TestLogRotateX_ConcurrentPerformance", t)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(filepath.Join(dir, "bench.log"))
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 准备测试数据
	testData := []byte(strings.Repeat("This is a concurrent performance test line.\n", 10))
	numGoroutines := []int{1, 2, 4, 8, 16}
	writesPerGoroutine := 1000

	for _, num := range numGoroutines {
		start := time.Now()

		var wg sync.WaitGroup
		for i := 0; i < num; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < writesPerGoroutine; j++ {
					_, err := logger.Write(testData)
					if err != nil {
						t.Errorf("Write failed: %v", err)
					}
				}
			}()
		}
		wg.Wait()

		duration := time.Since(start)
		totalWrites := num * writesPerGoroutine
		t.Logf("%d goroutines: %d writes in %v (%.2f writes/sec)",
			num, totalWrites, duration, float64(totalWrites)/duration.Seconds())
	}
}
