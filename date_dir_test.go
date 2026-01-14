// date_dir_test.go 包含基于天数存储日志轮转的测试用例
package logrotatex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// dateDirLogFile 返回日志文件路径
func dateDirLogFile(dir string) string {
	return filepath.Join(dir, "datedir.log")
}

// TestDateDirLayout_Disabled 测试禁用日期目录模式（默认行为）
func TestDateDirLayout_Disabled(t *testing.T) {
	dir := makeBoundaryTempDir("TestDateDirLayout_Disabled", t)
	defer func() { _ = os.RemoveAll(dir) }()

	l := &LogRotateX{
		LogFilePath:   dateDirLogFile(dir),
		MaxSize:       1,     // 1MB
		MaxFiles:      0,     // 不限制文件数量（避免清理干扰测试）
		DateDirLayout: false, // 禁用日期目录
	}
	defer func() { _ = l.Close() }()

	// 写入数据触发轮转
	// 每次写入超过1MB，确保触发轮转
	data := make([]byte, 1024*1024+100) // 1MB + 100 bytes
	for i := range data {
		data[i] = 'a'
	}

	for i := 0; i < 3; i++ {
		t.Logf("第 %d 次写入 %d 字节数据", i+1, len(data))
		n, err := l.Write(data)
		if err != nil {
			t.Fatalf("写入数据失败: %v", err)
		}
		t.Logf("  实际写入 %d 字节", n)
		time.Sleep(1 * time.Second) // 确保时间戳不同（增加到1秒）
	}

	// 检查文件结构
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}

	t.Logf("找到 %d 个文件/目录:", len(files))
	for _, file := range files {
		if !file.IsDir() {
			info, _ := file.Info()
			t.Logf("  - %s (大小: %d 字节)", file.Name(), info.Size())
		} else {
			t.Logf("  - %s (目录)", file.Name())
		}
	}

	// 列出所有备份文件
	t.Logf("所有备份文件:")
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "datedir_") {
			info, _ := file.Info()
			t.Logf("  - %s (大小: %d 字节)", file.Name(), info.Size())
		}
	}

	// 应该只有根目录下的文件，没有日期目录
	dateDirCount := 0
	logFileCount := 0
	for _, file := range files {
		if file.IsDir() {
			dateDirCount++
		} else if strings.HasPrefix(file.Name(), "datedir_") {
			logFileCount++
		}
	}

	if dateDirCount != 0 {
		t.Errorf("期望0个日期目录，实际找到%d个", dateDirCount)
	}

	if logFileCount != 3 {
		t.Errorf("期望3个备份日志文件，实际找到%d个", logFileCount)
	}

	t.Logf("✅ 禁用日期目录模式测试通过")
}

// TestDateDirLayout_Enabled 测试启用日期目录模式
func TestDateDirLayout_Enabled(t *testing.T) {
	dir := makeBoundaryTempDir("TestDateDirLayout_Enabled", t)
	defer func() { _ = os.RemoveAll(dir) }()

	l := &LogRotateX{
		LogFilePath:   dateDirLogFile(dir),
		MaxSize:       1,    // 1MB
		MaxFiles:      0,    // 不限制文件数量（避免清理干扰测试）
		DateDirLayout: true, // 启用日期目录
	}
	defer func() { _ = l.Close() }()

	// 写入数据触发轮转
	// 每次写入超过1MB，确保触发轮转
	data := make([]byte, 1024*1024+100) // 1MB + 100 bytes
	for i := range data {
		data[i] = 'a'
	}

	for i := 0; i < 3; i++ {
		_, err := l.Write(data)
		if err != nil {
			t.Fatalf("写入数据失败: %v", err)
		}
		time.Sleep(1 * time.Second) // 确保时间戳不同（增加到1秒）
	}

	// 检查文件结构
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}

	// 应该有日期目录
	dateDirCount := 0
	dateDirName := ""
	for _, file := range files {
		if file.IsDir() {
			dateDirCount++
			dateDirName = file.Name()
		}
	}

	if dateDirCount != 1 {
		t.Errorf("期望1个日期目录，实际找到%d个", dateDirCount)
	}

	// 检查日期目录格式（YYYY-MM-DD）
	if dateDirName != "" {
		if len(dateDirName) != 10 || dateDirName[4] != '-' || dateDirName[7] != '-' {
			t.Errorf("日期目录格式错误: %s", dateDirName)
		}
	}

	// 检查日期目录中的日志文件
	dateDirPath := filepath.Join(dir, dateDirName)
	dateDirFiles, err := os.ReadDir(dateDirPath)
	if err != nil {
		t.Fatalf("读取日期目录失败: %v", err)
	}

	logFileCount := 0
	for _, file := range dateDirFiles {
		if strings.HasPrefix(file.Name(), "datedir_") {
			logFileCount++
		}
	}

	if logFileCount != 3 {
		t.Errorf("期望3个备份日志文件，实际找到%d个", logFileCount)
	}

	t.Logf("✅ 启用日期目录模式测试通过")
}

// TestDateDirLayout_MixedMode 测试混合模式（既有根目录文件，也有日期目录文件）
func TestDateDirLayout_MixedMode(t *testing.T) {
	dir := makeBoundaryTempDir("TestDateDirLayout_MixedMode", t)
	defer func() { _ = os.RemoveAll(dir) }()

	// 手动创建一些根目录的日志文件（模拟旧文件）
	now := time.Now()
	for i := 0; i < 2; i++ {
		timestamp := now.Add(time.Duration(-i) * time.Hour)
		fileName := fmt.Sprintf("datedir_%s.log", timestamp.Format("20060102150405"))
		filePath := filepath.Join(dir, fileName)
		err := os.WriteFile(filePath, []byte("old log"), 0644)
		if err != nil {
			t.Fatalf("创建旧日志文件失败: %v", err)
		}
	}

	// 创建一个日期目录并放入日志文件
	dateDir := now.Format("2006-01-02")
	dateDirPath := filepath.Join(dir, dateDir)
	err := os.Mkdir(dateDirPath, 0755)
	if err != nil {
		t.Fatalf("创建日期目录失败: %v", err)
	}

	for i := 0; i < 2; i++ {
		timestamp := now.Add(time.Duration(-i-2) * time.Hour)
		fileName := fmt.Sprintf("datedir_%s.log", timestamp.Format("20060102150405"))
		filePath := filepath.Join(dateDirPath, fileName)
		err := os.WriteFile(filePath, []byte("new log"), 0644)
		if err != nil {
			t.Fatalf("创建日期目录日志文件失败: %v", err)
		}
	}

	// 创建 LogRotateX 实例并启用日期目录
	l := &LogRotateX{
		LogFilePath:   dateDirLogFile(dir),
		MaxSize:       1,
		MaxFiles:      0, // 不限制文件数量（避免清理干扰测试）
		DateDirLayout: true,
	}
	defer func() { _ = l.Close() }()

	// 触发一次轮转
	data := make([]byte, 1024*1024+100) // 1MB + 100 bytes
	for i := range data {
		data[i] = 'a'
	}
	_, err = l.Write(data)
	if err != nil {
		t.Fatalf("写入数据失败: %v", err)
	}

	// 检查是否正确扫描了所有文件（根目录 + 日期目录）
	files, err := l.oldLogFiles()
	if err != nil {
		t.Fatalf("获取旧日志文件失败: %v", err)
	}

	// 应该找到 5 个文件（2个根目录 + 2个日期目录 + 1个新轮转的）
	if len(files) != 5 {
		t.Errorf("期望5个日志文件，实际找到%d个", len(files))
	}

	t.Logf("✅ 混合模式测试通过")
}

// TestDateDirLayout_Cleanup 测试日期目录的清理功能
func TestDateDirLayout_Cleanup(t *testing.T) {
	dir := makeBoundaryTempDir("TestDateDirLayout_Cleanup", t)
	defer func() { _ = os.RemoveAll(dir) }()

	l := &LogRotateX{
		LogFilePath:   dateDirLogFile(dir),
		MaxSize:       1,
		MaxFiles:      3, // 只保留3个文件
		DateDirLayout: true,
	}
	defer func() { _ = l.Close() }()

	// 创建多个日期目录和日志文件
	now := time.Now()
	for i := 0; i < 5; i++ {
		dateDir := now.Add(time.Duration(-i) * 24 * time.Hour).Format("2006-01-02")
		dateDirPath := filepath.Join(dir, dateDir)
		err := os.Mkdir(dateDirPath, 0755)
		if err != nil {
			t.Fatalf("创建日期目录失败: %v", err)
		}

		timestamp := now.Add(time.Duration(-i) * 24 * time.Hour)
		fileName := fmt.Sprintf("datedir_%s.log", timestamp.Format("20060102150405"))
		filePath := filepath.Join(dateDirPath, fileName)
		err = os.WriteFile(filePath, []byte("log data"), 0644)
		if err != nil {
			t.Fatalf("创建日志文件失败: %v", err)
		}
	}

	// 触发清理
	data := make([]byte, 1024*1024+100) // 1MB + 100 bytes
	for i := range data {
		data[i] = 'a'
	}
	_, err := l.Write(data)
	if err != nil {
		t.Fatalf("写入数据失败: %v", err)
	}

	// 等待异步清理完成
	time.Sleep(100 * time.Millisecond)

	// 检查是否只保留了3个文件
	files, err := l.oldLogFiles()
	if err != nil {
		t.Fatalf("获取旧日志文件失败: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("期望3个日志文件，实际找到%d个", len(files))
	}

	t.Logf("✅ 日期目录清理功能测试通过")
}

// TestDateDirLayout_EmptyDirCleanup 测试空日期目录的清理
func TestDateDirLayout_EmptyDirCleanup(t *testing.T) {
	dir := makeBoundaryTempDir("TestDateDirLayout_EmptyDirCleanup", t)
	defer func() { _ = os.RemoveAll(dir) }()

	l := &LogRotateX{
		LogFilePath:   dateDirLogFile(dir),
		MaxSize:       1,
		MaxFiles:      1, // 只保留1个文件
		DateDirLayout: true,
	}
	defer func() { _ = l.Close() }()

	// 创建多个日期目录，每个目录只有一个日志文件
	now := time.Now()
	for i := 0; i < 3; i++ {
		dateDir := now.Add(time.Duration(-i) * 24 * time.Hour).Format("2006-01-02")
		dateDirPath := filepath.Join(dir, dateDir)
		err := os.Mkdir(dateDirPath, 0755)
		if err != nil {
			t.Fatalf("创建日期目录失败: %v", err)
		}

		timestamp := now.Add(time.Duration(-i) * 24 * time.Hour)
		fileName := fmt.Sprintf("datedir_%s.log", timestamp.Format("20060102150405"))
		filePath := filepath.Join(dateDirPath, fileName)
		err = os.WriteFile(filePath, []byte("log data"), 0644)
		if err != nil {
			t.Fatalf("创建日志文件失败: %v", err)
		}
	}

	// 触发清理，应该删除所有旧文件，只保留最新的1个
	data := make([]byte, 1024*1024+100) // 1MB + 100 bytes
	for i := range data {
		data[i] = 'a'
	}
	_, err := l.Write(data)
	if err != nil {
		t.Fatalf("写入数据失败: %v", err)
	}

	// 等待异步清理完成
	time.Sleep(100 * time.Millisecond)

	// 检查是否只保留了1个文件
	files, err := l.oldLogFiles()
	if err != nil {
		t.Fatalf("获取旧日志文件失败: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("期望1个日志文件，实际找到%d个", len(files))
	}

	// 检查空的日期目录是否被清理
	dateDirs, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}

	emptyDirCount := 0
	for _, item := range dateDirs {
		if item.IsDir() {
			dirPath := filepath.Join(dir, item.Name())
			dirFiles, err := os.ReadDir(dirPath)
			if err != nil {
				continue
			}
			if len(dirFiles) == 0 {
				emptyDirCount++
			}
		}
	}

	if emptyDirCount != 0 {
		t.Errorf("期望0个空日期目录，实际找到%d个", emptyDirCount)
	}

	t.Logf("✅ 空日期目录清理测试通过")
}

// TestDateDirLayout_MultipleDateDirs 测试多个日期目录的扫描和清理
func TestDateDirLayout_MultipleDateDirs(t *testing.T) {
	dir := makeBoundaryTempDir("TestDateDirLayout_MultipleDateDirs", t)
	defer func() { _ = os.RemoveAll(dir) }()

	l := &LogRotateX{
		LogFilePath:   dateDirLogFile(dir),
		MaxSize:       1,
		MaxFiles:      0, // 不限制文件数量（避免清理干扰测试）
		DateDirLayout: true,
	}
	defer func() { _ = l.Close() }()

	// 创建多个日期目录，每个目录有多个日志文件
	now := time.Now()
	for i := 0; i < 5; i++ {
		dateDir := now.Add(time.Duration(-i) * 24 * time.Hour).Format("2006-01-02")
		dateDirPath := filepath.Join(dir, dateDir)
		err := os.Mkdir(dateDirPath, 0755)
		if err != nil {
			t.Fatalf("创建日期目录失败: %v", err)
		}

		// 每个日期目录创建2个日志文件
		for j := 0; j < 2; j++ {
			// 确保时间戳不同：使用不同的秒数
			timestamp := now.Add(time.Duration(-i*24*3600-j*60) * time.Second)
			fileName := fmt.Sprintf("datedir_%s.log", timestamp.Format("20060102150405"))
			filePath := filepath.Join(dateDirPath, fileName)
			err = os.WriteFile(filePath, []byte("log data"), 0644)
			if err != nil {
				t.Fatalf("创建日志文件失败: %v", err)
			}
		}
	}

	// 触发扫描
	data := make([]byte, 1024*1024+100) // 1MB + 100 bytes
	for i := range data {
		data[i] = 'a'
	}
	_, err := l.Write(data)
	if err != nil {
		t.Fatalf("写入数据失败: %v", err)
	}

	// 检查是否正确扫描了所有文件
	files, err := l.oldLogFiles()
	if err != nil {
		t.Fatalf("获取旧日志文件失败: %v", err)
	}

	// 应该找到 11 个文件（5个日期目录 × 2个文件 + 1个新轮转的）
	if len(files) != 11 {
		t.Errorf("期望11个日志文件，实际找到%d个", len(files))
	}

	// 检查文件是否按时间戳正确排序
	for i := 1; i < len(files); i++ {
		if files[i-1].timestamp.Before(files[i].timestamp) {
			t.Errorf("文件未按时间戳从新到旧排序")
		}
	}

	t.Logf("✅ 多个日期目录扫描测试通过")
}

// TestDateDirLayout_DateDirFormat 测试日期目录格式
func TestDateDirLayout_DateDirFormat(t *testing.T) {
	dir := makeBoundaryTempDir("TestDateDirLayout_DateDirFormat", t)
	defer func() { _ = os.RemoveAll(dir) }()

	l := &LogRotateX{
		LogFilePath:   dateDirLogFile(dir),
		MaxSize:       1,
		MaxFiles:      5,
		DateDirLayout: true,
	}
	defer func() { _ = l.Close() }()

	// 写入数据触发轮转
	data := make([]byte, 1024*1024+100) // 1MB + 100 bytes
	for i := range data {
		data[i] = 'a'
	}
	_, err := l.Write(data)
	if err != nil {
		t.Fatalf("写入数据失败: %v", err)
	}

	// 检查日期目录格式
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}

	var dateDirName string
	for _, file := range files {
		if file.IsDir() {
			dateDirName = file.Name()
			break
		}
	}

	if dateDirName == "" {
		t.Fatal("未找到日期目录")
	}

	// 检查格式是否为 YYYY-MM-DD
	if len(dateDirName) != 10 {
		t.Errorf("日期目录长度错误: %s", dateDirName)
	}

	if dateDirName[4] != '-' || dateDirName[7] != '-' {
		t.Errorf("日期目录格式错误: %s", dateDirName)
	}

	// 尝试解析日期
	_, err = time.Parse("2006-01-02", dateDirName)
	if err != nil {
		t.Errorf("日期目录无法解析: %s, 错误: %v", dateDirName, err)
	}

	t.Logf("✅ 日期目录格式测试通过")
}
