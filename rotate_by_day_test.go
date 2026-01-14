// rotate_by_day_test.go 包含了按天轮转功能的测试用例。
// 该文件测试了 RotateByDay 配置选项的正确性，
// 确保按天轮转功能能够正常工作。

package logrotatex

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRotateByDay 测试按天轮转功能。
// 该测试会模拟时间跨天，验证日志文件是否按天轮转。
func TestRotateByDay(t *testing.T) {
	// 保存原始值
	originalMegabyte := megabyte
	originalCurrentTime := currentTime

	// 测试结束后恢复原始值
	defer func() {
		megabyte = originalMegabyte
		currentTime = originalCurrentTime
	}()

	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 设置 megabyte 变量的值为 1
	megabyte = 1

	// 创建临时目录用于测试
	dir := makeTempDir("TestRotateByDay", t)
	defer func() { _ = os.RemoveAll(dir) }()

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 LogRotateX 实例，启用按天轮转
	l := &LogRotateX{
		LogFilePath: filename,
		MaxSize:     100,  // 设置较大的 MaxSize，避免按大小轮转
		RotateByDay: true, // 启用按天轮转
	}
	// 测试结束后关闭日志文件
	defer func() { _ = l.Close() }()

	// 定义要写入日志文件的初始数据
	b := []byte("day1 data")
	// 尝试将初始数据写入日志文件
	n, err := l.Write(b)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与初始数据的长度一致
	equals(len(b), n, t)

	// 验证日志文件的内容是否为初始数据
	existsWithContent(filename, b, t)
	// 验证临时目录中文件数量是否为 1，即只存在一个日志文件
	fileCount(dir, 1, t)

	// 模拟时间跨天
	newFakeDay()

	// 写入新数据，这将触发按天轮转
	b2 := []byte("day2 data")
	// 尝试将新数据写入日志文件
	n, err = l.Write(b2)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与新数据的长度一致
	equals(len(b2), n, t)

	// 获取第一个备份文件的路径
	firstBackup := backupFile(dir)

	// 验证第一个备份文件的内容是否为初始数据
	existsWithContent(firstBackup, b, t)

	// 验证主日志文件的内容是否为新写入的数据
	existsWithContent(filename, b2, t)

	// 验证临时目录中文件数量是否为 2，即存在主日志文件和一个备份文件
	fileCount(dir, 2, t)

	t.Logf("✅ 按天轮转测试通过")
}

// TestRotateByDay_Disabled 测试禁用按天轮转功能。
// 该测试会模拟时间跨天，验证日志文件不会按天轮转。
func TestRotateByDay_Disabled(t *testing.T) {
	// 保存原始值
	originalMegabyte := megabyte
	originalCurrentTime := currentTime

	// 测试结束后恢复原始值
	defer func() {
		megabyte = originalMegabyte
		currentTime = originalCurrentTime
	}()

	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 设置 megabyte 变量的值为 1
	megabyte = 1

	// 创建临时目录用于测试
	dir := makeTempDir("TestRotateByDay_Disabled", t)
	defer func() { _ = os.RemoveAll(dir) }()

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 LogRotateX 实例，禁用按天轮转
	l := &LogRotateX{
		LogFilePath: filename,
		MaxSize:     100,   // 设置较大的 MaxSize，避免按大小轮转
		RotateByDay: false, // 禁用按天轮转
	}
	// 测试结束后关闭日志文件
	defer func() { _ = l.Close() }()

	// 定义要写入日志文件的初始数据
	b := []byte("day1 data")
	// 尝试将初始数据写入日志文件
	n, err := l.Write(b)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与初始数据的长度一致
	equals(len(b), n, t)

	// 验证日志文件的内容是否为初始数据
	existsWithContent(filename, b, t)
	// 验证临时目录中文件数量是否为 1，即只存在一个日志文件
	fileCount(dir, 1, t)

	// 模拟时间跨天
	newFakeDay()

	// 写入新数据，这不会触发按天轮转（因为禁用了）
	b2 := []byte("day2 data")
	// 尝试将新数据写入日志文件
	n, err = l.Write(b2)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与新数据的长度一致
	equals(len(b2), n, t)

	// 验证主日志文件的内容是否包含所有数据（没有轮转）
	existsWithContent(filename, append(b, b2...), t)

	// 验证临时目录中文件数量仍为 1，即只存在一个日志文件（没有备份文件）
	fileCount(dir, 1, t)

	t.Logf("✅ 禁用按天轮转测试通过")
}

// TestRotateByDay_MultipleDays 测试多天轮转。
// 该测试会模拟时间跨多天，验证日志文件是否每天轮转一次。
func TestRotateByDay_MultipleDays(t *testing.T) {
	// 保存原始值
	originalMegabyte := megabyte
	originalCurrentTime := currentTime

	// 测试结束后恢复原始值
	defer func() {
		megabyte = originalMegabyte
		currentTime = originalCurrentTime
	}()

	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 设置 megabyte 变量的值为 1
	megabyte = 1

	// 创建临时目录用于测试
	dir := makeTempDir("TestRotateByDay_MultipleDays", t)
	defer func() { _ = os.RemoveAll(dir) }()

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 LogRotateX 实例，启用按天轮转
	l := &LogRotateX{
		LogFilePath: filename,
		MaxSize:     100,  // 设置较大的 MaxSize，避免按大小轮转
		RotateByDay: true, // 启用按天轮转
	}
	// 测试结束后关闭日志文件
	defer func() { _ = l.Close() }()

	// 第一天：写入数据
	b1 := []byte("day1 data")
	n, err := l.Write(b1)
	isNil(err, t)
	equals(len(b1), n, t)
	existsWithContent(filename, b1, t)
	fileCount(dir, 1, t)

	// 模拟时间跨天
	newFakeDay()

	// 第二天：写入数据，触发轮转
	b2 := []byte("day2 data")
	n, err = l.Write(b2)
	isNil(err, t)
	equals(len(b2), n, t)

	// 验证第一天备份文件
	backup1 := backupFile(dir)
	existsWithContent(backup1, b1, t)
	existsWithContent(filename, b2, t)
	fileCount(dir, 2, t)

	// 模拟时间跨天
	newFakeDay()

	// 第三天：写入数据，再次触发轮转
	b3 := []byte("day3 data")
	n, err = l.Write(b3)
	isNil(err, t)
	equals(len(b3), n, t)

	// 验证第二天备份文件
	backup2 := backupFile(dir)
	existsWithContent(backup2, b2, t)
	existsWithContent(filename, b3, t)
	fileCount(dir, 3, t)

	// 验证第一天备份文件仍然存在
	existsWithContent(backup1, b1, t)

	t.Logf("✅ 多天轮转测试通过")
}

// TestRotateByDay_WithDateDirLayout 测试按天轮转与日期目录布局的配合。
// 该测试会模拟时间跨天，验证日志文件是否按天轮转并存放在日期目录中。
func TestRotateByDay_WithDateDirLayout(t *testing.T) {
	// 保存原始值
	originalMegabyte := megabyte
	originalCurrentTime := currentTime

	// 测试结束后恢复原始值
	defer func() {
		megabyte = originalMegabyte
		currentTime = originalCurrentTime
	}()

	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 设置 megabyte 变量的值为 1
	megabyte = 1

	// 创建临时目录用于测试
	dir := makeTempDir("TestRotateByDay_WithDateDirLayout", t)
	defer func() { _ = os.RemoveAll(dir) }()

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 LogRotateX 实例，启用按天轮转和日期目录布局
	l := &LogRotateX{
		LogFilePath:   filename,
		MaxSize:       100,  // 设置较大的 MaxSize，避免按大小轮转
		RotateByDay:   true, // 启用按天轮转
		DateDirLayout: true, // 启用日期目录布局
	}
	// 测试结束后关闭日志文件
	defer func() { _ = l.Close() }()

	// 定义要写入日志文件的初始数据
	b := []byte("day1 data")
	// 尝试将初始数据写入日志文件
	n, err := l.Write(b)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与初始数据的长度一致
	equals(len(b), n, t)

	// 验证日志文件的内容是否为初始数据
	existsWithContent(filename, b, t)

	// 模拟时间跨天
	newFakeDay()

	// 写入新数据，这将触发按天轮转
	b2 := []byte("day2 data")
	// 尝试将新数据写入日志文件
	n, err = l.Write(b2)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与新数据的长度一致
	equals(len(b2), n, t)

	// 验证主日志文件的内容是否为新写入的数据
	existsWithContent(filename, b2, t)

	// 验证备份文件是否存放在日期目录中
	// 直接读取目录，找到实际的日期目录
	entries, err := os.ReadDir(dir)
	isNil(err, t)

	// 找到日期目录（不是主日志文件）
	var backupDir string
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != filepath.Base(filename) {
			backupDir = filepath.Join(dir, entry.Name())
			break
		}
	}

	if backupDir == "" {
		t.Fatal("未找到日期目录")
	}

	files, err := os.ReadDir(backupDir)
	isNil(err, t)
	if len(files) != 1 {
		t.Errorf("期望日期目录中有1个文件，实际找到%d个", len(files))
	}

	// 验证备份文件的内容是否为初始数据
	backupFile := filepath.Join(backupDir, files[0].Name())
	existsWithContent(backupFile, b, t)

	t.Logf("✅ 按天轮转与日期目录布局配合测试通过")
}

// newFakeDay 将模拟的 "当前时间" 设置为一天后。
// 该函数用于测试按天轮转功能，模拟时间跨天的场景。
func newFakeDay() {
	// 将模拟的当前时间增加一天
	fakeCurrentTime = fakeCurrentTime.Add(time.Hour * 24)
}
