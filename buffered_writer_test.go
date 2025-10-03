/* buffered_writer_test.go 包含对带缓冲批量写入器的测试，用于验证刷新条件、并发安全与集成行为。*/
package logrotatex

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockWriter 模拟写入器，用于测试
type mockWriter struct {
	data       bytes.Buffer
	writeErr   error
	writeCalls int
	mu         sync.Mutex
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalls++
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return m.data.Write(p)
}

func (m *mockWriter) Close() error {
	return nil
}

func (m *mockWriter) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data.String()
}

func (m *mockWriter) WriteCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeCalls
}

func (m *mockWriter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data.Reset()
	m.writeCalls = 0
	m.writeErr = nil
}

// mockCloser 模拟关闭器，用于测试关闭错误
type mockCloser struct {
	closeErr error
	closed   bool
	mu       sync.Mutex
}

func (m *mockCloser) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (m *mockCloser) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return m.closeErr
}

func (m *mockCloser) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// writerCloser 组合 mockWriter 与 mockCloser，适配为 io.WriteCloser
type writerCloser struct {
	w *mockWriter
	c *mockCloser
}

func (wc *writerCloser) Write(p []byte) (int, error) {
	return wc.w.Write(p)
}

func (wc *writerCloser) Close() error {
	return wc.c.Close()
}

// TestDefBufCfg 测试默认配置
func TestDefBufCfg(t *testing.T) {
	cfg := DefBufCfg()

	if cfg.MaxBufferSize != 64*1024 {
		t.Errorf("Expected MaxBufferSize 64KB, got %d", cfg.MaxBufferSize)
	}
	if cfg.MaxWriteCount != 500 {
		t.Errorf("Expected MaxWriteCount 500, got %d", cfg.MaxWriteCount)
	}
	if cfg.FlushInterval != 1*time.Second {
		t.Errorf("Expected FlushInterval 1s, got %v", cfg.FlushInterval)
	}
}

// TestNewBufferedWriter 测试构造函数
func TestNewBufferedWriter(t *testing.T) {
	t.Run("正常创建", func(t *testing.T) {
		writer := &mockWriter{}
		closer := &mockCloser{}
		wc := &writerCloser{w: writer, c: closer}

		bw := NewBufferedWriter(wc, nil)
		defer func() { _ = bw.Close() }()

		if bw.wc != wc {
			t.Error("WriteCloser not set correctly")
		}
		if bw.maxBufferSize != 64*1024 {
			t.Error("Default buffer size not set")
		}
	})

	t.Run("自定义配置", func(t *testing.T) {
		writer := &mockWriter{}
		closer := &mockCloser{}
		config := &BufCfg{
			MaxBufferSize: 1024,
			MaxWriteCount: 10,
			FlushInterval: 100 * time.Millisecond,
		}

		bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, config)
		defer func() { _ = bw.Close() }()

		if bw.maxBufferSize != 1024 {
			t.Errorf("Expected buffer size 1024, got %d", bw.maxBufferSize)
		}
		if bw.maxWriteCount != 10 {
			t.Errorf("Expected log count 10, got %d", bw.maxWriteCount)
		}
		if bw.flushInterval != 100*time.Millisecond {
			t.Errorf("Expected flush interval 100ms, got %v", bw.flushInterval)
		}
	})

	t.Run("WriteCloser为nil应该panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when WriteCloser is nil")
			}
		}()
		NewBufferedWriter(nil, nil)
	})
}

// TestWrite 测试写入功能
func TestWrite(t *testing.T) {
	t.Run("基本写入", func(t *testing.T) {
		writer := &mockWriter{}
		closer := &mockCloser{}
		config := &BufCfg{
			MaxBufferSize: 1024,
			MaxWriteCount: 5,
			FlushInterval: 1 * time.Second,
		}

		bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, config)
		defer func() { _ = bw.Close() }()

		data := []byte("test log message\n")
		n, err := bw.Write(data)

		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != len(data) {
			t.Errorf("Expected %d bytes written, got %d", len(data), n)
		}
		if bw.WriteCount() != 1 {
			t.Errorf("Expected log count 1, got %d", bw.WriteCount())
		}
		if bw.BufferSize() != len(data) {
			t.Errorf("Expected buffer size %d, got %d", len(data), bw.BufferSize())
		}
	})

	t.Run("写入已关闭的writer", func(t *testing.T) {
		writer := &mockWriter{}
		closer := &mockCloser{}

		bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, nil)
		_ = bw.Close()

		_, err := bw.Write([]byte("test"))
		if !errors.Is(err, io.ErrClosedPipe) && err.Error() != "logrotatex: write on closed" {
			t.Fatalf("Expected ErrClosedPipe or 'logrotatex: write on closed', got %v", err)
		}
	})
}

// TestFlushConditions 测试三重刷新条件
func TestFlushConditions(t *testing.T) {
	t.Run("大小触发刷新", func(t *testing.T) {
		writer := &mockWriter{}
		closer := &mockCloser{}
		config := &BufCfg{
			MaxBufferSize: 10, // 很小的缓冲区
			MaxWriteCount: 100,
			FlushInterval: 1 * time.Hour, // 很长的间隔
		}

		bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, config)
		defer func() { _ = bw.Close() }()

		// 写入超过缓冲区大小的数据
		data := []byte("this is a long message")
		_, _ = bw.Write(data)

		// 应该触发刷新
		if writer.WriteCalls() != 1 {
			t.Errorf("Expected 1 write call, got %d", writer.WriteCalls())
		}
		if bw.BufferSize() != 0 {
			t.Errorf("Buffer should be empty after flush, got %d", bw.BufferSize())
		}
	})

	t.Run("数量触发刷新", func(t *testing.T) {
		writer := &mockWriter{}
		closer := &mockCloser{}
		config := &BufCfg{
			MaxBufferSize: 1024,
			MaxWriteCount: 3, // 很小的计数
			FlushInterval: 1 * time.Hour,
		}

		bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, config)
		defer func() { _ = bw.Close() }()

		// 写入3条日志
		for i := 0; i < 3; i++ {
			_, _ = bw.Write([]byte("log\n"))
		}

		// 应该触发刷新
		if writer.WriteCalls() != 1 {
			t.Errorf("Expected 1 write call, got %d", writer.WriteCalls())
		}
		if bw.WriteCount() != 0 {
			t.Errorf("Log count should be 0 after flush, got %d", bw.WriteCount())
		}
	})

	t.Run("时间触发刷新", func(t *testing.T) {
		writer := &mockWriter{}
		closer := &mockCloser{}
		config := &BufCfg{
			MaxBufferSize: 1024,
			MaxWriteCount: 100,
			FlushInterval: 50 * time.Millisecond, // 很短的间隔
		}

		bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, config)
		defer func() { _ = bw.Close() }()

		// 写入一条日志
		_, _ = bw.Write([]byte("log\n"))

		// 等待超过刷新间隔
		time.Sleep(60 * time.Millisecond)

		// 再写入一条，应该触发时间刷新
		_, _ = bw.Write([]byte("log2\n"))

		if writer.WriteCalls() != 1 {
			t.Errorf("Expected 1 write call, got %d", writer.WriteCalls())
		}
	})
}

// TestFlush 测试手动刷新
func TestFlush(t *testing.T) {
	writer := &mockWriter{}
	closer := &mockCloser{}

	bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, nil)
	defer func() { _ = bw.Close() }()

	// 写入一些数据
	_, _ = bw.Write([]byte("test data"))

	// 手动刷新
	err := bw.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	if writer.WriteCalls() != 1 {
		t.Errorf("Expected 1 write call, got %d", writer.WriteCalls())
	}
	if bw.BufferSize() != 0 {
		t.Error("Buffer should be empty after flush")
	}
	if bw.WriteCount() != 0 {
		t.Error("Log count should be 0 after flush")
	}
}

// TestClose 测试关闭功能
func TestClose(t *testing.T) {
	t.Run("正常关闭", func(t *testing.T) {
		writer := &mockWriter{}
		closer := &mockCloser{}

		bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, nil)

		// 写入一些数据
		_, _ = bw.Write([]byte("test data"))

		err := bw.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		// 应该刷新数据并关闭底层writer
		if writer.WriteCalls() != 1 {
			t.Error("Should flush data before close")
		}
		if !closer.IsClosed() {
			t.Error("Closer should be closed")
		}
		if !bw.IsClosed() {
			t.Error("BufferedWriter should be closed")
		}
	})

	t.Run("重复关闭", func(t *testing.T) {
		writer := &mockWriter{}
		closer := &mockCloser{}

		bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, nil)

		// 第一次关闭
		err1 := bw.Close()
		if err1 != nil {
			t.Fatalf("First close failed: %v", err1)
		}

		// 第二次关闭应该无操作
		err2 := bw.Close()
		if err2 != nil {
			t.Fatalf("Second close failed: %v", err2)
		}
	})

	t.Run("关闭时底层writer出错", func(t *testing.T) {
		writer := &mockWriter{}
		closer := &mockCloser{closeErr: errors.New("close error")}

		bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, nil)

		err := bw.Close()
		if err == nil {
			t.Error("Expected close error")
		}
		if err.Error() != "close error" {
			t.Errorf("Expected 'close error', got %v", err)
		}
	})
}

// TestStatusMethods 测试状态查询方法
func TestStatusMethods(t *testing.T) {
	writer := &mockWriter{}
	closer := &mockCloser{}

	bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, nil)
	defer func() { _ = bw.Close() }()

	// 初始状态
	if bw.BufferSize() != 0 {
		t.Error("Initial buffer size should be 0")
	}
	if bw.WriteCount() != 0 {
		t.Error("Initial log count should be 0")
	}
	if bw.IsClosed() {
		t.Error("Should not be closed initially")
	}

	// 写入数据后
	data := []byte("test message")
	_, _ = bw.Write(data)

	if bw.BufferSize() != len(data) {
		t.Errorf("Expected buffer size %d, got %d", len(data), bw.BufferSize())
	}
	if bw.WriteCount() != 1 {
		t.Errorf("Expected log count 1, got %d", bw.WriteCount())
	}

	// 测试时间方法
	duration := bw.TimeSinceLastFlush()
	if duration < 0 {
		t.Error("TimeSinceLastFlush should not be negative")
	}
}

// TestConcurrency 测试并发安全
func TestConcurrency(t *testing.T) {
	writer := &mockWriter{}
	closer := &mockCloser{}
	config := &BufCfg{
		MaxBufferSize: 1024,
		MaxWriteCount: 100,
		FlushInterval: 100 * time.Millisecond,
	}

	bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, config)
	defer func() { _ = bw.Close() }()

	// 并发写入
	var wg sync.WaitGroup
	numGoroutines := 10
	numWrites := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numWrites; j++ {
				data := []byte("goroutine " + string(rune(id+'0')) + " message\n")
				_, _ = bw.Write(data)
			}
		}(i)
	}

	// 并发读取状态
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				bw.BufferSize()
				bw.WriteCount()
				bw.IsClosed()
				bw.TimeSinceLastFlush()
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// 最终刷新
	_ = bw.Flush()

	// 验证没有数据丢失
	totalExpected := numGoroutines * numWrites
	actualLines := strings.Count(writer.String(), "\n")
	if actualLines != totalExpected {
		t.Errorf("Expected %d lines, got %d", totalExpected, actualLines)
	}
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	t.Run("底层writer写入错误", func(t *testing.T) {
		writer := &mockWriter{writeErr: errors.New("write error")}
		closer := &mockCloser{}
		config := &BufCfg{
			MaxBufferSize: 10,
			MaxWriteCount: 1,
			FlushInterval: 1 * time.Hour,
		}

		bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, config)
		defer func() { _ = bw.Close() }()

		// 写入数据触发刷新，应该返回错误
		_, err := bw.Write([]byte("test message"))
		if err == nil {
			t.Error("Expected write error")
		}
		if err.Error() != "write error" {
			t.Errorf("Expected 'write error', got %v", err)
		}
	})

	t.Run("手动刷新时出错", func(t *testing.T) {
		writer := &mockWriter{}
		closer := &mockCloser{}

		bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, nil)
		defer func() { _ = bw.Close() }()

		// 写入数据
		_, _ = bw.Write([]byte("test"))

		// 设置写入错误
		writer.writeErr = errors.New("flush error")

		err := bw.Flush()
		if err == nil {
			t.Error("Expected flush error")
		}
	})
}

// TestRealLogRotateX 测试与真实LogRotateX的集成
func TestRealLogRotateX(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	logger := NewLogRotateX(logFile)
	logger.MaxSize = 1 // 1MB

	config := &BufCfg{
		MaxBufferSize: 1024,
		MaxWriteCount: 10,
		FlushInterval: 100 * time.Millisecond,
	}

	bw := NewBufferedWriter(logger, config)
	defer func() { _ = bw.Close() }()

	// 写入一些测试数据
	for i := 0; i < 20; i++ {
		message := []byte("This is test log message " + string(rune(i+'0')) + "\n")
		n, err := bw.Write(message)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != len(message) {
			t.Errorf("Expected %d bytes written, got %d", len(message), n)
		}
	}

	// 手动刷新确保数据写入
	err := bw.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// 验证文件存在且有内容
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file should exist")
	}

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	lines := strings.Count(string(content), "\n")
	if lines != 20 {
		t.Errorf("Expected 20 lines in log file, got %d", lines)
	}
}

// BenchmarkBufferedWriter 性能基准测试
func BenchmarkBufferedWriter(b *testing.B) {
	writer := &mockWriter{}
	closer := &mockCloser{}

	bw := NewBufferedWriter(&writerCloser{w: writer, c: closer}, nil)
	defer func() { _ = bw.Close() }()

	data := []byte("benchmark test message\n")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = bw.Write(data)
		}
	})
}

// BenchmarkDirectWrite 直接写入的基准测试（对比）
func BenchmarkDirectWrite(b *testing.B) {
	writer := &mockWriter{}
	data := []byte("benchmark test message\n")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = writer.Write(data)
		}
	})
}
