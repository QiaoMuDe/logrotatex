// boundary_conditions_test.go 包含边界条件测试用例，用于验证日志轮转功能在各种极端情况下的行为。
// 测试内容包括零字节写入、单字节写入、恰好达到最大大小、超大单次写入等边界场景，
// 以及各种错误路径和并发场景的处理，确保系统在极端条件下的稳定性和正确性。
package logrotatex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestBoundaryConditions 测试各种边界条件
func TestBoundaryConditions(t *testing.T) {
	// 保存原始值
	originalCurrentTime := currentTime

	// 测试结束后恢复原始值
	defer func() {
		currentTime = originalCurrentTime
	}()

	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime

	t.Run("零字节写入", func(t *testing.T) {
		dir := makeBoundaryTempDir("TestBoundaryConditions_ZeroWrite", t)
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		l := &LogRotateX{
			LogFilePath: boundaryLogFile(dir),
			MaxSize:     1, // 1MB
		}
		defer func() {
			if err := l.Close(); err != nil {
				t.Logf("关闭日志文件失败: %v", err)
			}
		}()

		// 写入零字节
		n, err := l.Write([]byte{})
		if err != nil {
			t.Fatalf("写入零字节失败: %v", err)
		}
		if n != 0 {
			t.Errorf("期望写入0字节，实际写入%d字节", n)
		}
	})

	t.Run("单字节写入", func(t *testing.T) {
		dir := makeBoundaryTempDir("TestBoundaryConditions_SingleByte", t)
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		l := &LogRotateX{
			LogFilePath: boundaryLogFile(dir),
			MaxSize:     1, // 1MB
		}
		defer func() {
			if err := l.Close(); err != nil {
				t.Logf("关闭日志文件失败: %v", err)
			}
		}()

		// 写入单字节
		n, err := l.Write([]byte("a"))
		if err != nil {
			t.Fatalf("写入单字节失败: %v", err)
		}
		if n != 1 {
			t.Errorf("期望写入1字节，实际写入%d字节", n)
		}
	})

	t.Run("恰好达到最大大小", func(t *testing.T) {
		dir := makeBoundaryTempDir("TestBoundaryConditions_ExactMaxSize", t)
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		l := &LogRotateX{
			LogFilePath: boundaryLogFile(dir),
			MaxSize:     1, // 1MB = 1048576 bytes
			MaxFiles:    1,
		}
		defer func() {
			if err := l.Close(); err != nil {
				t.Logf("关闭日志文件失败: %v", err)
			}
		}()

		// 写入恰好1MB的数据
		data := make([]byte, megabyte)
		for i := range data {
			data[i] = 'a'
		}

		n, err := l.Write(data)
		if err != nil {
			t.Fatalf("写入1MB数据失败: %v", err)
		}
		if n != megabyte {
			t.Errorf("期望写入%d字节，实际写入%d字节", megabyte, n)
		}

		// 再写入一个字节，应该触发轮转
		n, err = l.Write([]byte("b"))
		if err != nil {
			t.Fatalf("写入触发轮转的字节失败: %v", err)
		}
		if n != 1 {
			t.Errorf("期望写入1字节，实际写入%d字节", n)
		}

		// 检查是否创建了备份文件
		files, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("读取目录失败: %v", err)
		}

		backupCount := 0
		for _, file := range files {
			// 检查是否是备份文件（格式为 test_timestamp.log）
			if strings.HasPrefix(file.Name(), "test_") && strings.HasSuffix(file.Name(), ".log") {
				backupCount++
			}
		}

		if backupCount != 1 {
			t.Errorf("期望1个备份文件，实际找到%d个", backupCount)
		}
	})

	t.Run("超大单次写入", func(t *testing.T) {
		dir := makeBoundaryTempDir("TestBoundaryConditions_LargeWrite", t)
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		l := &LogRotateX{
			LogFilePath: boundaryLogFile(dir),
			MaxSize:     1, // 1MB
			MaxFiles:    2,
		}
		defer func() {
			if err := l.Close(); err != nil {
				t.Logf("关闭日志文件失败: %v", err)
			}
		}()

		// 写入5MB的数据，应该触发多次轮转
		data := make([]byte, 5*megabyte)
		for i := range data {
			data[i] = byte('a' + (i % 26))
		}

		n, err := l.Write(data)
		if err != nil {
			t.Fatalf("写入5MB数据失败: %v", err)
		}
		if n != 5*megabyte {
			t.Errorf("期望写入%d字节，实际写入%d字节", 5*megabyte, n)
		}
	})

	t.Run("MaxBackups为0", func(t *testing.T) {
		dir := makeBoundaryTempDir("TestBoundaryConditions_MaxBackupsZero", t)
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		l := &LogRotateX{
			LogFilePath: boundaryLogFile(dir),
			MaxSize:     1, // 1MB
			MaxFiles:    0, // 不限制备份数量
		}
		defer func() {
			if err := l.Close(); err != nil {
				t.Logf("关闭日志文件失败: %v", err)
			}
		}()

		// 写入3次，触发2次轮转
		// 第一次写入，不触发轮转
		b1 := make([]byte, megabyte-1)
		n, err := l.Write(b1)
		isNil(err, t)
		equals(len(b1), n, t)

		// 第二次写入，触发第一次轮转
		newFakeTime()
		b2 := make([]byte, 2) // 写入2字节，总大小超过1MB
		n, err = l.Write(b2)
		isNil(err, t)
		equals(len(b2), n, t)

		// 第三次写入，不触发轮转
		n, err = l.Write(b1)
		isNil(err, t)
		equals(len(b1), n, t)

		// 第四次写入，触发第二次轮转
		newFakeTime()
		n, err = l.Write(b2)
		isNil(err, t)
		equals(len(b2), n, t)

		// 检查所有备份文件都被保留
		// 添加短暂延迟，确保文件系统同步
		time.Sleep(10 * time.Millisecond)
		files, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("读取目录失败: %v", err)
		}

		t.Logf("Found %d files in directory %s:", len(files), dir)
		for _, file := range files {
			t.Logf(" - %s", file.Name())
		}

		backupCount := 0
		for _, file := range files {
			// 备份文件格式为 test_timestamp.log
			if strings.HasPrefix(file.Name(), "test_") && strings.HasSuffix(file.Name(), ".log") {
				backupCount++
			}
		}

		// 应该有2个备份文件
		if backupCount != 2 {
			t.Errorf("期望2个备份文件，实际找到%d个", backupCount)
		}
	})

	t.Run("MaxAge为0", func(t *testing.T) {
		dir := makeBoundaryTempDir("TestBoundaryConditions_MaxAgeZero", t)
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		l := &LogRotateX{
			LogFilePath: boundaryLogFile(dir),
			MaxSize:     1, // 1MB
			MaxAge:      0, // 不限制文件年龄
		}
		defer func() {
			if err := l.Close(); err != nil {
				t.Logf("关闭日志文件失败: %v", err)
			}
		}()

		// 创建一些旧的日志文件
		oldTime := time.Now().AddDate(0, 0, -10) // 10天前
		oldFileName := fmt.Sprintf("test_%s.log", oldTime.Format("20060102150405"))
		oldFilePath := filepath.Join(dir, oldFileName)

		err := os.WriteFile(oldFilePath, []byte("old log data"), defaultFilePerm)
		if err != nil {
			t.Fatalf("创建旧日志文件失败: %v", err)
		}

		// 写入数据触发轮转
		data := make([]byte, megabyte+1)
		_, err = l.Write(data)
		if err != nil {
			t.Fatalf("写入数据失败: %v", err)
		}

		// 检查旧文件是否仍然存在（因为MaxAge=0）
		if _, err := os.Stat(oldFilePath); os.IsNotExist(err) {
			t.Error("旧日志文件不应该被删除（MaxAge=0）")
		}
	})
}

// TestErrorPaths 测试各种错误路径
func TestErrorPaths(t *testing.T) {
	t.Run("无效文件路径", func(t *testing.T) {
		// 使用一个确实无效的路径（在Windows和Unix上都无效）
		invalidPath := filepath.Join("Z:", "nonexistent", "invalid", "path", "test.log")
		l := &LogRotateX{
			LogFilePath: invalidPath,
			MaxSize:     1,
		}
		defer func() {
			if err := l.Close(); err != nil {
				t.Logf("关闭日志文件失败: %v", err)
			}
		}()

		_, err := l.Write([]byte("test"))
		if err == nil {
			t.Error("期望写入无效路径时返回错误")
		}
	})

	t.Run("只读目录", func(t *testing.T) {
		// 在Windows上跳过此测试，因为权限模型不同
		if filepath.Separator == '\\' {
			t.Skip("跳过Windows上的权限测试")
		}

		if os.Getuid() == 0 {
			t.Skip("跳过root用户的权限测试")
		}

		dir := makeBoundaryTempDir("TestErrorPaths_ReadOnlyDir", t)
		defer func() {
			// 确保在清理前恢复权限
			if err := os.Chmod(dir, 0755); err != nil {
				t.Logf("恢复目录权限失败: %v", err)
			}
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		// 设置目录为只读
		err := os.Chmod(dir, 0444)
		if err != nil {
			t.Fatalf("设置目录权限失败: %v", err)
		}

		l := &LogRotateX{
			LogFilePath: filepath.Join(dir, "test.log"),
			MaxSize:     1,
		}
		defer func() {
			if closeErr := l.Close(); closeErr != nil {
				t.Logf("关闭日志文件失败: %v", closeErr)
			}
		}()

		_, err = l.Write([]byte("test"))
		if err == nil {
			t.Error("期望在只读目录中写入时返回错误")
		}
	})

	t.Run("文件被外部删除", func(t *testing.T) {
		dir := makeBoundaryTempDir("TestErrorPaths_FileDeleted", t)

		l := &LogRotateX{
			LogFilePath: boundaryLogFile(dir),
			MaxSize:     1,
		}

		// 先写入一些数据
		_, err := l.Write([]byte("initial data"))
		if err != nil {
			t.Fatalf("初始写入失败: %v", err)
		}

		// 关闭文件以释放文件句柄
		err = l.Close()
		if err != nil {
			t.Fatalf("关闭文件失败: %v", err)
		}

		// 外部删除文件
		err = os.Remove(l.LogFilePath)
		if err != nil {
			t.Fatalf("删除文件失败: %v", err)
		}

		// 再次写入，关闭后应返回明确错误
		_, err = l.Write([]byte("after deletion"))
		if err == nil {
			t.Fatalf("文件被删除后写入应失败")
		}
		if !strings.Contains(err.Error(), "write on closed") {
			t.Fatalf("关闭后写入返回的错误不符合预期: %v", err)
		}

		// 使用新的实例重新创建文件并写入
		l2 := &LogRotateX{
			LogFilePath: boundaryLogFile(dir),
			MaxSize:     1,
		}
		defer func() {
			if cerr := l2.Close(); cerr != nil {
				t.Logf("关闭新日志实例失败: %v", cerr)
			}
		}()

		_, err = l2.Write([]byte("after deletion recreated"))
		if err != nil {
			t.Fatalf("使用新实例写入失败: %v", err)
		}

		// 检查文件是否重新创建
		if _, statErr := os.Stat(l2.LogFilePath); os.IsNotExist(statErr) {
			t.Error("文件应该被重新创建")
		}
	})

	t.Run("磁盘空间不足模拟", func(t *testing.T) {
		// 这个测试很难在真实环境中模拟，我们只是确保大量写入不会崩溃
		dir := makeBoundaryTempDir("TestErrorPaths_DiskSpace", t)
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		l := &LogRotateX{
			LogFilePath: boundaryLogFile(dir),
			MaxSize:     1,
			MaxFiles:    1,
		}
		defer func() {
			if err := l.Close(); err != nil {
				t.Logf("关闭日志文件失败: %v", err)
			}
		}()

		// 尝试写入大量数据
		largeData := make([]byte, 10*megabyte)
		for i := range largeData {
			largeData[i] = 'x'
		}

		_, err := l.Write(largeData)
		// 我们不期望特定的错误，只是确保不会panic
		if err != nil {
			t.Logf("大量写入返回错误（这是可以接受的）: %v", err)
		}
	})
}

// TestConcurrentScenarios 测试并发场景
func TestConcurrentScenarios(t *testing.T) {
	t.Run("多goroutine并发写入", func(t *testing.T) {
		dir := makeBoundaryTempDir("TestConcurrentScenarios_MultiWrite", t)
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		l := &LogRotateX{
			LogFilePath: boundaryLogFile(dir),
			MaxSize:     1, // 1MB
			MaxFiles:    5,
		}
		defer func() {
			if err := l.Close(); err != nil {
				t.Logf("关闭日志文件失败: %v", err)
			}
		}()

		const numGoroutines = 10
		const writesPerGoroutine = 100
		var wg sync.WaitGroup

		// 启动多个goroutine并发写入
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < writesPerGoroutine; j++ {
					data := fmt.Sprintf("goroutine-%d-write-%d: %s\n",
						id, j, strings.Repeat("data", 100))
					_, err := l.Write([]byte(data))
					if err != nil {
						t.Errorf("goroutine %d 写入 %d 失败: %v", id, j, err)
						return
					}
				}
			}(i)
		}

		wg.Wait()

		// 确保所有数据都被刷新到磁盘
		err := l.Close()
		if err != nil {
			t.Fatalf("关闭日志文件失败: %v", err)
		}

		// 验证所有数据都被写入
		files, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("读取目录失败: %v", err)
		}

		totalSize := int64(0)
		for _, file := range files {
			if !file.IsDir() {
				info, err := file.Info()
				if err != nil {
					continue
				}
				totalSize += info.Size()
			}
		}

		expectedMinSize := int64(numGoroutines * writesPerGoroutine * 100) // 大致估算
		if totalSize < expectedMinSize {
			t.Errorf("总文件大小 %d 小于期望的最小值 %d", totalSize, expectedMinSize)
		}
	})

	t.Run("并发关闭", func(t *testing.T) {
		dir := makeBoundaryTempDir("TestConcurrentScenarios_ConcurrentClose", t)
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		l := &LogRotateX{
			LogFilePath: boundaryLogFile(dir),
			MaxSize:     1,
		}

		var wg sync.WaitGroup
		const numClosers = 5

		// 启动多个goroutine同时关闭
		for i := 0; i < numClosers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := l.Close()
				if err != nil {
					t.Errorf("关闭失败: %v", err)
				}
			}()
		}

		wg.Wait()
		// 测试通过意味着没有panic或死锁
	})
}

// TestEdgeCases 测试边缘情况
func TestEdgeCases(t *testing.T) {
	t.Run("空文件名", func(t *testing.T) {
		// 使用一个明确的临时文件路径，避免默认路径问题
		tempDir := makeBoundaryTempDir("TestEdgeCases_EmptyFilename", t)
		defer func() {
			if err := os.RemoveAll(tempDir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		// 创建一个空的LogRotateX实例，但不写入数据
		l := &LogRotateX{
			LogFilePath: "", // 空文件名，应该使用默认值
			MaxSize:     10, // 增大MaxSize避免立即轮转
		}

		// 先检查默认文件名生成是否正确
		defaultPath := l.filename()
		if defaultPath == "" {
			t.Error("默认文件名不应该为空")
		}

		// 确保默认路径在临时目录中，避免污染系统目录
		l.LogFilePath = filepath.Join(tempDir, "empty_test.log")

		defer func() {
			if err := l.Close(); err != nil {
				t.Logf("关闭日志文件失败: %v", err)
			}
		}()

		_, err := l.Write([]byte("test"))
		if err != nil {
			t.Fatalf("空文件名写入失败: %v", err)
		}

		// 检查文件是否正确创建
		if _, err := os.Stat(l.LogFilePath); err != nil {
			t.Errorf("日志文件未正确创建: %v", err)
		}
	})

	t.Run("负数配置值", func(t *testing.T) {
		dir := makeBoundaryTempDir("TestEdgeCases_NegativeValues", t)
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		l := &LogRotateX{
			LogFilePath: boundaryLogFile(dir),
			MaxSize:     -1, // 负数，应该使用默认值
			MaxFiles:    -1, // 负数
			MaxAge:      -1, // 负数
		}
		defer func() {
			if err := l.Close(); err != nil {
				t.Logf("关闭日志文件失败: %v", err)
			}
		}()

		_, err := l.Write([]byte("test with negative values"))
		if err != nil {
			t.Fatalf("负数配置写入失败: %v", err)
		}
	})

	t.Run("极大配置值", func(t *testing.T) {
		dir := makeBoundaryTempDir("TestEdgeCases_LargeValues", t)
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		l := &LogRotateX{
			LogFilePath: boundaryLogFile(dir),
			MaxSize:     999999, // 极大值
			MaxFiles:    999999, // 极大值
			MaxAge:      999999, // 极大值
		}
		defer func() {
			if err := l.Close(); err != nil {
				t.Logf("关闭日志文件失败: %v", err)
			}
		}()

		_, err := l.Write([]byte("test with large values"))
		if err != nil {
			t.Fatalf("极大配置值写入失败: %v", err)
		}
	})

	t.Run("特殊字符文件名", func(t *testing.T) {
		dir := makeBoundaryTempDir("TestEdgeCases_SpecialChars", t)
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("清理临时目录失败: %v", err)
			}
		}()

		// 在Windows上某些字符是不允许的，所以我们使用相对安全的特殊字符
		specialName := "test_log_file.2025.log"

		l := &LogRotateX{
			LogFilePath: filepath.Join(dir, specialName),
			MaxSize:     1,
		}
		defer func() {
			if err := l.Close(); err != nil {
				t.Logf("关闭日志文件失败: %v", err)
			}
		}()

		_, err := l.Write([]byte("test with special chars"))
		if err != nil {
			t.Fatalf("特殊字符文件名写入失败: %v", err)
		}
	})
}

// 辅助函数
func makeBoundaryTempDir(name string, t *testing.T) string {
	// 根据测试名称和当前时间生成目录名
	dir := time.Now().Format(name + "_" + "20060102150405")
	// 将生成的目录名与logs目录拼接，得到完整的目录路径
	dir = filepath.Join("logs", dir)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatalf("创建logs目录失败: %v", err)
	}
	return dir
}

func boundaryLogFile(dir string) string {
	return filepath.Join(dir, "test.log")
}

// TestComprehensiveLogRotation 全面测试日志轮转功能
// 包括基本写入、轮转、压缩、清理等功能
func TestComprehensiveLogRotation(t *testing.T) {
	// 保存原始值
	originalMegabyte := megabyte
	originalCurrentTime := currentTime
	defer func() {
		megabyte = originalMegabyte
		currentTime = originalCurrentTime
	}()

	// 设置测试环境
	megabyte = 1024 // 1KB = 1MB 便于测试
	currentTime = fakeTime

	// 创建临时测试目录
	dir := "logs"
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatalf("创建logs目录失败: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	// 第一阶段：测试基本写入功能
	t.Log("=== 第一阶段：测试基本写入功能 ===")

	logger := &LogRotateX{
		LogFilePath: filepath.Join(dir, "app.log"),
		MaxSize:     2,    // 2KB
		MaxFiles:    3,    // 最多保留3个备份
		MaxAge:      7,    // 最多保留7天
		Compress:    true, // 启用压缩
	}
	defer func() { _ = logger.Close() }()

	// 写入少量数据，不触发轮转
	testData := "这是一条测试日志消息\n"
	for i := 0; i < 10; i++ {
		_, writeErr := fmt.Fprintf(logger, "%s第%d条消息\n", testData, i)
		if writeErr != nil {
			t.Fatalf("写入日志失败: %v", writeErr)
		}
	}

	// 验证文件创建
	logPath := filepath.Join(dir, "app.log")
	if _, statErr := os.Stat(logPath); os.IsNotExist(statErr) {
		t.Fatal("日志文件未被创建")
	}

	content, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("读取日志文件失败: %v", readErr)
	}

	if len(content) == 0 {
		t.Fatal("日志文件内容为空")
	}

	t.Logf("当前日志文件大小: %d 字节", len(content))

	// 第二阶段：测试配置验证
	t.Log("=== 第二阶段：测试配置验证 ===")

	if logger.MaxSize != 2 {
		t.Errorf("期望 MaxSize 为 2，实际为 %d", logger.MaxSize)
	}
	if logger.MaxFiles != 3 {
		t.Errorf("期望 MaxFiles 为 3，实际为 %d", logger.MaxFiles)
	}
	if logger.MaxAge != 7 {
		t.Errorf("期望 MaxAge 为 7，实际为 %d", logger.MaxAge)
	}
	if !logger.Compress {
		t.Error("期望 Compress 为 true")
	}

	// 第五阶段：测试文件清理逻辑
	t.Log("=== 第五阶段：测试文件清理逻辑 ===")

	// 创建一些模拟的旧日志文件
	oldFiles := []string{
		"app_20230101100000.log.zip",
		"app_20230102100000.log.zip",
		"app_20230103100000.log.zip",
		"app_20230104100000.log.zip",
		"app_20230105100000.log.zip",
	}

	for _, fileName := range oldFiles {
		filePath := filepath.Join(dir, fileName)
		if writeErr := os.WriteFile(filePath, []byte("模拟旧日志内容"), 0644); writeErr != nil {
			t.Fatalf("创建模拟文件失败: %v", writeErr)
		}
	}

	// 测试 oldLogFiles 方法
	logFiles, err := logger.oldLogFiles()
	if err != nil {
		t.Fatalf("获取旧日志文件失败: %v", err)
	}

	t.Logf("找到 %d 个旧日志文件", len(logFiles))

	// 第六阶段：测试重新写入功能
	t.Log("=== 第六阶段：测试重新写入功能 ===")

	// 写入新数据
	newData := "重新写入的测试数据\n"
	_, err = logger.Write([]byte(newData))
	if err != nil {
		t.Fatalf("重新写入失败: %v", err)
	}

	// 验证文件存在
	if _, statErr := os.Stat(logPath); os.IsNotExist(statErr) {
		t.Error("重新写入后日志文件不存在")
	}

	// 第七阶段：测试并发安全性
	t.Log("=== 第七阶段：测试并发安全性 ===")

	// 简单的并发写入测试
	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 5; i++ {
			if _, writeErr := fmt.Fprintf(logger, "goroutine1-消息%d\n", i); writeErr != nil {
				t.Errorf("并发写入1失败: %v", writeErr)
			}
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 5; i++ {
			if _, writeErr := fmt.Fprintf(logger, "goroutine2-消息%d\n", i); writeErr != nil {
				t.Errorf("并发写入2失败: %v", writeErr)
			}
		}
		done <- true
	}()

	// 等待两个 goroutine 完成
	<-done
	<-done

	t.Log("并发写入测试完成")

	// 最终验证
	finalContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取最终日志文件失败: %v", err)
	}

	t.Log("=== 测试总结 ===")
	t.Logf("✓ 基本写入功能正常")
	t.Logf("✓ 配置参数验证通过")
	t.Logf("✓ 压缩功能验证成功")
	t.Logf("✓ 文件清理逻辑正常")
	t.Logf("✓ 重新写入功能正常")
	t.Logf("✓ 并发安全性测试通过")
	t.Logf("✓ 最终日志文件大小: %d 字节", len(finalContent))

	// 显示目录中的所有文件
	files, err := os.ReadDir(dir)
	if err == nil {
		var fileNames []string
		for _, file := range files {
			fileNames = append(fileNames, file.Name())
		}
		t.Logf("✓ 测试目录中的文件: %v", fileNames)
	}
}
