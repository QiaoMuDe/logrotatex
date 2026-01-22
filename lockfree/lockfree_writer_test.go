// lockfree_writer_test.go - 无锁写入器测试用例
package lockfree

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// mockWriter 模拟写入器，用于测试
type mockWriter struct {
	data   []byte
	mutex  sync.Mutex
	closed bool
}

func newMockWriter() *mockWriter {
	return &mockWriter{
		data: make([]byte, 0),
	}
}

func (mw *mockWriter) Write(p []byte) (n int, err error) {
	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	if mw.closed {
		return 0, fmt.Errorf("writer is closed")
	}

	mw.data = append(mw.data, p...)
	return len(p), nil
}

func (mw *mockWriter) Close() error {
	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	mw.closed = true
	return nil
}

func (mw *mockWriter) Data() []byte {
	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	result := make([]byte, len(mw.data))
	copy(result, mw.data)
	return result
}

func (mw *mockWriter) Reset() {
	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	mw.data = mw.data[:0]
}

// TestLockFreeBufferedWriter_BasicWrite 测试基本写入功能
func TestLockFreeBufferedWriter_BasicWrite(t *testing.T) {
	mock := newMockWriter()
	config := &LockFreeWriterConfig{
		BufferSize:    1024,
		FlushInterval: time.Millisecond * 100,
		BatchSize:     256,
		MaxRetries:    3,
	}

	writer := NewLockFreeBufferedWriter(mock, config)
	writer.Start()
	defer writer.Close()

	// 写入数据
	data := []byte("Hello, World!")
	n, err := writer.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	// 等待刷新
	time.Sleep(time.Millisecond * 200)

	// 验证数据已写入到底层
	mockData := mock.Data()
	if !bytes.Contains(mockData, data) {
		t.Errorf("Expected data %q in mock writer, got %q", data, mockData)
	}
}

// TestLockFreeBufferedWriter_ConcurrentWrite 测试并发写入
func TestLockFreeBufferedWriter_ConcurrentWrite(t *testing.T) {
	mock := newMockWriter()
	config := &LockFreeWriterConfig{
		BufferSize:    1024 * 1024, // 1MB
		FlushInterval: time.Second,
		BatchSize:     4096, // 4KB
		MaxRetries:    3,
	}

	writer := NewLockFreeBufferedWriter(mock, config)
	writer.Start()
	defer writer.Close()

	const numWriters = 10
	const writesPerWriter = 100

	var wg sync.WaitGroup

	// 启动多个写入协程
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()

			for j := 0; j < writesPerWriter; j++ {
				data := fmt.Sprintf("Writer %d, Message %d\n", writerID, j)
				_, err := writer.Write([]byte(data))
				if err != nil {
					t.Errorf("Write failed: %v", err)
				}
			}
		}(i)
	}

	// 等待所有写入完成
	wg.Wait()

	// 等待刷新完成
	time.Sleep(time.Second * 2)

	// 验证数据
	mockData := mock.Data()
	expectedMessages := numWriters * writesPerWriter

	// 计算实际写入的消息数
	actualMessages := 0
	for i := 0; i < numWriters; i++ {
		for j := 0; j < writesPerWriter; j++ {
			expectedMsg := fmt.Sprintf("Writer %d, Message %d\n", i, j)
			if bytes.Contains(mockData, []byte(expectedMsg)) {
				actualMessages++
			}
		}
	}

	if actualMessages < expectedMessages*9/10 { // 允许10%的误差
		t.Errorf("Expected at least %d messages, got %d", expectedMessages*9/10, actualMessages)
	}

	t.Logf("Expected messages: %d, Actual messages: %d", expectedMessages, actualMessages)
}

// TestLockFreeBufferedWriter_BufferOverflow 测试缓冲区溢出
func TestLockFreeBufferedWriter_BufferOverflow(t *testing.T) {
	mock := newMockWriter()
	config := &LockFreeWriterConfig{
		BufferSize:    256,              // 小缓冲区
		FlushInterval: time.Second * 10, // 长刷新间隔
		BatchSize:     64,
		MaxRetries:    3,
	}

	writer := NewLockFreeBufferedWriter(mock, config)
	writer.Start()
	defer writer.Close()

	// 写入超过缓冲区大小的数据
	totalWritten := 0
	for i := 0; i < 10; i++ {
		data := make([]byte, 100) // 每次写入100字节
		for j := range data {
			data[j] = byte(i)
		}

		n, err := writer.Write(data)
		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
		totalWritten += n
	}

	// 等待刷新
	time.Sleep(time.Millisecond * 500)

	// 验证至少有一些数据被写入
	mockData := mock.Data()
	if len(mockData) == 0 {
		t.Error("No data was written to mock writer")
	}

	t.Logf("Total written: %d, Mock data length: %d", totalWritten, len(mockData))
}

// TestLockFreeBufferedWriter_Flush 测试手动刷新
func TestLockFreeBufferedWriter_Flush(t *testing.T) {
	mock := newMockWriter()
	config := &LockFreeWriterConfig{
		BufferSize:    1024,
		FlushInterval: time.Second * 10, // 长刷新间隔
		BatchSize:     256,
		MaxRetries:    3,
	}

	writer := NewLockFreeBufferedWriter(mock, config)
	writer.Start()
	defer writer.Close()

	// 写入数据
	data := []byte("Test flush")
	_, err := writer.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// 手动刷新
	err = writer.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	// 验证数据已写入
	mockData := mock.Data()
	if !bytes.Contains(mockData, data) {
		t.Errorf("Expected data %q in mock writer after flush, got %q", data, mockData)
	}
}

// TestLockFreeBufferedWriter_Stats 测试统计信息
func TestLockFreeBufferedWriter_Stats(t *testing.T) {
	mock := newMockWriter()
	config := &LockFreeWriterConfig{
		BufferSize:    1024,
		FlushInterval: time.Second,
		BatchSize:     256,
		MaxRetries:    3,
	}

	writer := NewLockFreeBufferedWriter(mock, config)
	writer.Start()
	defer writer.Close()

	// 写入一些数据
	data := make([]byte, 100)
	for i := 0; i < 5; i++ {
		writer.Write(data)
	}

	// 获取统计信息
	stats := writer.Stats()

	if stats.BufferSize != 1024 {
		t.Errorf("Expected buffer size 1024, got %d", stats.BufferSize)
	}

	if stats.BufferUsed == 0 {
		t.Error("Expected buffer used > 0")
	}

	if stats.BufferAvailable < 0 {
		t.Error("Expected buffer available >= 0")
	}

	if stats.IsClosed {
		t.Error("Expected writer to be open")
	}

	t.Logf("Stats: Size=%d, Used=%d, Available=%d",
		stats.BufferSize, stats.BufferUsed, stats.BufferAvailable)
}

// TestAdaptiveBuffer 测试自适应缓冲区
func TestAdaptiveBuffer(t *testing.T) {
	mock := newMockWriter()
	config := &LockFreeWriterConfig{
		BufferSize:    1024,
		FlushInterval: time.Second,
		BatchSize:     256,
		MaxRetries:    3,
	}

	adaptive := NewAdaptiveBuffer(mock, config)
	adaptive.Start()
	defer adaptive.Close()

	// 模拟高频写入
	for i := 0; i < 100; i++ {
		data := fmt.Sprintf("High frequency write %d\n", i)
		adaptive.Write([]byte(data))
		time.Sleep(time.Millisecond) // 高频但不是极高频
	}

	// 等待自适应调整
	time.Sleep(time.Second * 11)

	// 获取写入间隔
	interval := adaptive.GetWriteInterval()
	if interval > time.Millisecond*50 {
		t.Errorf("Expected write interval < 50ms for high frequency writes, got %v", interval)
	}

	// 获取写入次数
	count := adaptive.GetWriteCount()
	if count != 100 {
		t.Errorf("Expected write count 100, got %d", count)
	}

	t.Logf("Write interval: %v, Write count: %d", interval, count)
}

// TestLockFreeBufferedWriter_Close 测试关闭功能
func TestLockFreeBufferedWriter_Close(t *testing.T) {
	mock := newMockWriter()
	config := &LockFreeWriterConfig{
		BufferSize:    1024,
		FlushInterval: time.Second,
		BatchSize:     256,
		MaxRetries:    3,
	}

	writer := NewLockFreeBufferedWriter(mock, config)
	writer.Start()

	// 写入一些数据
	data := []byte("Before close")
	writer.Write(data)

	// 关闭写入器
	err := writer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// 验证写入器已关闭
	stats := writer.Stats()
	if !stats.IsClosed {
		t.Error("Expected writer to be closed")
	}

	// 尝试写入应该失败
	_, err = writer.Write([]byte("After close"))
	if err == nil {
		t.Error("Expected write to fail after close")
	}

	// 验证关闭前的数据被刷新
	time.Sleep(time.Millisecond * 100)
	mockData := mock.Data()
	if !bytes.Contains(mockData, data) {
		t.Errorf("Expected data %q in mock writer after close, got %q", data, mockData)
	}
}

// BenchmarkLockFreeWriter 性能测试：无锁写入器
func BenchmarkLockFreeWriter(b *testing.B) {
	mock := newMockWriter()
	config := &LockFreeWriterConfig{
		BufferSize:    1024 * 1024, // 1MB
		FlushInterval: time.Second,
		BatchSize:     4096, // 4KB
		MaxRetries:    3,
	}

	writer := NewLockFreeBufferedWriter(mock, config)
	writer.Start()
	defer writer.Close()

	data := make([]byte, 256)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			writer.Write(data)
		}
	})
}

// BenchmarkAdaptiveWriter 性能测试：自适应写入器
func BenchmarkAdaptiveWriter(b *testing.B) {
	mock := newMockWriter()
	config := &LockFreeWriterConfig{
		BufferSize:    1024 * 1024, // 1MB
		FlushInterval: time.Second,
		BatchSize:     4096, // 4KB
		MaxRetries:    3,
	}

	adaptive := NewAdaptiveBuffer(mock, config)
	adaptive.Start()
	defer adaptive.Close()

	data := make([]byte, 256)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			adaptive.Write(data)
		}
	})
}

// BenchmarkDirectWrite 性能测试：直接写入
func BenchmarkDirectWrite(b *testing.B) {
	mock := newMockWriter()
	defer mock.Close()

	data := make([]byte, 256)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mock.Write(data)
		}
	})
}

// TestLockFreeBufferedWriter_RealFile 测试真实文件写入
func TestLockFreeBufferedWriter_RealFile(t *testing.T) {
	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "lockfree_test_*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	config := &LockFreeWriterConfig{
		BufferSize:    1024,
		FlushInterval: time.Millisecond * 100,
		BatchSize:     256,
		MaxRetries:    3,
	}

	writer := NewLockFreeBufferedWriter(tmpFile, config)
	writer.Start()
	defer writer.Close()

	// 写入数据
	for i := 0; i < 100; i++ {
		data := fmt.Sprintf("Log message %d\n", i)
		_, err := writer.Write([]byte(data))
		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
	}

	// 等待刷新
	time.Sleep(time.Millisecond * 200)

	// 验证文件内容
	tmpFile.Seek(0, 0)
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	// 检查是否包含预期的消息
	for i := 0; i < 100; i++ {
		expectedMsg := fmt.Sprintf("Log message %d\n", i)
		if !bytes.Contains(content, []byte(expectedMsg)) {
			t.Errorf("Expected message %q not found in file", expectedMsg)
			break
		}
	}

	t.Logf("File size: %d bytes", len(content))
}
