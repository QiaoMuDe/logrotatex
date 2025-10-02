package logrotatex

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"
)

// 专测 NewStdoutBW：写入+Flush 输出到 stdout，Close 不关闭 stdout
func TestNewStdoutBW(t *testing.T) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	// 禁用三重条件自动刷新，便于精确控制
	cfg := &BufCfg{MaxBufferSize: 0, MaxWriteCount: 0, FlushInterval: 0}
	bw := NewStdoutBW(cfg)

	data1 := []byte("hello\n")
	if _, err := bw.Write(data1); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if err := bw.Flush(); err != nil {
		t.Fatalf("Flush error: %v", err)
	}

	// 关闭 BufferedWriter，不应关闭 stdout
	if err := bw.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if !bw.IsClosed() {
		t.Fatalf("BufferedWriter should be closed")
	}

	// 仍可直接向 stdout 写入
	data2 := []byte("world\n")
	if _, err := w.Write(data2); err != nil {
		t.Fatalf("stdout should remain writable: %v", err)
	}

	_ = w.Close()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	want := append(data1, data2...)
	if !bytes.Equal(got, want) {
		t.Fatalf("output mismatch: want %q, got %q", string(want), string(got))
	}
}

// 缓冲区大小触发：达到 MaxBufferSize 时自动写出
func TestStdoutBW_FlushOnMaxBufferSize(t *testing.T) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	cfg := &BufCfg{MaxBufferSize: 10, MaxWriteCount: 0, FlushInterval: 0}
	bw := NewStdoutBW(cfg)

	// 先写不足阈值的数据，不应触发写出
	part1 := []byte("12345") // 5
	if _, err := bw.Write(part1); err != nil {
		t.Fatalf("Write part1 error: %v", err)
	}
	if bw.BufferSize() != len(part1) {
		t.Fatalf("buffer size mismatch: want %d got %d", len(part1), bw.BufferSize())
	}

	// 再写使总长度达到阈值，应该触发一次写出（10字节）
	part2 := []byte("67890") // 5 => total 10
	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 10)
		_, _ = io.ReadFull(r, buf)
		done <- buf
	}()

	if _, err := bw.Write(part2); err != nil {
		t.Fatalf("Write part2 error: %v", err)
	}

	// 读取到的应是前10字节
	select {
	case got := <-done:
		want := append(part1, part2...)
		if !bytes.Equal(got, want) {
			t.Fatalf("flush content mismatch: want %q got %q", string(want), string(got))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timeout waiting for flush on MaxBufferSize")
	}

	// 清理
	_ = w.Close()
	_, _ = io.ReadAll(r)
}

// 写入次数触发：达到 MaxWriteCount 次写入时自动写出
func TestStdoutBW_FlushOnMaxWriteCount(t *testing.T) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	cfg := &BufCfg{MaxBufferSize: 0, MaxWriteCount: 3, FlushInterval: 0}
	bw := NewStdoutBW(cfg)

	data1 := []byte("a")
	data2 := []byte("b")
	data3 := []byte("c")
	want := []byte("abc")

	// 前两次写入不触发
	if _, err := bw.Write(data1); err != nil {
		t.Fatalf("Write 1 error: %v", err)
	}
	if bw.WriteCount() != 1 {
		t.Fatalf("write count mismatch: want 1 got %d", bw.WriteCount())
	}
	if _, err := bw.Write(data2); err != nil {
		t.Fatalf("Write 2 error: %v", err)
	}
	if bw.WriteCount() != 2 {
		t.Fatalf("write count mismatch: want 2 got %d", bw.WriteCount())
	}

	// 第三次写入应触发写出
	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, len(want))
		_, _ = io.ReadFull(r, buf)
		done <- buf
	}()

	if _, err := bw.Write(data3); err != nil {
		t.Fatalf("Write 3 error: %v", err)
	}

	select {
	case got := <-done:
		if !bytes.Equal(got, want) {
			t.Fatalf("flush content mismatch: want %q got %q", string(want), string(got))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timeout waiting for flush on MaxWriteCount")
	}

	// 清理
	_ = w.Close()
	_, _ = io.ReadAll(r)
}

// 刷新间隔触发：超过 FlushInterval 后的下一次写入触发写出
func TestStdoutBW_FlushOnInterval(t *testing.T) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	cfg := &BufCfg{MaxBufferSize: 0, MaxWriteCount: 0, FlushInterval: 50 * time.Millisecond}
	bw := NewStdoutBW(cfg)

	first := []byte("interval")
	if _, err := bw.Write(first); err != nil {
		t.Fatalf("Write first error: %v", err)
	}

	// 等待超过刷新间隔
	time.Sleep(80 * time.Millisecond)

	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, len(first))
		_, _ = io.ReadFull(r, buf)
		done <- buf
	}()

	// 下一次写入应触发将缓冲区（包含 first）写出
	if _, err := bw.Write([]byte("x")); err != nil {
		t.Fatalf("Write trigger error: %v", err)
	}

	select {
	case got := <-done:
		if !bytes.Equal(got, first) {
			t.Fatalf("flush content mismatch: want %q got %q", string(first), string(got))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timeout waiting for flush on interval")
	}

	// 清理
	_ = w.Close()
	_, _ = io.ReadAll(r)
}

// 验证：写入次数触发刷新后，计数与缓冲区被重置，避免频繁终端输出
func TestStdoutBW_WriteCountResetAfterFlush(t *testing.T) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	// 仅启用写入次数触发
	cfg := &BufCfg{MaxBufferSize: 0, MaxWriteCount: 2, FlushInterval: 0}
	bw := NewStdoutBW(cfg)

	// 第1、2次写入应在第二次后触发自动刷新
	data1 := []byte("a")
	data2 := []byte("b")
	wantBatch := []byte("ab")

	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, len(wantBatch))
		_, _ = io.ReadFull(r, buf)
		done <- buf
	}()

	if _, err := bw.Write(data1); err != nil {
		t.Fatalf("Write 1 error: %v", err)
	}
	if _, err := bw.Write(data2); err != nil {
		t.Fatalf("Write 2 error: %v", err)
	}

	// 应读到批量写出的 "ab"
	gotBatch := <-done
	if !bytes.Equal(gotBatch, wantBatch) {
		t.Fatalf("flush content mismatch: want %q got %q", string(wantBatch), string(gotBatch))
	}

	// 刷新后计数与缓冲区应被重置
	if bw.WriteCount() != 0 {
		t.Fatalf("write count should reset to 0, got %d", bw.WriteCount())
	}
	if bw.BufferSize() != 0 {
		t.Fatalf("buffer size should reset to 0, got %d", bw.BufferSize())
	}

	// 再写入一条数据，此时不应立即刷新（因为只写了一次）
	data3 := []byte("c")
	if _, err := bw.Write(data3); err != nil {
		t.Fatalf("Write 3 error: %v", err)
	}
	if bw.BufferSize() != len(data3) {
		t.Fatalf("buffer size mismatch after Write 3: want %d got %d", len(data3), bw.BufferSize())
	}

	// 手动 Flush 写出剩余数据
	if err := bw.Flush(); err != nil {
		t.Fatalf("Flush error: %v", err)
	}

	// 关闭写端并读取剩余数据
	_ = w.Close()
	gotRest, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll rest error: %v", err)
	}
	if !bytes.Equal(gotRest, data3) {
		t.Fatalf("rest content mismatch: want %q got %q", string(data3), string(gotRest))
	}
}

// 验证 WrapWriter：Close 为无操作，关闭后仍可向底层 writer 写入
func TestWrapWriter_NoClose(t *testing.T) {
	// 使用管道模拟 stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	defer func() { _ = r.Close() }() // 读端由测试关闭
	// 写端由我们手动控制关闭时机

	wc := WrapWriter(w) // 不可关闭包装器
	data1 := []byte("hello")
	if _, err := wc.Write(data1); err != nil {
		t.Fatalf("wc.Write error: %v", err)
	}

	// 关闭包装器，应为 no-op
	if err := wc.Close(); err != nil {
		t.Fatalf("wc.Close should return nil, got %v", err)
	}

	// 关闭后仍可继续写入到底层 writer
	data2 := []byte("world")
	if _, err := w.Write(data2); err != nil {
		t.Fatalf("underlying writer should remain writable after wc.Close: %v", err)
	}

	// 关闭写端，读取全部数据
	_ = w.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}

	want := append(data1, data2...)
	if !bytes.Equal(got, want) {
		t.Fatalf("content mismatch: want %q got %q", string(want), string(got))
	}
}

// 验证 WrapWriter 与 BufferedWriter 集成：bw.Close 不关闭底层 writer
func TestWrapWriter_WithBufferedWriter(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	defer func() { _ = r.Close() }()

	// 使用 WrapWriter 包装底层 writer，确保不可关闭
	bw := NewBufferedWriter(WrapWriter(w), &BufCfg{
		MaxBufferSize: 0,
		MaxWriteCount: 2, // 两次写入触发刷新
		FlushInterval: 0,
	})

	// 第1、2次写入触发自动刷出
	if _, err := bw.Write([]byte("A")); err != nil {
		t.Fatalf("Write A error: %v", err)
	}
	if _, err := bw.Write([]byte("B")); err != nil {
		t.Fatalf("Write B error: %v", err)
	}

	// 读取已刷出的数据
	buf := make([]byte, 2)
	if _, err := io.ReadFull(r, buf); err != nil {
		t.Fatalf("ReadFull AB error: %v", err)
	}
	if !bytes.Equal(buf, []byte("AB")) {
		t.Fatalf("flushed content mismatch: want %q got %q", "AB", string(buf))
	}

	// 关闭 BufferedWriter，不应关闭底层 writer
	if err := bw.Close(); err != nil {
		t.Fatalf("bw.Close error: %v", err)
	}

	// 仍可直接向底层 writer 写入
	if _, err := w.Write([]byte("C")); err != nil {
		t.Fatalf("underlying writer should remain writable after bw.Close: %v", err)
	}

	// 关闭写端，读取剩余数据
	_ = w.Close()
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll rest error: %v", err)
	}
	if !bytes.Equal(rest, []byte("C")) {
		t.Fatalf("rest content mismatch: want %q got %q", "C", string(rest))
	}
}
