// logrotatex_functional_test.go - LogRotateX功能测试用例
// 测试LogRotateX的核心功能，包括日志写入、文件轮转、压缩处理、时间控制等
package logrotatex

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLogRotateX_BasicWrite 测试基本写入功能
func TestLogRotateX_BasicWrite(t *testing.T) {
	// 保存原始时间函数
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	// 使用固定时间确保测试可重复
	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("TestLogRotateX_BasicWrite", t)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(logFile(dir))
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 写入测试数据
	testData := []byte("Hello, LogRotateX!")
	n, err := logger.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch. Got: %d, Want: %d", n, len(testData))
	}

	// 验证文件存在且内容正确
	existsWithContent(logFile(dir), testData, t)
	fileCount(dir, 1, t)
}

// TestLogRotateX_FileRotation 测试文件轮转功能
func TestLogRotateX_FileRotation(t *testing.T) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("TestLogRotateX_FileRotation", t)
	defer func() { _ = os.RemoveAll(dir) }()

	var ll *LogRotateX
	fmt.Printf("ll: %v\n", ll)

	// 创建小文件大小的logger，便于测试轮转
	logger := NewLogRotateX(logFile(dir))
	logger.MaxSize = 1 // 1MB
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 写入足够多的数据触发轮转
	testData := strings.Repeat("This is a test line.\n", 10000)
	for i := 0; i < 110; i++ {
		var sb strings.Builder
		sb.WriteString("Log line ")
		sb.WriteString(fmt.Sprintf("%d", i))
		sb.WriteString(": ")
		sb.WriteString(testData)
		_, err := logger.Write([]byte(sb.String()))
		if err != nil {
			t.Fatalf("Write failed on iteration %d: %v", i, err)
		}
	}

	// 验证轮转后应该有多个文件
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}
	if len(files) < 2 {
		t.Errorf("Expected at least 2 files after rotation, got %d", len(files))
	}

	// 验证当前日志文件存在
	if _, err := os.Stat(logFile(dir)); os.IsNotExist(err) {
		t.Errorf("Current log file does not exist after rotation")
	}
}

// TestLogRotateX_DailyRotation 测试按天轮转功能
func TestLogRotateX_DailyRotation(t *testing.T) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	// 设置为第一天
	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("TestLogRotateX_DailyRotation", t)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(logFile(dir))
	logger.RotateByDay = true
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 写入第一天日志
	_, err := logger.Write([]byte("Day 1 log\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// 切换到第二天
	currentTime = func() time.Time {
		return time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)
	}

	// 写入第二天日志，应该触发轮转
	_, err = logger.Write([]byte("Day 2 log\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// 验证应该有轮转文件
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}
	if len(files) < 2 {
		t.Errorf("Expected at least 2 files after daily rotation, got %d", len(files))
	}
}

// TestLogRotateX_DateDirectoryLayout 测试日期目录布局功能
func TestLogRotateX_DateDirectoryLayout(t *testing.T) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 12, 30, 0, 0, time.UTC)
	}

	dir := makeTempDir("TestLogRotateX_DateDirectoryLayout", t)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(logFile(dir))
	logger.DateDirLayout = true
	logger.MaxSize = 1 // 1MB，便于触发轮转
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 写入足够多的数据触发轮转
	testData := strings.Repeat("This is a test line.\n", 10000)
	for i := 0; i < 110; i++ {
		var sb strings.Builder
		sb.WriteString("Log line ")
		sb.WriteString(fmt.Sprintf("%d", i))
		sb.WriteString(": ")
		sb.WriteString(testData)
		_, err := logger.Write([]byte(sb.String()))
		if err != nil {
			t.Fatalf("Write failed on iteration %d: %v", i, err)
		}
	}

	// 验证日期目录存在
	dateDir := filepath.Join(dir, "2023-01-01")
	if _, err := os.Stat(dateDir); os.IsNotExist(err) {
		t.Errorf("Expected date directory %s to exist", dateDir)
	}

	// 验证日期目录中有轮转文件
	dateFiles, err := os.ReadDir(dateDir)
	if err != nil {
		t.Fatalf("Failed to read date directory: %v", err)
	}
	if len(dateFiles) == 0 {
		t.Errorf("Expected at least one file in date directory")
	}
}

// TestLogRotateX_Compression 测试压缩功能
func TestLogRotateX_Compression(t *testing.T) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("TestLogRotateX_Compression", t)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(logFile(dir))
	logger.MaxSize = 1 // 1MB，便于触发轮转
	logger.Compress = true
	logger.DateDirLayout = false // 禁用日期目录，简化测试
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 写入足够多的数据触发轮转
	testData := strings.Repeat("This is a test line.\n", 10000)
	for i := 0; i < 110; i++ {
		var sb strings.Builder
		sb.WriteString("Log line ")
		sb.WriteString(fmt.Sprintf("%d", i))
		sb.WriteString(": ")
		sb.WriteString(testData)
		_, err := logger.Write([]byte(sb.String()))
		if err != nil {
			t.Fatalf("Write failed on iteration %d: %v", i, err)
		}
	}

	// 等待一段时间，确保压缩操作完成
	time.Sleep(500 * time.Millisecond)

	// 手动触发清理操作，包括压缩
	if err := logger.cleanupSync(); err != nil {
		t.Logf("Cleanup sync failed: %v", err)
	}

	// 查找压缩文件
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	var compressedFiles []string
	for _, file := range files {
		// 检查是否有压缩文件扩展名
		if strings.HasSuffix(file.Name(), ".zip") {
			compressedFiles = append(compressedFiles, file.Name())
		}
	}

	if len(compressedFiles) == 0 {
		// 打印所有文件名，帮助调试
		var fileNames []string
		for _, file := range files {
			fileNames = append(fileNames, file.Name())
		}
		t.Errorf("Expected at least one compressed file, but found: %v", fileNames)
	}

	// 验证压缩文件可以正常解压
	for _, fileName := range compressedFiles {
		filePath := filepath.Join(dir, fileName)
		zipReader, err := zip.OpenReader(filePath)
		if err != nil {
			t.Fatalf("Failed to open zip file %s: %v", fileName, err)
		}
		if err := zipReader.Close(); err != nil {
			t.Errorf("Failed to close zip file %s: %v", fileName, err)
		}
	}
}

// TestLogRotateX_CleanupRules 测试清理规则
func TestLogRotateX_CleanupRules(t *testing.T) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("TestLogRotateX_CleanupRules", t)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(logFile(dir))
	logger.MaxSize = 1  // 1MB，便于触发轮转
	logger.MaxFiles = 3 // 最多保留3个文件
	// logger.MaxAge = 7          // 最多保留7天 - 注释掉，只测试文件数量限制
	logger.DateDirLayout = false // 禁用日期目录，简化测试
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 创建多个轮转文件
	testData := strings.Repeat("This is a test line.\n", 10000)
	for i := 0; i < 5; i++ {
		// 每次写入前推进时间，确保有不同的时间戳
		currentTime = func() time.Time {
			return time.Date(2023, 1, i+1, 0, 0, 0, 0, time.UTC)
		}

		for j := 0; j < 110; j++ {
			var sb strings.Builder
			sb.WriteString("Log line ")
			sb.WriteString(fmt.Sprintf("%d-%d", i, j))
			sb.WriteString(": ")
			sb.WriteString(testData)
			_, err := logger.Write([]byte(sb.String()))
			if err != nil {
				t.Fatalf("Write failed on iteration %d-%d: %v", i, j, err)
			}
		}
	}

	// 等待一段时间，确保清理操作完成
	time.Sleep(500 * time.Millisecond)

	// 手动触发清理操作
	if err := logger.cleanupSync(); err != nil {
		t.Logf("Cleanup sync failed: %v", err)
	}

	// 验证文件数量不超过MaxFiles限制
	// 使用 oldLogFiles 方法获取备份文件列表
	backupFiles, err := logger.oldLogFiles()
	if err != nil {
		t.Fatalf("Failed to get old log files: %v", err)
	}

	// 打印所有文件名，帮助调试
	allFiles, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}
	var fileNames []string
	for _, file := range allFiles {
		fileNames = append(fileNames, file.Name())
	}
	t.Logf("Found files: %v", fileNames)
	t.Logf("Backup files count: %d", len(backupFiles))

	// 当前日志文件 + 最多3个备份文件
	// 注意：oldLogFiles 只返回备份文件，不包括当前文件
	// 所以这里检查的是备份文件数量，而不是总文件数量
	if len(backupFiles) > 3 {
		t.Errorf("Expected at most 3 backup files (MaxFiles=3), got %d", len(backupFiles))
	}

	// 检查总文件数量（包括当前文件）
	// 总文件数量应该是备份文件数量 + 1（当前文件）
	if len(allFiles) > len(backupFiles)+1 {
		t.Errorf("Total files count mismatch. Expected at most %d (backup files) + 1 (current file) = %d, got %d",
			len(backupFiles), len(backupFiles)+1, len(allFiles))
	}
}

// TestLogRotateX_ConcurrentWrite 测试并发写入安全性
func TestLogRotateX_ConcurrentWrite(t *testing.T) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("TestLogRotateX_ConcurrentWrite", t)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(logFile(dir))
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 启动多个goroutine并发写入
	const numGoroutines = 10
	const numWrites = 100
	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numWrites; j++ {
				var sb strings.Builder
				sb.WriteString("Goroutine ")
				sb.WriteString(fmt.Sprintf("%d", id))
				sb.WriteString(", Write ")
				sb.WriteString(fmt.Sprintf("%d", j))
				sb.WriteString("\n")
				_, err := logger.Write([]byte(sb.String()))
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

	// 验证文件存在
	if _, err := os.Stat(logFile(dir)); os.IsNotExist(err) {
		t.Errorf("Log file does not exist after concurrent writes")
	}
}

// TestLogRotateX_Sync 测试Sync功能
func TestLogRotateX_Sync(t *testing.T) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("TestLogRotateX_Sync", t)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(logFile(dir))
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// 写入数据
	_, err := logger.Write([]byte("Test data for sync\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// 调用Sync
	err = logger.Sync()
	if err != nil {
		t.Errorf("Sync failed: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(logFile(dir)); os.IsNotExist(err) {
		t.Errorf("Log file does not exist after sync")
	}
}

// TestLogRotateX_Close 测试Close功能
func TestLogRotateX_Close(t *testing.T) {
	originalCurrentTime := currentTime
	defer func() {
		currentTime = originalCurrentTime
	}()

	currentTime = func() time.Time {
		return time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dir := makeTempDir("TestLogRotateX_Close", t)
	defer func() { _ = os.RemoveAll(dir) }()

	logger := NewLogRotateX(logFile(dir))

	// 写入数据
	_, err := logger.Write([]byte("Test data before close\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// 关闭logger
	err = logger.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// 验证关闭后写入失败
	_, err = logger.Write([]byte("Test data after close\n"))
	if err == nil {
		t.Errorf("Expected write to fail after close, but it succeeded")
	}

	// 验证多次关闭是安全的
	err = logger.Close()
	if err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}
