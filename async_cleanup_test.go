/* async_cleanup_test.go 包含对异步清理机制的测试，用于验证轮转触发的后台清理、合并触发与关闭语义。*/
package logrotatex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// waitUntil 封装轮询等待，直到 cond 返回 true 或超时
func waitUntil(t *testing.T, timeout time.Duration, interval time.Duration, cond func() bool, onTimeoutMsg string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatal(onTimeoutMsg)
}

// TestAsyncCleanup_BasicTrigger
// 开启 Async 后，触发轮转应启动异步清理；超出 MaxFiles 的旧文件会被删除。
func TestAsyncCleanup_BasicTrigger(t *testing.T) {
	dir := makeBoundaryTempDir("TestAsyncCleanup_BasicTrigger", t)
	defer func() { _ = os.RemoveAll(dir) }()

	// 构造一些旧文件（包含时间戳但不压缩）
	oldFiles := []string{
		"test_20250101100000.log",
		"test_20250102100000.log",
		"test_20250103100000.log",
	}
	for _, name := range oldFiles {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0644); err != nil {
			t.Fatalf("创建旧文件失败: %v", err)
		}
	}

	l := &LogRotateX{
		LogFilePath: filepath.Join(dir, "test.log"),
		MaxSize:     1, // 1MB
		MaxFiles:    1, // 最多保留1个
		Compress:    false,
		Async:       true,
	}
	defer func() { _ = l.Close() }()

	// 写入触发轮转，从而调用 cleanupAsync
	data := make([]byte, megabyte+1)
	_, err := l.Write(data)
	if err != nil {
		t.Fatalf("写入触发轮转失败: %v", err)
	}

	// 等待异步清理完成：目录里应只剩 1 个旧文件（最新），加上当前 test.log
	waitUntil(t, 2*time.Second, 100*time.Millisecond, func() bool {
		files, _ := os.ReadDir(dir)
		oldCount := 0
		for _, f := range files {
			if strings.HasPrefix(f.Name(), "test_") && strings.HasSuffix(f.Name(), ".log") && f.Name() != "test.log" {
				oldCount++
			}
		}
		return oldCount <= 1
	}, "异步清理未在预期时间内收敛至仅保留1个旧文件")
}

// TestAsyncCleanup_MergeTriggersSingleWorker
// 已在运行时再次触发，应仅设置 rerunNeeded；最终两轮都完成，避免并发协程。
func TestAsyncCleanup_MergeTriggersSingleWorker(t *testing.T) {
	dir := makeBoundaryTempDir("TestAsyncCleanup_MergeTriggersSingleWorker", t)
	defer func() { _ = os.RemoveAll(dir) }()

	// 预置两批旧文件，分两次触发
	batch1 := []string{"test_20250104100000.log", "test_20250105100000.log"}
	for _, name := range batch1 {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("b1"), 0644); err != nil {
			t.Fatalf("创建旧文件失败: %v", err)
		}
	}

	l := &LogRotateX{
		LogFilePath: filepath.Join(dir, "test.log"),
		MaxSize:     1,
		MaxFiles:    1,
		Compress:    false,
		Async:       true,
	}
	defer func() { _ = l.Close() }()

	// 第一次触发：写入超过上限
	_, err := l.Write(make([]byte, megabyte+1))
	if err != nil {
		t.Fatalf("写入触发轮转失败: %v", err)
	}

	// 在协程运行中，追加第二批旧文件，并再次触发
	batch2 := []string{"test_20250106100000.log", "test_20250107100000.log"}
	for _, name := range batch2 {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("b2"), 0644); err != nil {
			t.Fatalf("创建旧文件失败: %v", err)
		}
	}
	// 再次触发（不应启动新的协程，仅设置 rerunNeeded）
	l.cleanupAsync()

	// 等待所有清理完成：目录里应最多保留 1 个旧日志（最新），其余被删除
	waitUntil(t, 3*time.Second, 100*time.Millisecond, func() bool {
		files, _ := os.ReadDir(dir)
		oldCount := 0
		for _, f := range files {
			if strings.HasPrefix(f.Name(), "test_") && strings.HasSuffix(f.Name(), ".log") && f.Name() != "test.log" {
				oldCount++
			}
		}
		return oldCount <= 1
	}, "合并触发的异步清理未在预期时间内收敛")
}

// TestAsyncCleanup_CloseWaits
// 异步清理触发后调用 Close，应等待后台协程结束（通过最终状态验证）。
func TestAsyncCleanup_CloseWaits(t *testing.T) {
	dir := makeBoundaryTempDir("TestAsyncCleanup_CloseWaits", t)
	defer func() { _ = os.RemoveAll(dir) }()

	// 在子目录中进行隔离，避免其他用例影响
	subdir := filepath.Join(dir, "closewaits")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("创建子目录失败: %v", err)
	}

	// 创建多一些旧文件，保证清理有工作量（放在子目录中）
	for i := 0; i < 5; i++ {
		// 使用标准备份时间格式 20060102150405，确保 fastTimeFromName 可解析
		tstamp := time.Date(2025, 1, 1, 0, i, 0, 0, time.Local)
		name := filepath.Join(subdir, "test_"+tstamp.Format(backupTimeFormat)+".log")
		if err := os.WriteFile(name, []byte("x"), 0644); err != nil {
			t.Fatalf("创建旧文件失败: %v", err)
		}
	}

	l := &LogRotateX{
		LogFilePath: filepath.Join(subdir, "test.log"),
		MaxSize:     1,
		MaxFiles:    2, // 最终应保留最多2个旧文件
		Compress:    false,
		Async:       true,
	}

	// 写入触发轮转与异步清理
	_, err := l.Write(make([]byte, megabyte+1))
	if err != nil {
		t.Fatalf("写入触发轮转失败: %v", err)
	}

	// 调用 Close，应等待清理结束后返回
	if cerr := l.Close(); cerr != nil {
		t.Fatalf("关闭失败: %v", cerr)
	}

	// 验证最终旧文件数量不超过2
	files, _ := os.ReadDir(subdir)
	oldCount := 0
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "test_") && strings.HasSuffix(f.Name(), ".log") && f.Name() != "test.log" {
			oldCount++
		}
	}
	if oldCount > 2 {
		t.Fatalf("Close 后异步清理未收敛，旧文件数量=%d (应<=2)", oldCount)
	}
}
