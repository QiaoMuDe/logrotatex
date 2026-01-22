// ring_buffer_test.go - 环形缓冲区测试用例
package lockfree

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

// TestNewRingBuffer 测试环形缓冲区创建
func TestNewRingBuffer(t *testing.T) {
	// 测试正常大小
	rb := NewRingBuffer(1024)
	if rb.Size() != 1024 {
		t.Errorf("Expected size 1024, got %d", rb.Size())
	}

	// 测试非2的幂次方大小，应该向上调整
	rb = NewRingBuffer(1000)
	expectedSize := 1024 // 1000向上调整到最近的2的幂次方
	if rb.Size() != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, rb.Size())
	}

	// 测试最小大小
	rb = NewRingBuffer(10)
	if rb.Size() != 64 {
		t.Errorf("Expected size 64, got %d", rb.Size())
	}
}

// TestBasicReadWrite 测试基本的读写操作
func TestBasicReadWrite(t *testing.T) {
	rb := NewRingBuffer(1024)

	// 写入数据
	data := []byte("Hello, World!")
	n, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	// 读取数据
	readBuf := make([]byte, len(data))
	n, err = rb.Read(readBuf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to read %d bytes, read %d", len(data), n)
	}

	// 验证数据一致性
	if !bytes.Equal(data, readBuf) {
		t.Errorf("Data mismatch: expected %v, got %v", data, readBuf)
	}

	// 缓冲区应该为空
	if !rb.IsEmpty() {
		t.Error("Buffer should be empty after reading all data")
	}
}

// TestWrapAround 测试环绕写入和读取
func TestWrapAround(t *testing.T) {
	rb := NewRingBuffer(64)

	// 填充缓冲区到接近满
	data := make([]byte, 50)
	for i := range data {
		data[i] = byte(i)
	}

	n, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	// 读取一部分数据
	readBuf := make([]byte, 30)
	n, err = rb.Read(readBuf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 30 {
		t.Errorf("Expected to read 30 bytes, read %d", n)
	}

	// 验证读取的数据
	for i := 0; i < 30; i++ {
		if readBuf[i] != byte(i) {
			t.Errorf("Data mismatch at position %d: expected %d, got %d", i, i, readBuf[i])
		}
	}

	// 再写入数据，这次应该环绕
	moreData := []byte{100, 101, 102, 103, 104}
	n, err = rb.Write(moreData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(moreData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(moreData), n)
	}

	// 读取剩余数据
	remainingBuf := make([]byte, 25)
	n, err = rb.Read(remainingBuf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 25 {
		t.Errorf("Expected to read 25 bytes, read %d", n)
	}

	// 验证剩余数据
	expectedRemaining := []byte{30, 31, 32, 33, 34, 35, 36, 37, 38, 39,
		40, 41, 42, 43, 44, 45, 46, 47, 48, 49,
		100, 101, 102, 103, 104}

	if !bytes.Equal(remainingBuf[:n], expectedRemaining) {
		t.Errorf("Remaining data mismatch: expected %v, got %v", expectedRemaining, remainingBuf[:n])
	}
}

// TestConcurrentReadWrite 测试并发读写
func TestConcurrentReadWrite(t *testing.T) {
	rb := NewRingBuffer(1024 * 1024) // 1MB缓冲区

	const numWriters = 10
	const numReaders = 5
	const writesPerWriter = 1000

	var wg sync.WaitGroup
	writeDone := make(chan struct{})

	// 启动写入协程
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()

			for j := 0; j < writesPerWriter; j++ {
				data := fmt.Sprintf("Writer %d, Message %d\n", writerID, j)

				// 尝试写入，如果缓冲区满则重试
				for {
					err := rb.TryWrite([]byte(data))
					if err == nil {
						break
					}
					time.Sleep(time.Microsecond * 10)
				}
			}
		}(i)
	}

	// 启动读取协程
	readCount := make([]int, numReaders)
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			buf := make([]byte, 1024)
			for {
				n, err := rb.Read(buf)
				if err != nil || n == 0 {
					select {
					case <-writeDone:
						return
					case <-time.After(time.Millisecond * 10):
						continue
					}
				}

				readCount[readerID]++
			}
		}(i)
	}

	// 等待写入完成
	wg.Wait()
	close(writeDone)

	// 等待读取完成（给一些时间处理剩余数据）
	time.Sleep(time.Millisecond * 100)

	// 验证读取的总数
	totalReads := 0
	for _, count := range readCount {
		totalReads += count
	}

	expectedWrites := numWriters * writesPerWriter
	if totalReads < expectedWrites/2 { // 允许一些消息还在缓冲区中
		t.Errorf("Expected at least %d reads, got %d", expectedWrites/2, totalReads)
	}

	t.Logf("Total reads: %d, Expected writes: %d", totalReads, expectedWrites)
}

// TestBufferFull 测试缓冲区满的情况
func TestBufferFull(t *testing.T) {
	rb := NewRingBuffer(64)

	// 填满缓冲区
	data := make([]byte, 64)
	n, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 64 {
		t.Errorf("Expected to write 64 bytes, wrote %d", n)
	}

	// 缓冲区应该已满
	if !rb.IsFull() {
		t.Error("Buffer should be full")
	}

	// 尝试写入更多数据，应该失败
	moreData := []byte{1, 2, 3}
	_, err = rb.Write(moreData)
	if err == nil {
		t.Error("Expected write to fail when buffer is full")
	}

	// 读取一些数据，释放空间
	readBuf := make([]byte, 32)
	n, err = rb.Read(readBuf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 32 {
		t.Errorf("Expected to read 32 bytes, read %d", n)
	}

	// 现在应该可以写入更多数据
	n, err = rb.Write(moreData)
	if err != nil {
		t.Errorf("Write should succeed after reading some data: %v", err)
	}
	if n != len(moreData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(moreData), n)
	}
}

// TestBatchOperations 测试批量操作
func TestBatchOperations(t *testing.T) {
	rb := NewRingBuffer(1024)

	// 准备多个数据块
	batches := [][]byte{
		[]byte("First batch"),
		[]byte("Second batch"),
		[]byte("Third batch"),
	}

	// 批量写入
	totalWritten, err := rb.WriteBatch(batches)
	if err != nil {
		t.Fatalf("WriteBatch failed: %v", err)
	}

	expectedTotal := 0
	for _, batch := range batches {
		expectedTotal += len(batch)
	}

	if totalWritten != expectedTotal {
		t.Errorf("Expected to write %d bytes total, wrote %d", expectedTotal, totalWritten)
	}

	// 准备读取缓冲区
	buffers := [][]byte{
		make([]byte, 12),
		make([]byte, 13),
		make([]byte, 12),
	}

	// 批量读取
	totalRead, err := rb.ReadBatch(buffers)
	if err != nil {
		t.Fatalf("ReadBatch failed: %v", err)
	}

	if totalRead != expectedTotal {
		t.Errorf("Expected to read %d bytes total, read %d", expectedTotal, totalRead)
	}

	// 验证数据
	expectedData := string(batches[0]) + string(batches[1]) + string(batches[2])
	actualData := string(buffers[0]) + string(buffers[1]) + string(buffers[2])

	if expectedData != actualData {
		t.Errorf("Data mismatch: expected %q, got %q", expectedData, actualData)
	}
}

// TestPeek 测试查看操作
func TestPeek(t *testing.T) {
	rb := NewRingBuffer(1024)

	// 写入数据
	data := []byte("Hello, World!")
	_, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// 查看数据
	peekBuf := make([]byte, len(data))
	n, err := rb.Peek(peekBuf)
	if err != nil {
		t.Fatalf("Peek failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to peek %d bytes, peeked %d", len(data), n)
	}

	// 验证查看的数据
	if !bytes.Equal(data, peekBuf) {
		t.Errorf("Peeked data mismatch: expected %v, got %v", data, peekBuf)
	}

	// 缓冲区中的数据应该还在
	if rb.IsEmpty() {
		t.Error("Buffer should not be empty after peek")
	}

	// 正常读取应该仍然可以工作
	readBuf := make([]byte, len(data))
	n, err = rb.Read(readBuf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to read %d bytes, read %d", len(data), n)
	}

	if !bytes.Equal(data, readBuf) {
		t.Errorf("Read data mismatch: expected %v, got %v", data, readBuf)
	}
}

// BenchmarkRingBufferWrite 性能测试：写入
func BenchmarkRingBufferWrite(b *testing.B) {
	rb := NewRingBuffer(1024 * 1024) // 1MB缓冲区
	data := make([]byte, 256)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rb.TryWrite(data)
		}
	})
}

// BenchmarkRingBufferRead 性能测试：读取
func BenchmarkRingBufferRead(b *testing.B) {
	rb := NewRingBuffer(1024 * 1024) // 1MB缓冲区
	data := make([]byte, 256)

	// 预填充缓冲区
	for i := 0; i < 1000; i++ {
		rb.TryWrite(data)
	}

	readBuf := make([]byte, 256)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rb.TryRead(readBuf)
		}
	})
}

// BenchmarkRingBufferConcurrent 性能测试：并发读写
func BenchmarkRingBufferConcurrent(b *testing.B) {
	rb := NewRingBuffer(1024 * 1024) // 1MB缓冲区
	data := make([]byte, 256)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		readBuf := make([]byte, 256)

		for pb.Next() {
			if rng.Float32() < 0.5 {
				// 50%概率写入
				rb.TryWrite(data)
			} else {
				// 50%概率读取
				rb.TryRead(readBuf)
			}
		}
	})
}
