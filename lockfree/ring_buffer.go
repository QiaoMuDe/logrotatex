// Package lockfree 提供高性能的无锁数据结构实现
// 主要包括环形缓冲区等无锁并发组件，适用于高并发场景
package lockfree

import (
	"errors"
	"sync/atomic"
	"time"
	"unsafe"
)

// RingBuffer 环形缓冲区实现
// 使用原子操作实现无锁并发读写，适用于生产者-消费者场景
type RingBuffer struct {
	// 缓冲区数据存储
	buffer []byte

	// 读写位置索引，使用 uint64 保证原子操作
	writeHead uint64 // 写入位置头部
	readTail  uint64 // 读取位置尾部

	// 缓冲区大小，必须是2的幂次方，便于使用位运算优化
	size uint64
	mask uint64 // size - 1，用于快速取模

	// 用于对齐和防止伪共享
	padding [56]byte // 填充到64字节边界，避免CPU缓存行伪共享
}

// NewRingBuffer 创建一个新的环形缓冲区
// size 参数必须是2的幂次方，如果不是会自动向上调整到最近的2的幂次方
func NewRingBuffer(size int) *RingBuffer {
	// 确保大小是2的幂次方
	if size < 64 {
		size = 64 // 最小64字节
	}

	// 向上调整到最近的2的幂次方
	if size&(size-1) != 0 {
		size = nextPowerOfTwo(size)
	}

	return &RingBuffer{
		buffer: make([]byte, size),
		size:   uint64(size),
		mask:   uint64(size) - 1,
	}
}

// nextPowerOfTwo 计算大于等于n的最小2的幂次方
func nextPowerOfTwo(n int) int {
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	n++
	return n
}

// Size 返回缓冲区总大小
func (rb *RingBuffer) Size() int {
	return int(rb.size)
}

// Used 返回已使用的空间
func (rb *RingBuffer) Used() int {
	writeHead := atomic.LoadUint64(&rb.writeHead)
	readTail := atomic.LoadUint64(&rb.readTail)
	return int(writeHead - readTail)
}

// Available 返回可用空间
func (rb *RingBuffer) Available() int {
	return rb.Size() - rb.Used()
}

// IsEmpty 检查缓冲区是否为空
func (rb *RingBuffer) IsEmpty() bool {
	return rb.Used() == 0
}

// IsFull 检查缓冲区是否已满
func (rb *RingBuffer) IsFull() bool {
	return rb.Available() == 0
}

// Close 关闭环形缓冲区
// 设置关闭标志，防止新的写入操作
func (rb *RingBuffer) Close() error {
	// 使用原子操作设置关闭标志
	// 这里我们使用writeHead的最高位作为关闭标志
	for {
		writeHead := atomic.LoadUint64(&rb.writeHead)
		if writeHead&(1<<63) != 0 {
			return nil // 已经关闭
		}

		// 设置最高位为1，表示关闭
		newWriteHead := writeHead | (1 << 63)
		if atomic.CompareAndSwapUint64(&rb.writeHead, writeHead, newWriteHead) {
			return nil
		}
	}
}

// IsClosed 检查缓冲区是否已关闭
func (rb *RingBuffer) IsClosed() bool {
	writeHead := atomic.LoadUint64(&rb.writeHead)
	return writeHead&(1<<63) != 0
}

// Reset 重置缓冲区，清空所有数据
// 使用原子操作确保线程安全
func (rb *RingBuffer) Reset() {
	// 原子性地重置读写位置
	writeHead := atomic.LoadUint64(&rb.writeHead)
	readTail := atomic.LoadUint64(&rb.readTail)

	// 清除关闭标志（如果有的话）
	writeHead &= ^(uint64(1) << 63)

	// 使用CAS循环确保重置操作的原子性
	for !atomic.CompareAndSwapUint64(&rb.writeHead, writeHead, readTail) {
		writeHead = atomic.LoadUint64(&rb.writeHead)
		writeHead &= ^(uint64(1) << 63)
	}

	// 清空缓冲区数据（可选，但有助于安全）
	for i := range rb.buffer {
		rb.buffer[i] = 0
	}
}

// WriteString 写入字符串，避免不必要的内存分配
func (rb *RingBuffer) WriteString(s string) (int, error) {
	return rb.Write([]byte(s))
}

// ReadString 读取指定长度的字符串
func (rb *RingBuffer) ReadString(len int) (string, error) {
	buf := make([]byte, len)
	n, err := rb.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

// Peek 查看数据但不移动读取位置
// 适用于需要预览数据的场景
func (rb *RingBuffer) Peek(data []byte) (int, error) {
	dataLen := uint64(len(data))
	if dataLen == 0 {
		return 0, nil
	}

	// 原子获取当前位置
	writeHead := atomic.LoadUint64(&rb.writeHead)
	readTail := atomic.LoadUint64(&rb.readTail)

	// 计算可读取的数据量
	available := writeHead - readTail
	if available == 0 {
		return 0, nil // 缓冲区为空
	}

	// 限制读取长度
	if dataLen > available {
		dataLen = available
	}

	// 计算读取位置
	readPos := readTail & rb.mask

	// 读取数据但不更新读取位置
	if readPos+dataLen <= rb.size {
		copy(data, rb.buffer[readPos:readPos+dataLen])
	} else {
		firstPart := rb.size - readPos
		copy(data[:firstPart], rb.buffer[readPos:])
		copy(data[firstPart:], rb.buffer[:dataLen-firstPart])
	}

	return int(dataLen), nil
}

// WriteWithTimeout 带超时的写入操作
// 在指定时间内尝试写入，超时则返回错误
func (rb *RingBuffer) WriteWithTimeout(data []byte, timeout time.Duration) error {
	startTime := time.Now()
	//dataLen := len(data)

	for {
		err := rb.TryWrite(data)
		if err == nil {
			return nil // 写入成功
		}

		// 检查是否超时
		if time.Since(startTime) > timeout {
			return errors.New("ring buffer: write timeout")
		}

		// 短暂休眠后重试
		time.Sleep(time.Microsecond * 10)
	}
}

// ReadWithTimeout 带超时的读取操作
// 在指定时间内尝试读取，超时则返回错误
func (rb *RingBuffer) ReadWithTimeout(data []byte, timeout time.Duration) (int, error) {
	startTime := time.Now()

	for {
		n, err := rb.TryRead(data)
		if err == nil {
			return n, nil // 读取成功
		}

		// 检查是否超时
		if time.Since(startTime) > timeout {
			return 0, errors.New("ring buffer: read timeout")
		}

		// 短暂休眠后重试
		time.Sleep(time.Microsecond * 10)
	}
}

// WriteOrDiscard 写入数据，如果缓冲区满则丢弃旧数据
// 适用于可以容忍数据丢失的场景
func (rb *RingBuffer) WriteOrDiscard(data []byte) (int, error) {
	dataLen := uint64(len(data))
	if dataLen == 0 {
		return 0, nil
	}

	writeHead := atomic.LoadUint64(&rb.writeHead)
	readTail := atomic.LoadUint64(&rb.readTail)

	// 计算可用空间
	available := rb.size - (writeHead - readTail)

	if dataLen > available {
		// 如果空间不足，丢弃旧数据为新数据腾出空间
		discardNeeded := dataLen - available
		atomic.AddUint64(&rb.readTail, discardNeeded)
	}

	// 重新计算写入位置
	writeHead = atomic.LoadUint64(&rb.writeHead)
	writePos := writeHead & rb.mask

	// 写入数据（可能需要分两次）
	if writePos+dataLen <= rb.size {
		copy(rb.buffer[writePos:], data)
	} else {
		firstPart := rb.size - writePos
		copy(rb.buffer[writePos:], data[:firstPart])
		copy(rb.buffer[0:], data[firstPart:])
	}

	// 更新写入位置
	atomic.AddUint64(&rb.writeHead, dataLen)

	return int(dataLen), nil
}

// WriteAtomic 带内存屏障的写入操作
// 确保写入操作的可见性和顺序性
func (rb *RingBuffer) WriteAtomic(data []byte) (int, error) {
	dataLen := uint64(len(data))
	if dataLen == 0 {
		return 0, nil
	}

	// 使用原子操作获取当前写入位置
	writeHead := atomic.LoadUint64(&rb.writeHead)

	// 循环尝试写入，直到成功或失败
	for {
		readTail := atomic.LoadUint64(&rb.readTail)

		// 计算可用空间
		available := rb.size - (writeHead - readTail)

		// 检查是否有足够空间
		if dataLen > available {
			return 0, errors.New("ring buffer: insufficient space")
		}

		// 尝试原子性地更新写入位置
		newWriteHead := writeHead + dataLen
		if atomic.CompareAndSwapUint64(&rb.writeHead, writeHead, newWriteHead) {
			// 成功获取写入位置，现在可以安全地写入数据
			break
		}

		// CAS失败，重新加载写入位置并重试
		writeHead = atomic.LoadUint64(&rb.writeHead)
	}

	// 计算实际写入位置
	writePos := writeHead & rb.mask

	// 使用内存屏障确保写入操作的顺序性
	atomic.StoreUint64((*uint64)(unsafe.Pointer(&rb.buffer[writePos])), 0) // 强制内存屏障

	// 执行实际的数据写入
	if writePos+dataLen <= rb.size {
		// 一次性写入，不需要环绕
		copy(rb.buffer[writePos:], data)
	} else {
		// 需要分两次写入
		firstPart := rb.size - writePos
		copy(rb.buffer[writePos:], data[:firstPart])
		copy(rb.buffer[0:], data[firstPart:])
	}

	// 再次使用内存屏障确保数据写入完成
	atomic.StoreUint64((*uint64)(unsafe.Pointer(&rb.buffer[writePos])), 0) // 强制内存屏障

	return int(dataLen), nil
}

// ReadAtomic 带内存屏障的读取操作
// 确保读取操作的可见性和顺序性
func (rb *RingBuffer) ReadAtomic(data []byte) (int, error) {
	dataLen := uint64(cap(data))
	if dataLen == 0 {
		return 0, nil
	}

	// 使用原子操作获取当前读取位置
	readTail := atomic.LoadUint64(&rb.readTail)

	// 循环尝试读取，直到成功或失败
	for {
		writeHead := atomic.LoadUint64(&rb.writeHead)

		// 计算可读取的数据量
		available := writeHead - readTail
		if available == 0 {
			return 0, nil // 缓冲区为空
		}

		// 限制读取长度
		if dataLen > available {
			dataLen = available
		}

		// 尝试原子性地更新读取位置
		newReadTail := readTail + dataLen
		if atomic.CompareAndSwapUint64(&rb.readTail, readTail, newReadTail) {
			// 成功获取读取位置，现在可以安全地读取数据
			break
		}

		// CAS失败，重新加载读取位置并重试
		readTail = atomic.LoadUint64(&rb.readTail)
	}

	// 计算实际读取位置
	readPos := readTail & rb.mask

	// 使用内存屏障确保读取操作的顺序性
	atomic.LoadUint64((*uint64)(unsafe.Pointer(&rb.buffer[readPos]))) // 强制内存屏障

	// 执行实际的数据读取
	if readPos+dataLen <= rb.size {
		// 一次性读取，不需要环绕
		copy(data, rb.buffer[readPos:readPos+dataLen])
	} else {
		// 需要分两次读取
		firstPart := rb.size - readPos
		copy(data[:firstPart], rb.buffer[readPos:])
		copy(data[firstPart:], rb.buffer[:dataLen-firstPart])
	}

	// 再次使用内存屏障确保数据读取完成
	atomic.LoadUint64((*uint64)(unsafe.Pointer(&rb.buffer[readPos]))) // 强制内存屏障

	return int(dataLen), nil
}

// WriteBatch 批量写入多个数据块
// 减少原子操作次数，提高性能
func (rb *RingBuffer) WriteBatch(batches [][]byte) (int, error) {
	// 计算总长度
	totalLen := 0
	for _, batch := range batches {
		totalLen += len(batch)
	}

	if totalLen == 0 {
		return 0, nil
	}

	// 原子获取当前写入位置
	writeHead := atomic.LoadUint64(&rb.writeHead)
	readTail := atomic.LoadUint64(&rb.readTail)

	// 检查是否有足够空间
	available := rb.size - (writeHead - readTail)
	if uint64(totalLen) > available {
		return 0, errors.New("ring buffer: insufficient space for batch")
	}

	// 批量写入所有数据块
	writePos := writeHead & rb.mask
	currentWritePos := writePos

	for _, batch := range batches {
		batchLen := uint64(len(batch))
		if batchLen == 0 {
			continue
		}

		// 检查是否需要环绕
		if currentWritePos+batchLen <= rb.size {
			// 一次性写入
			copy(rb.buffer[currentWritePos:], batch)
			currentWritePos += batchLen
		} else {
			// 分两次写入
			firstPart := rb.size - currentWritePos
			copy(rb.buffer[currentWritePos:], batch[:firstPart])
			copy(rb.buffer[0:], batch[firstPart:])
			currentWritePos = batchLen - firstPart
		}
	}

	// 一次性更新写入位置
	atomic.AddUint64(&rb.writeHead, uint64(totalLen))

	return totalLen, nil
}

// ReadBatch 批量读取数据到多个缓冲区
// 减少原子操作次数，提高性能
func (rb *RingBuffer) ReadBatch(buffers [][]byte) (int, error) {
	// 计算总容量
	totalCap := 0
	for _, buf := range buffers {
		totalCap += cap(buf)
	}

	if totalCap == 0 {
		return 0, nil
	}

	// 原子获取当前读取位置
	writeHead := atomic.LoadUint64(&rb.writeHead)
	readTail := atomic.LoadUint64(&rb.readTail)

	// 计算可读取的数据量
	available := writeHead - readTail
	if available == 0 {
		return 0, nil // 缓冲区为空
	}

	// 限制读取长度
	readLen := available
	if uint64(totalCap) < readLen {
		readLen = uint64(totalCap)
	}

	// 批量读取数据
	readPos := readTail & rb.mask
	currentReadPos := readPos
	remaining := readLen
	totalRead := 0

	for _, buf := range buffers {
		bufCap := cap(buf)
		if bufCap == 0 || remaining == 0 {
			continue
		}

		// 计算本次读取长度
		thisRead := remaining
		if uint64(bufCap) < thisRead {
			thisRead = uint64(bufCap)
		}

		// 检查是否需要环绕
		if currentReadPos+thisRead <= rb.size {
			// 一次性读取
			copy(buf, rb.buffer[currentReadPos:currentReadPos+thisRead])
			currentReadPos += thisRead
		} else {
			// 分两次读取
			firstPart := rb.size - currentReadPos
			copy(buf[:firstPart], rb.buffer[currentReadPos:])
			copy(buf[firstPart:], rb.buffer[:thisRead-firstPart])
			currentReadPos = thisRead - firstPart
		}

		remaining -= thisRead
		totalRead += int(thisRead)
	}

	// 一次性更新读取位置
	atomic.AddUint64(&rb.readTail, readLen)

	return totalRead, nil
}
