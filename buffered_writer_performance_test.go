// buffered_writer_performance_test.go - BufferedWriter性能测试用例
// 测试BufferedWriter在不同场景下的性能表现，包括写入性能、并发性能、不同配置下的性能对比等
package logrotatex

import (
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// BenchmarkBufferedWriter_Write 测试基本写入性能
func BenchmarkBufferedWriter_Write(b *testing.B) {
	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, nil)
	defer func() {
		if err := bw.Close(); err != nil {
			b.Errorf("Close failed: %v", err)
		}
	}()

	// 准备测试数据
	testData := []byte(strings.Repeat("This is a benchmark test line.\n", 10))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := bw.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

// BenchmarkBufferedWriter_ConcurrentWrite 测试并发写入性能
func BenchmarkBufferedWriter_ConcurrentWrite(b *testing.B) {
	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, nil)
	defer func() {
		if err := bw.Close(); err != nil {
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
				_, err := bw.Write(testData)
				if err != nil {
					b.Errorf("Write failed: %v", err)
				}
			}
		}()
	}
	wg.Wait()
}

// BenchmarkBufferedWriter_DirectWrite 对比直接写入性能
func BenchmarkBufferedWriter_DirectWrite(b *testing.B) {
	mock := &mockWriter{}

	// 准备测试数据
	testData := []byte(strings.Repeat("This is a direct write benchmark test line.\n", 10))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := mock.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

// BenchmarkBufferedWriter_SmallBuffer 测试小缓冲区性能
func BenchmarkBufferedWriter_SmallBuffer(b *testing.B) {
	config := &BufCfg{
		MaxBufferSize: 1 * 1024, // 1KB
		MaxWriteCount: 100,
		FlushInterval: 1 * time.Second,
	}

	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, config)
	defer func() {
		if err := bw.Close(); err != nil {
			b.Errorf("Close failed: %v", err)
		}
	}()

	testData := []byte(strings.Repeat("This is a small buffer benchmark test line.\n", 10))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := bw.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

// BenchmarkBufferedWriter_LargeBuffer 测试大缓冲区性能
func BenchmarkBufferedWriter_LargeBuffer(b *testing.B) {
	config := &BufCfg{
		MaxBufferSize: 256 * 1024, // 256KB
		MaxWriteCount: 1000,
		FlushInterval: 5 * time.Second,
	}

	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, config)
	defer func() {
		if err := bw.Close(); err != nil {
			b.Errorf("Close failed: %v", err)
		}
	}()

	testData := []byte(strings.Repeat("This is a large buffer benchmark test line.\n", 10))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := bw.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

// BenchmarkBufferedWriter_FrequentFlush 测试频繁刷新性能
func BenchmarkBufferedWriter_FrequentFlush(b *testing.B) {
	config := &BufCfg{
		MaxBufferSize: 64 * 1024, // 64KB
		MaxWriteCount: 10,        // 10次写入触发刷新
		FlushInterval: 1 * time.Second,
	}

	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, config)
	defer func() {
		if err := bw.Close(); err != nil {
			b.Errorf("Close failed: %v", err)
		}
	}()

	testData := []byte(strings.Repeat("This is a frequent flush benchmark test line.\n", 10))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := bw.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

// BenchmarkBufferedWriter_InfrequentFlush 测试不频繁刷新性能
func BenchmarkBufferedWriter_InfrequentFlush(b *testing.B) {
	config := &BufCfg{
		MaxBufferSize: 64 * 1024, // 64KB
		MaxWriteCount: 1000,      // 1000次写入触发刷新
		FlushInterval: 10 * time.Second,
	}

	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, config)
	defer func() {
		if err := bw.Close(); err != nil {
			b.Errorf("Close failed: %v", err)
		}
	}()

	testData := []byte(strings.Repeat("This is an infrequent flush benchmark test line.\n", 10))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := bw.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

// BenchmarkBufferedWriter_TimeBasedFlush 测试基于时间的刷新性能
func BenchmarkBufferedWriter_TimeBasedFlush(b *testing.B) {
	config := &BufCfg{
		MaxBufferSize: 1024 * 1024,            // 1MB，避免大小触发
		MaxWriteCount: 10000,                  // 大次数，避免次数触发
		FlushInterval: 100 * time.Millisecond, // 100ms刷新间隔
	}

	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, config)
	defer func() {
		if err := bw.Close(); err != nil {
			b.Errorf("Close failed: %v", err)
		}
	}()

	testData := []byte(strings.Repeat("This is a time-based flush benchmark test line.\n", 10))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := bw.Write(testData)
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
}

// TestBufferedWriter_PerformanceComparison 性能对比测试
// 比较不同配置下的性能表现
func TestBufferedWriter_PerformanceComparison(t *testing.T) {
	// 准备测试数据
	testData := []byte(strings.Repeat("This is a performance comparison test line.\n", 100))
	numWrites := 10000

	// 测试场景1: 直接写入
	func() {
		mock := &mockWriter{}

		start := time.Now()
		for i := 0; i < numWrites; i++ {
			_, err := mock.Write(testData)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}
		duration := time.Since(start)
		t.Logf("Direct write: %d writes in %v (%.2f writes/sec, %d calls)",
			numWrites, duration, float64(numWrites)/duration.Seconds(), mock.WriteCalls())
	}()

	// 测试场景2: 默认缓冲写入器
	func() {
		mock := &mockWriter{}
		bw := NewBufferedWriter(mock, nil)
		defer func() {
			if err := bw.Close(); err != nil {
				t.Errorf("Close failed: %v", err)
			}
		}()

		start := time.Now()
		for i := 0; i < numWrites; i++ {
			_, err := bw.Write(testData)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}
		duration := time.Since(start)
		t.Logf("Default buffered: %d writes in %v (%.2f writes/sec, %d calls)",
			numWrites, duration, float64(numWrites)/duration.Seconds(), mock.WriteCalls())
	}()

	// 测试场景3: 小缓冲区
	func() {
		config := &BufCfg{
			MaxBufferSize: 4 * 1024, // 4KB
			MaxWriteCount: 50,
			FlushInterval: 1 * time.Second,
		}
		mock := &mockWriter{}
		bw := NewBufferedWriter(mock, config)
		defer func() {
			if err := bw.Close(); err != nil {
				t.Errorf("Close failed: %v", err)
			}
		}()

		start := time.Now()
		for i := 0; i < numWrites; i++ {
			_, err := bw.Write(testData)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}
		duration := time.Since(start)
		t.Logf("Small buffer: %d writes in %v (%.2f writes/sec, %d calls)",
			numWrites, duration, float64(numWrites)/duration.Seconds(), mock.WriteCalls())
	}()

	// 测试场景4: 大缓冲区
	func() {
		config := &BufCfg{
			MaxBufferSize: 1024 * 1024, // 1MB
			MaxWriteCount: 1000,
			FlushInterval: 10 * time.Second,
		}
		mock := &mockWriter{}
		bw := NewBufferedWriter(mock, config)
		defer func() {
			if err := bw.Close(); err != nil {
				t.Errorf("Close failed: %v", err)
			}
		}()

		start := time.Now()
		for i := 0; i < numWrites; i++ {
			_, err := bw.Write(testData)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}
		duration := time.Since(start)
		t.Logf("Large buffer: %d writes in %v (%.2f writes/sec, %d calls)",
			numWrites, duration, float64(numWrites)/duration.Seconds(), mock.WriteCalls())
	}()
}

// TestBufferedWriter_ConcurrentPerformance 并发性能测试
func TestBufferedWriter_ConcurrentPerformance(t *testing.T) {
	// 准备测试数据
	testData := []byte(strings.Repeat("This is a concurrent performance test line.\n", 10))
	numGoroutines := []int{1, 2, 4, 8, 16}
	writesPerGoroutine := 1000

	for _, num := range numGoroutines {
		mock := &mockWriter{}
		bw := NewBufferedWriter(mock, nil)

		start := time.Now()

		var wg sync.WaitGroup
		for i := 0; i < num; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < writesPerGoroutine; j++ {
					_, err := bw.Write(testData)
					if err != nil {
						t.Errorf("Write failed: %v", err)
					}
				}
			}()
		}
		wg.Wait()

		duration := time.Since(start)
		totalWrites := num * writesPerGoroutine
		t.Logf("%d goroutines: %d writes in %v (%.2f writes/sec, %d calls)",
			num, totalWrites, duration, float64(totalWrites)/duration.Seconds(), mock.WriteCalls())

		if err := bw.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}
}

// TestBufferedWriter_MemoryUsage 内存使用测试
func TestBufferedWriter_MemoryUsage(t *testing.T) {
	// 获取初始内存状态
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// 创建缓冲写入器
	config := &BufCfg{
		MaxBufferSize: 64 * 1024, // 64KB
		MaxWriteCount: 500,
		FlushInterval: 1 * time.Second,
	}
	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, config)
	defer func() {
		if err := bw.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 执行大量写入操作
	testData := []byte(strings.Repeat("This is a memory usage test line.\n", 100))
	numWrites := 10000

	for i := 0; i < numWrites; i++ {
		_, err := bw.Write(testData)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// 获取最终内存状态
	runtime.GC()
	runtime.ReadMemStats(&m2)

	// 计算内存使用情况
	allocDiff := m2.TotalAlloc - m1.TotalAlloc
	t.Logf("Memory usage for %d writes: %d bytes (%.2f KB per write, %d calls)",
		numWrites, allocDiff, float64(allocDiff)/float64(numWrites)/1024, mock.WriteCalls())
}

// TestBufferedWriter_FlushTriggerEfficiency 刷新触发效率测试
func TestBufferedWriter_FlushTriggerEfficiency(t *testing.T) {
	// 准备测试数据
	testData := []byte(strings.Repeat("This is a flush trigger efficiency test line.\n", 10))
	numWrites := 1000

	// 测试场景1: 缓冲区大小触发
	func() {
		config := &BufCfg{
			MaxBufferSize: len(testData) * 10, // 10次写入触发
			MaxWriteCount: 1000,
			FlushInterval: 10 * time.Second,
		}
		mock := &mockWriter{}
		bw := NewBufferedWriter(mock, config)
		defer func() {
			if err := bw.Close(); err != nil {
				t.Errorf("Close failed: %v", err)
			}
		}()

		start := time.Now()
		for i := 0; i < numWrites; i++ {
			_, err := bw.Write(testData)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}
		duration := time.Since(start)
		t.Logf("Buffer size trigger: %d writes in %v (%.2f writes/sec, %d calls)",
			numWrites, duration, float64(numWrites)/duration.Seconds(), mock.WriteCalls())
	}()

	// 测试场景2: 写入次数触发
	func() {
		config := &BufCfg{
			MaxBufferSize: 1024 * 1024, // 1MB，避免大小触发
			MaxWriteCount: 10,          // 10次写入触发
			FlushInterval: 10 * time.Second,
		}
		mock := &mockWriter{}
		bw := NewBufferedWriter(mock, config)
		defer func() {
			if err := bw.Close(); err != nil {
				t.Errorf("Close failed: %v", err)
			}
		}()

		start := time.Now()
		for i := 0; i < numWrites; i++ {
			_, err := bw.Write(testData)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}
		duration := time.Since(start)
		t.Logf("Write count trigger: %d writes in %v (%.2f writes/sec, %d calls)",
			numWrites, duration, float64(numWrites)/duration.Seconds(), mock.WriteCalls())
	}()

	// 测试场景3: 时间间隔触发
	func() {
		config := &BufCfg{
			MaxBufferSize: 1024 * 1024,           // 1MB，避免大小触发
			MaxWriteCount: 10000,                 // 大次数，避免次数触发
			FlushInterval: 50 * time.Millisecond, // 50ms刷新间隔
		}
		mock := &mockWriter{}
		bw := NewBufferedWriter(mock, config)
		defer func() {
			if err := bw.Close(); err != nil {
				t.Errorf("Close failed: %v", err)
			}
		}()

		start := time.Now()
		for i := 0; i < numWrites; i++ {
			_, err := bw.Write(testData)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
			// 添加小延迟，确保时间触发
			time.Sleep(1 * time.Millisecond)
		}
		duration := time.Since(start)
		t.Logf("Time interval trigger: %d writes in %v (%.2f writes/sec, %d calls)",
			numWrites, duration, float64(numWrites)/duration.Seconds(), mock.WriteCalls())
	}()
}

// TestBufferedWriter_RealWorldScenario 真实场景性能测试
func TestBufferedWriter_RealWorldScenario(t *testing.T) {
	// 模拟真实日志场景：不同大小的日志消息
	logMessages := [][]byte{
		[]byte("INFO: Application started\n"),
		[]byte("DEBUG: Processing request #12345 with parameters: param1=value1, param2=value2, param3=value3\n"),
		[]byte("WARN: High memory usage detected: current=85%, threshold=80%\n"),
		[]byte("ERROR: Database connection failed: connection timeout after 30 seconds\n"),
		[]byte("INFO: User login: user_id=12345, ip=192.168.1.100, timestamp=2023-01-01T12:00:00Z\n"),
		[]byte("DEBUG: Cache hit: key=user_session_12345, ttl=3600\n"),
		[]byte("WARN: Rate limit exceeded: ip=192.168.1.100, limit=100/min, current=150\n"),
		[]byte("ERROR: File not found: path=/var/log/app.log, error=no such file or directory\n"),
	}

	numWrites := 10000

	// 测试场景1: 直接写入
	func() {
		mock := &mockWriter{}

		start := time.Now()
		for i := 0; i < numWrites; i++ {
			msg := logMessages[i%len(logMessages)]
			_, err := mock.Write(msg)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}
		duration := time.Since(start)
		t.Logf("Direct write (real world): %d writes in %v (%.2f writes/sec, %d calls)",
			numWrites, duration, float64(numWrites)/duration.Seconds(), mock.WriteCalls())
	}()

	// 测试场景2: 缓冲写入器
	func() {
		config := &BufCfg{
			MaxBufferSize: 64 * 1024, // 64KB
			MaxWriteCount: 100,
			FlushInterval: 1 * time.Second,
		}
		mock := &mockWriter{}
		bw := NewBufferedWriter(mock, config)
		defer func() {
			if err := bw.Close(); err != nil {
				t.Errorf("Close failed: %v", err)
			}
		}()

		start := time.Now()
		for i := 0; i < numWrites; i++ {
			msg := logMessages[i%len(logMessages)]
			_, err := bw.Write(msg)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
		}
		duration := time.Since(start)
		t.Logf("Buffered write (real world): %d writes in %v (%.2f writes/sec, %d calls)",
			numWrites, duration, float64(numWrites)/duration.Seconds(), mock.WriteCalls())
	}()
}
