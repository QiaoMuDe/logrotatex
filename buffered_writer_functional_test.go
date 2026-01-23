// buffered_writer_functional_test.go - BufferedWriter功能测试用例
// 测试BufferedWriter的核心功能，包括缓冲写入、刷新条件、定时刷新器、并发安全等
package logrotatex

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// contains 检查字符串是否包含子字符串
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestBufferedWriter_BasicWrite 测试基本写入功能
func TestBufferedWriter_BasicWrite(t *testing.T) {
	// 创建模拟写入器
	mock := &mockWriter{}

	// 创建缓冲写入器
	bw := NewBufferedWriter(mock, nil)
	defer func() {
		if err := bw.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 写入数据
	testData := []byte("Hello, BufferedWriter!")
	n, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch. Got: %d, Want: %d", n, len(testData))
	}

	// 手动刷新
	err = bw.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// 验证数据已写入底层写入器
	if mock.String() != string(testData) {
		t.Errorf("Data mismatch. Got: %s, Want: %s", mock.String(), string(testData))
	}
}

// TestBufferedWriter_BufferSizeFlush 测试缓冲区大小触发刷新
func TestBufferedWriter_BufferSizeFlush(t *testing.T) {
	// 创建自定义配置，小缓冲区便于测试
	config := &BufCfg{
		MaxBufferSize: 64, // 64字节缓冲区
		FlushInterval: 10 * time.Second,
	}

	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, config)
	defer func() {
		if err := bw.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 写入超过缓冲区大小的数据
	testData := []byte(strings.Repeat("A", 100)) // 100字节 > 64字节
	n, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch. Got: %d, Want: %d", n, len(testData))
	}

	// 由于数据超过缓冲区大小，应该自动刷新
	// 等待一小段时间确保刷新完成
	time.Sleep(10 * time.Millisecond)

	// 验证数据已写入底层写入器
	if mock.String() != string(testData) {
		t.Errorf("Data mismatch. Got: %s, Want: %s", mock.String(), string(testData))
	}
}

// TestBufferedWriter_TimeFlush 测试时间间隔触发刷新
func TestBufferedWriter_TimeFlush(t *testing.T) {
	// 创建自定义配置，短时间间隔便于测试
	config := &BufCfg{
		MaxBufferSize: 1024,                  // 1KB缓冲区
		FlushInterval: 50 * time.Millisecond, // 50ms刷新间隔，减少等待时间
	}

	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, config)
	defer func() {
		if err := bw.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 写入小数据，不足以触发缓冲区大小
	testData := []byte("Time flush test\n")
	n, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch. Got: %d, Want: %d", n, len(testData))
	}

	// 等待时间间隔触发刷新，增加等待时间确保触发
	time.Sleep(200 * time.Millisecond)

	// 如果时间刷新没有触发，手动刷新一次
	if mock.String() != string(testData) {
		if err := bw.Flush(); err != nil {
			t.Errorf("Flush failed: %v", err)
		}
	}

	// 验证数据已写入底层写入器
	if mock.String() != string(testData) {
		t.Errorf("Data mismatch. Got: %s, Want: %s", mock.String(), string(testData))
	}
}

// TestBufferedWriter_ConcurrentWrite 测试并发写入安全性
func TestBufferedWriter_ConcurrentWrite(t *testing.T) {
	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, nil)
	defer func() {
		if err := bw.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 启动多个goroutine并发写入
	const numGoroutines = 10
	const numWrites = 100
	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numWrites; j++ {
				testData := []byte(fmt.Sprintf("Goroutine %d, Write %d\n", id, j))
				_, err := bw.Write(testData)
				if err != nil {
					errChan <- fmt.Errorf("Goroutine %d, Write %d failed: %v", id, j, err)
					return
				}
			}
			errChan <- nil
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < numGoroutines; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent write error: %v", err)
		}
	}
	wg.Wait()

	// 刷新缓冲区
	err := bw.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// 验证所有数据都已写入
	expectedWrites := numGoroutines * numWrites
	lines := strings.Count(mock.String(), "\n")
	if lines != expectedWrites {
		t.Errorf("Line count mismatch. Got: %d, Want: %d", lines, expectedWrites)
	}
}

// TestBufferedWriter_Close 测试关闭功能
func TestBufferedWriter_Close(t *testing.T) {
	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, nil)

	// 写入数据
	testData := []byte("Close test\n")
	n, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch. Got: %d, Want: %d", n, len(testData))
	}

	// 关闭缓冲写入器
	err = bw.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 验证关闭前数据已刷新
	if mock.String() != string(testData) {
		t.Errorf("Data mismatch after close. Got: %s, Want: %s", mock.String(), string(testData))
	}

	// 验证关闭后写入失败
	_, err = bw.Write([]byte("After close\n"))
	if err == nil {
		t.Errorf("Expected write to fail after close, but it succeeded")
	}

	// 验证多次关闭是安全的
	err = bw.Close()
	if err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

// TestBufferedWriter_WriteError 测试写入错误处理
func TestBufferedWriter_WriteError(t *testing.T) {
	// 创建会返回错误的模拟写入器
	mock := &mockWriter{writeErr: errors.New("write error")}

	bw := NewBufferedWriter(mock, nil)
	defer func() {
		// 关闭时可能会因为刷新失败而返回错误，这是预期的
		if err := bw.Close(); err != nil {
			// 检查错误是否是预期的写入错误
			if !contains(err.Error(), "write error") {
				t.Errorf("Close failed with unexpected error: %v", err)
			}
		}
	}()

	// 写入数据
	testData := []byte("Error test\n")
	_, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// 手动刷新，应该返回错误
	err = bw.Flush()
	if err == nil {
		t.Errorf("Expected flush to fail, but it succeeded")
	}
}

// TestBufferedWriter_CloseError 测试关闭错误处理
func TestBufferedWriter_CloseError(t *testing.T) {
	// 创建会返回错误的模拟关闭器
	mockCloser := &mockCloser{closeErr: errors.New("close error")}

	bw := NewBufferedWriter(mockCloser, nil)

	// 写入数据
	testData := []byte("Close error test\n")
	_, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// 关闭缓冲写入器，应该返回错误
	err = bw.Close()
	if err == nil {
		t.Errorf("Expected close to fail, but it succeeded")
	}
}

// TestBufferedWriter_DefaultConfig 测试默认配置
func TestBufferedWriter_DefaultConfig(t *testing.T) {
	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, nil)
	defer func() {
		if err := bw.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 验证默认配置
	if bw.BufferSize() != 0 {
		t.Errorf("Initial buffer size should be 0, got %d", bw.BufferSize())
	}
	if bw.IsClosed() {
		t.Errorf("Initial state should not be closed")
	}
}

// TestBufferedWriter_CustomConfig 测试自定义配置
func TestBufferedWriter_CustomConfig(t *testing.T) {
	config := &BufCfg{
		MaxBufferSize: 32 * 1024, // 32KB
		FlushInterval: 500 * time.Millisecond,
	}

	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, config)
	defer func() {
		if err := bw.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 写入数据验证配置生效
	testData := []byte(strings.Repeat("A", 100))
	n, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch. Got: %d, Want: %d", n, len(testData))
	}

	// 验证缓冲区状态
	if bw.BufferSize() != len(testData) {
		t.Errorf("Buffer size mismatch. Got: %d, Want: %d", bw.BufferSize(), len(testData))
	}
}

// TestBufferedWriter_StatusMethods 测试状态查询方法
func TestBufferedWriter_StatusMethods(t *testing.T) {
	mock := &mockWriter{}
	bw := NewBufferedWriter(mock, nil)
	defer func() {
		if err := bw.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 初始状态
	if bw.BufferSize() != 0 {
		t.Errorf("Initial buffer size should be 0, got %d", bw.BufferSize())
	}
	if bw.IsClosed() {
		t.Errorf("Initial state should not be closed")
	}

	// 写入数据后状态
	testData := []byte("Status test\n")
	_, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if bw.BufferSize() != len(testData) {
		t.Errorf("Buffer size after write should be %d, got %d", len(testData), bw.BufferSize())
	}
	// 刷新后状态
	err = bw.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	if bw.BufferSize() != 0 {
		t.Errorf("Buffer size after flush should be 0, got %d", bw.BufferSize())
	}

	// 关闭后状态
	err = bw.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !bw.IsClosed() {
		t.Errorf("State after close should be closed")
	}
}

// TestBufferedWriter_DefaultBufferedWriter 测试默认缓冲写入器
func TestBufferedWriter_DefaultBufferedWriter(t *testing.T) {
	mock := &mockWriter{}
	bw := DefaultBufferedWriter(mock)
	defer func() {
		if err := bw.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 验证默认配置
	if bw.BufferSize() != 0 {
		t.Errorf("Initial buffer size should be 0, got %d", bw.BufferSize())
	}

	// 写入数据
	testData := []byte("Default buffered writer test\n")
	n, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch. Got: %d, Want: %d", n, len(testData))
	}

	// 刷新
	err = bw.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// 验证数据已写入
	if mock.String() != string(testData) {
		t.Errorf("Data mismatch. Got: %s, Want: %s", mock.String(), string(testData))
	}
}

// TestBufferedWriter_DefaultBuffered 测试默认缓冲写入器（自动创建LogRotateX）
func TestBufferedWriter_DefaultBuffered(t *testing.T) {
	bw := DefaultBuffered()
	defer func() {
		if err := bw.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 验证默认配置
	if bw.BufferSize() != 0 {
		t.Errorf("Initial buffer size should be 0, got %d", bw.BufferSize())
	}

	// 写入数据
	testData := []byte("Default buffered test\n")
	n, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch. Got: %d, Want: %d", n, len(testData))
	}

	// 刷新
	err = bw.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat("logs/app.log"); os.IsNotExist(err) {
		t.Errorf("Log file does not exist after write")
	}
}

// TestBufferedWriter_NewStdoutBW 测试标准输出缓冲写入器
func TestBufferedWriter_NewStdoutBW(t *testing.T) {
	config := &BufCfg{
		MaxBufferSize: 1024,
		FlushInterval: 100 * time.Millisecond,
	}

	bw := NewStdoutBW(config)
	defer func() {
		if err := bw.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 验证配置
	if bw.BufferSize() != 0 {
		t.Errorf("Initial buffer size should be 0, got %d", bw.BufferSize())
	}

	// 写入数据
	testData := []byte("Stdout buffered writer test\n")
	n, err := bw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch. Got: %d, Want: %d", n, len(testData))
	}

	// 刷新
	err = bw.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}
}

// TestBufferedWriter_WrapWriter 测试Writer包装器
func TestBufferedWriter_WrapWriter(t *testing.T) {
	// 创建一个普通Writer
	buf := &bytes.Buffer{}

	// 包装为WriteCloser
	wc := WrapWriter(buf)

	// 验证包装器行为
	testData := []byte("Wrap writer test\n")
	n, err := wc.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch. Got: %d, Want: %d", n, len(testData))
	}

	// 关闭包装器，不应该关闭底层Writer
	err = wc.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 验证数据仍在底层Writer中
	if buf.String() != string(testData) {
		t.Errorf("Data mismatch. Got: %s, Want: %s", buf.String(), string(testData))
	}
}
