// ring_buffer_io.go - 环形缓冲区的核心读写操作实现
// 包含无锁并发读写的基本操作
package lockfree

import (
	"errors"
	"sync/atomic"
	"unsafe"
)

// Write 向环形缓冲区写入数据
// 返回实际写入的字节数和可能的错误
func (rb *RingBuffer) Write(data []byte) (int, error) {
	dataLen := uint64(len(data))
	if dataLen == 0 {
		return 0, nil
	}

	// 原子获取当前写入位置
	writeHead := atomic.LoadUint64(&rb.writeHead)
	readTail := atomic.LoadUint64(&rb.readTail)

	// 计算可用空间
	available := rb.size - (writeHead - readTail)

	// 检查是否有足够空间
	if dataLen > available {
		return 0, errors.New("ring buffer: insufficient space")
	}

	// 计算写入位置
	writePos := writeHead & rb.mask

	// 检查是否需要分两次写入（环绕情况）
	if writePos+dataLen <= rb.size {
		// 一次性写入，不需要环绕
		copy(rb.buffer[writePos:], data)
	} else {
		// 需要分两次写入
		firstPart := rb.size - writePos
		copy(rb.buffer[writePos:], data[:firstPart])
		copy(rb.buffer[0:], data[firstPart:])
	}

	// 原子更新写入位置
	atomic.AddUint64(&rb.writeHead, dataLen)

	return int(dataLen), nil
}

// Read 从环形缓冲区读取数据
// 返回实际读取的字节数和可能的错误
func (rb *RingBuffer) Read(data []byte) (int, error) {
	dataLen := uint64(cap(data))
	if dataLen == 0 {
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
	if dataLen > available {
		dataLen = available
	}

	// 计算读取位置
	readPos := readTail & rb.mask

	// 检查是否需要分两次读取（环绕情况）
	if readPos+dataLen <= rb.size {
		// 一次性读取，不需要环绕
		copy(data, rb.buffer[readPos:readPos+dataLen])
	} else {
		// 需要分两次读取
		firstPart := rb.size - readPos
		copy(data[:firstPart], rb.buffer[readPos:])
		copy(data[firstPart:], rb.buffer[:dataLen-firstPart])
	}

	// 原子更新读取位置
	atomic.AddUint64(&rb.readTail, dataLen)

	return int(dataLen), nil
}

// TryWrite 尝试写入，非阻塞版本
// 如果空间不足立即返回错误，不等待
func (rb *RingBuffer) TryWrite(data []byte) error {
	_, err := rb.Write(data)
	return err
}

// TryRead 尝试读取，非阻塞版本
// 如果没有数据立即返回错误，不等待
func (rb *RingBuffer) TryRead(data []byte) (int, error) {
	n, err := rb.Read(data)
	if n == 0 && err == nil {
		return 0, errors.New("ring buffer: no data available")
	}
	return n, err
}

// WriteByte 写入单个字节
func (rb *RingBuffer) WriteByte(b byte) error {
	_, err := rb.Write([]byte{b})
	return err
}

// ReadByte 读取单个字节
func (rb *RingBuffer) ReadByte() (byte, error) {
	buf := make([]byte, 1)
	_, err := rb.Read(buf)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}

// WriteFast 快速写入，不进行空间检查
// 调用者必须确保有足够空间，否则可能导致数据损坏
func (rb *RingBuffer) WriteFast(data []byte) int {
	dataLen := uint64(len(data))
	if dataLen == 0 {
		return 0
	}

	// 原子获取当前写入位置
	writeHead := atomic.LoadUint64(&rb.writeHead)
	writePos := writeHead & rb.mask

	// 直接写入，不检查空间
	if writePos+dataLen <= rb.size {
		copy(rb.buffer[writePos:], data)
	} else {
		firstPart := rb.size - writePos
		copy(rb.buffer[writePos:], data[:firstPart])
		copy(rb.buffer[0:], data[firstPart:])
	}

	// 原子更新写入位置
	atomic.AddUint64(&rb.writeHead, dataLen)

	return int(dataLen)
}

// ReadFast 快速读取，不进行数据检查
// 调用者必须确保有数据可读，否则可能读取到无效数据
func (rb *RingBuffer) ReadFast(data []byte) int {
	dataLen := uint64(cap(data))
	if dataLen == 0 {
		return 0
	}

	// 原子获取当前读取位置
	writeHead := atomic.LoadUint64(&rb.writeHead)
	readTail := atomic.LoadUint64(&rb.readTail)

	// 计算可读取的数据量
	available := writeHead - readTail
	if available == 0 {
		return 0
	}

	// 限制读取长度
	if dataLen > available {
		dataLen = available
	}

	// 计算读取位置
	readPos := readTail & rb.mask

	// 直接读取，不检查数据有效性
	if readPos+dataLen <= rb.size {
		copy(data, rb.buffer[readPos:readPos+dataLen])
	} else {
		firstPart := rb.size - readPos
		copy(data[:firstPart], rb.buffer[readPos:])
		copy(data[firstPart:], rb.buffer[:dataLen-firstPart])
	}

	// 原子更新读取位置
	atomic.AddUint64(&rb.readTail, dataLen)

	return int(dataLen)
}

// WriteV 写入多个数据块，类似writev系统调用
func (rb *RingBuffer) WriteV(data [][]byte) (int, error) {
	return rb.WriteBatch(data)
}

// ReadV 读取数据到多个缓冲区，类似readv系统调用
func (rb *RingBuffer) ReadV(data [][]byte) (int, error) {
	return rb.ReadBatch(data)
}

// WriteUint32 写入32位整数（小端序）
func (rb *RingBuffer) WriteUint32(value uint32) error {
	buf := [4]byte{
		byte(value),
		byte(value >> 8),
		byte(value >> 16),
		byte(value >> 24),
	}
	_, err := rb.Write(buf[:])
	return err
}

// ReadUint32 读取32位整数（小端序）
func (rb *RingBuffer) ReadUint32() (uint32, error) {
	buf := make([]byte, 4)
	_, err := rb.Read(buf)
	if err != nil {
		return 0, err
	}
	return uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24, nil
}

// WriteUint64 写入64位整数（小端序）
func (rb *RingBuffer) WriteUint64(value uint64) error {
	buf := [8]byte{
		byte(value),
		byte(value >> 8),
		byte(value >> 16),
		byte(value >> 24),
		byte(value >> 32),
		byte(value >> 40),
		byte(value >> 48),
		byte(value >> 56),
	}
	_, err := rb.Write(buf[:])
	return err
}

// ReadUint64 读取64位整数（小端序）
func (rb *RingBuffer) ReadUint64() (uint64, error) {
	buf := make([]byte, 8)
	_, err := rb.Read(buf)
	if err != nil {
		return 0, err
	}
	return uint64(buf[0]) | uint64(buf[1])<<8 | uint64(buf[2])<<16 | uint64(buf[3])<<24 |
		uint64(buf[4])<<32 | uint64(buf[5])<<40 | uint64(buf[6])<<48 | uint64(buf[7])<<56, nil
}

// Reserve 预留指定大小的空间，返回可写入的缓冲区切片
// 调用者可以直接写入返回的切片，避免额外的内存拷贝
func (rb *RingBuffer) Reserve(size int) ([]byte, error) {
	if size <= 0 {
		return nil, errors.New("ring buffer: invalid reserve size")
	}

	dataLen := uint64(size)

	// 原子获取当前写入位置
	writeHead := atomic.LoadUint64(&rb.writeHead)
	readTail := atomic.LoadUint64(&rb.readTail)

	// 计算可用空间
	available := rb.size - (writeHead - readTail)

	// 检查是否有足够空间
	if dataLen > available {
		return nil, errors.New("ring buffer: insufficient space")
	}

	// 计算写入位置
	writePos := writeHead & rb.mask

	// 检查是否需要分两次写入（环绕情况）
	if writePos+dataLen <= rb.size {
		// 一次性预留，不需要环绕
		buf := rb.buffer[writePos : writePos+dataLen]
		atomic.AddUint64(&rb.writeHead, dataLen)
		return buf, nil
	} else {
		// 需要分两次写入，无法提供连续的缓冲区
		return nil, errors.New("ring buffer: cannot reserve contiguous space due to wraparound")
	}
}

// Commit 提交预留的空间中实际使用的部分
// 与Reserve配合使用，当实际写入的数据少于预留的空间时调用
func (rb *RingBuffer) Commit(used int) {
	if used < 0 {
		return
	}

	// 减少写入位置，只保留实际使用的部分
	actualUsed := uint64(used)
	atomic.AddUint64(&rb.writeHead, ^actualUsed+1) // 相当于 subtract
}

// Drain 排空缓冲区，返回所有数据
func (rb *RingBuffer) Drain() []byte {
	// 计算当前数据量
	dataLen := rb.Used()
	if dataLen == 0 {
		return nil
	}

	// 创建足够大的缓冲区
	result := make([]byte, dataLen)

	// 读取所有数据
	n, _ := rb.Read(result)

	// 返回实际读取的数据
	return result[:n]
}

// Capacity 返回缓冲区的总容量
func (rb *RingBuffer) Capacity() int {
	return int(rb.size)
}

// Length 返回当前缓冲区中的数据长度
func (rb *RingBuffer) Length() int {
	return rb.Used()
}

// Stats 返回缓冲区的统计信息
type BufferStats struct {
	Size      int  // 缓冲区总大小
	Used      int  // 已使用空间
	Available int  // 可用空间
	IsEmpty   bool // 是否为空
	IsFull    bool // 是否已满
}

// Stats 返回缓冲区的统计信息
func (rb *RingBuffer) Stats() BufferStats {
	used := rb.Used()
	available := rb.Available()
	return BufferStats{
		Size:      rb.Size(),
		Used:      used,
		Available: available,
		IsEmpty:   used == 0,
		IsFull:    available == 0,
	}
}

// MemoryBarrier 强制内存屏障，确保内存操作的顺序性
func (rb *RingBuffer) MemoryBarrier() {
	// 使用原子操作强制内存屏障
	atomic.StoreUint64((*uint64)(unsafe.Pointer(&rb.buffer[0])), 0)
	atomic.LoadUint64((*uint64)(unsafe.Pointer(&rb.buffer[0])))
}

// AlignToCacheLine 对齐到缓存行边界，提高性能
func AlignToCacheLine(size int) int {
	const cacheLineSize = 64
	return (size + cacheLineSize - 1) & ^(cacheLineSize - 1)
}

// IsPowerOfTwo 检查数字是否是2的幂次方
func IsPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

// RoundUpToPowerOfTwo 向上舍入到最近的2的幂次方
func RoundUpToPowerOfTwo(n int) int {
	if n <= 0 {
		return 1
	}

	if IsPowerOfTwo(n) {
		return n
	}

	return nextPowerOfTwo(n)
}
