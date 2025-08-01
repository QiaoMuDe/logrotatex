package logrotatex

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMaxBackups 测试日志备份文件数量限制功能。
// 该测试会多次写入数据触发日志轮转，验证备份文件数量是否符合最大备份数限制，
// 同时验证非日志文件和目录不会被误删。
func TestMaxBackups(t *testing.T) {
	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 设置 megabyte 变量的值为 1
	megabyte = 1
	// 创建一个临时目录用于测试，目录名包含测试名称
	dir := makeTempDir("TestMaxBackups", t)

	// 测试结束后删除临时目录
	defer func() { _ = os.RemoveAll(dir) }()

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 LogRotateX 实例，指定日志文件路径、最大文件大小和最大备份数
	l := &LogRotateX{
		Filename:   filename,
		MaxSize:    10,
		MaxBackups: 1,
	}
	// 测试结束后关闭日志文件
	defer func() { _ = l.Close() }()
	// 定义要写入日志文件的初始数据
	b := []byte("boo!")
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

	// 模拟时间前进
	newFakeTime()

	// 写入新数据，这将触发日志轮转并超过最大备份数限制
	b2 := []byte("foooooo!")
	// 尝试将新数据写入日志文件
	n, err = l.Write(b2)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与新数据的长度一致
	equals(len(b2), n, t)

	// 获取第二个备份文件的路径
	secondFilename := backupFile(dir)

	// 验证第二个备份文件的内容是否为初始数据
	existsWithContent(secondFilename, b, t)

	// 验证主日志文件的内容是否为新写入的数据
	existsWithContent(filename, b2, t)

	// 验证临时目录中文件数量是否为 2，即存在主日志文件和一个备份文件
	fileCount(dir, 2, t)

	// 模拟时间前进
	newFakeTime()

	// 写入新数据，这将再次触发日志轮转
	b3 := []byte("baaaaaar!")
	// 尝试将新数据写入日志文件
	n, err = l.Write(b3)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与新数据的长度一致
	equals(len(b3), n, t)

	// 获取第三个备份文件的路径
	thirdFilename := backupFile(dir)
	// 验证第三个备份文件的内容是否为上一次写入的数据
	existsWithContent(thirdFilename, b2, t)

	// 验证主日志文件的内容是否为最新写入的数据
	existsWithContent(filename, b3, t)

	// 等待一段时间，因为文件删除操作在不同的 goroutine 中执行
	<-time.After(time.Millisecond * 10)

	// 验证临时目录中文件数量是否仍为 2，符合最大备份数限制
	fileCount(dir, 2, t)

	// 验证第三个备份文件是否仍然存在
	existsWithContent(thirdFilename, b2, t)

	// 验证第一个备份文件是否已被删除
	notExist(secondFilename, t)

	// 测试非日志文件和目录不会被误删
	// 模拟时间前进
	newFakeTime()

	// 创建一个与日志文件名相近但不同的文件，该文件不应被删除过滤器捕获
	notlogfile := logFile(dir) + ".foo"
	// 将数据写入该文件，文件权限设置为 0644
	err = os.WriteFile(notlogfile, []byte("data"), 0644)
	// 验证写入操作是否成功
	isNil(err, t)

	// 创建一个与日志文件过滤规则匹配的目录，该目录不应被删除过滤器捕获
	notlogfiledir := backupFile(dir)
	// 创建该目录，权限设置为 0700
	err = os.Mkdir(notlogfiledir, 0700)
	// 验证目录创建操作是否成功
	isNil(err, t)

	// 模拟时间前进
	newFakeTime()

	// 获取第四个备份文件的路径
	fourthFilename := backupFile(dir)

	// 创建一个正在或已被压缩的日志文件，该文件在压缩和未压缩状态都存在时不应被计数
	compLogFile := fourthFilename + compressSuffix
	// 将数据写入该压缩日志文件，文件权限设置为 0644
	err = os.WriteFile(compLogFile, []byte("compress"), 0644)
	// 验证写入操作是否成功
	isNil(err, t)

	// 写入新数据，这将再次触发日志轮转
	b4 := []byte("baaaaaaz!")
	// 尝试将新数据写入日志文件
	n, err = l.Write(b4)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与新数据的长度一致
	equals(len(b4), n, t)

	// 验证第四个备份文件的内容是否为上一次写入的数据
	existsWithContent(fourthFilename, b3, t)
	// 验证压缩日志文件的内容是否正确
	existsWithContent(fourthFilename+compressSuffix, []byte("compress"), t)

	// 等待一段时间，因为文件删除操作在不同的 goroutine 中执行
	<-time.After(time.Millisecond * 10)

	// 验证临时目录中文件数量是否为 5，包括 2 个日志文件、非日志文件和目录
	fileCount(dir, 5, t)

	// 验证主日志文件的内容是否为最新写入的数据
	existsWithContent(filename, b4, t)

	// 验证第四个备份文件是否仍然存在
	existsWithContent(fourthFilename, b3, t)

	// 验证第三个备份文件是否已被删除
	notExist(thirdFilename, t)

	// 验证非日志文件是否仍然存在
	exists(notlogfile, t)

	// 验证目录是否仍然存在
	exists(notlogfiledir, t)
}

// TestCleanupExistingBackups 测试当存在超过最大备份数的备份文件时，在日志轮转时多余的备份文件是否会被清理。
func TestCleanupExistingBackups(t *testing.T) {
	// test that if we start with more backup files than we're supposed to have
	// in total, that extra ones get cleaned up when we rotate.

	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 设置 megabyte 变量的值为 1
	megabyte = 1

	// 创建一个临时目录用于测试，目录名包含测试名称
	dir := makeTempDir("TestCleanupExistingBackups", t)
	// 测试结束后删除临时目录
	defer func() { _ = os.RemoveAll(dir) }()

	// 创建 3 个备份文件
	data := []byte("data")
	// 获取第一个备份文件路径
	backup := backupFile(dir)
	// 将数据写入第一个备份文件
	err := os.WriteFile(backup, data, 0644)
	// 验证写入操作是否成功
	isNil(err, t)

	// 模拟时间前进
	newFakeTime()

	// 获取第二个备份文件路径
	backup = backupFile(dir)
	// 将数据写入第二个备份文件的压缩版本
	err = os.WriteFile(backup+compressSuffix, data, 0644)
	// 验证写入操作是否成功
	isNil(err, t)

	// 模拟时间前进
	newFakeTime()

	// 获取第三个备份文件路径
	backup = backupFile(dir)
	// 将数据写入第三个备份文件
	err = os.WriteFile(backup, data, 0644)
	// 验证写入操作是否成功
	isNil(err, t)

	// 现在创建一个包含一些数据的主日志文件
	filename := logFile(dir)
	// 将数据写入主日志文件
	err = os.WriteFile(filename, data, 0644)
	// 验证写入操作是否成功
	isNil(err, t)

	// 创建一个 LogRotateX 实例，指定日志文件路径、最大文件大小和最大备份数
	l := &LogRotateX{
		Filename:   filename,
		MaxSize:    10,
		MaxBackups: 1,
	}
	// 测试结束后关闭日志文件
	defer func() { _ = l.Close() }()

	// 模拟时间前进
	newFakeTime()

	// 定义要写入日志文件的新数据
	b2 := []byte("foooooo!")
	// 尝试将新数据写入日志文件
	n, err := l.Write(b2)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与新数据的长度一致
	equals(len(b2), n, t)

	// 我们需要等待一小段时间，因为文件删除操作在不同的 goroutine 中执行
	<-time.After(time.Millisecond * 10)

	// 现在我们应该只剩下 2 个文件 - 主日志文件和一个备份文件
	fileCount(dir, 2, t)
}

// TestMaxAge 测试备份文件最大保留天数的功能
func TestMaxAge(t *testing.T) {
	currentTime = fakeTime
	// 设置 1 兆字节的大小
	megabyte = 1

	// 创建一个临时目录用于测试
	dir := makeTempDir("TestMaxAge", t)
	// 测试结束后删除临时目录
	defer func() { _ = os.RemoveAll(dir) }()

	// 获取日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 Logger 实例
	l := &LogRotateX{
		Filename: filename,
		MaxSize:  10,
		MaxAge:   1,
	}
	// 测试结束后关闭 Logger
	defer func() { _ = l.Close() }()
	// 定义要写入日志的内容
	b := []byte("boo!")
	// 向日志文件写入内容
	n, err := l.Write(b)
	// 断言写入操作没有错误
	isNil(err, t)
	// 断言写入的字节数与预期一致
	equals(len(b), n, t)

	// 断言文件存在且内容正确
	existsWithContent(filename, b, t)
	// 断言目录中文件数量正确
	fileCount(dir, 1, t)

	// 两天后
	newFakeTime()

	// 定义要写入日志的内容
	b2 := []byte("foooooo!")
	// 向日志文件写入内容
	n, err = l.Write(b2)
	// 断言写入操作没有错误
	isNil(err, t)
	// 断言写入的字节数与预期一致
	equals(len(b2), n, t)
	// 断言备份文件内容正确
	existsWithContent(backupFile(dir), b, t)

	// 由于文件删除操作在另一个 goroutine 中进行，需要等待一段时间
	<-time.After(10 * time.Millisecond)

	// 目录中应该仍然有两个日志文件，因为最新的备份文件刚刚创建
	fileCount(dir, 2, t)

	// 断言主日志文件内容正确
	existsWithContent(filename, b2, t)

	// 应该已经删除了旧的备份文件，因为它超过了保留天数
	existsWithContent(backupFile(dir), b, t)

	// 两天后
	newFakeTime()

	// 定义要写入日志的内容
	b3 := []byte("baaaaar!")
	// 向日志文件写入内容
	n, err = l.Write(b3)
	// 断言写入操作没有错误
	isNil(err, t)
	// 断言写入的字节数与预期一致
	equals(len(b3), n, t)
	// 断言备份文件内容正确
	existsWithContent(backupFile(dir), b2, t)

	// 由于文件删除操作在另一个 goroutine 中进行，需要等待一段时间
	<-time.After(10 * time.Millisecond)

	// 目录中应该有两个日志文件 - 主日志文件和最新的备份文件，较早的备份文件应该已被删除
	fileCount(dir, 2, t)

	// 断言主日志文件内容正确
	existsWithContent(filename, b3, t)

	// 应该已经删除了旧的备份文件，因为它超过了保留天数
	existsWithContent(backupFile(dir), b2, t)
}

// TestOldLogFiles 测试获取旧日志文件列表的功能，验证返回的旧日志文件列表是否按时间排序。
func TestOldLogFiles(t *testing.T) {
	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 设置 megabyte 变量的值为 1
	megabyte = 1

	// 创建一个临时目录用于测试，目录名包含测试名称
	dir := makeTempDir("TestOldLogFiles", t)
	// 测试结束后删除临时目录
	defer func() { _ = os.RemoveAll(dir) }()

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 定义要写入文件的数据
	data := []byte("data")
	// 将数据写入日志文件
	err := os.WriteFile(filename, data, 07)
	// 验证写入操作是否成功
	isNil(err, t)

	// 这将为我们提供与从文件名中的时间戳获得的时间相同精度的时间。
	t1, err := time.Parse(backupTimeFormat, fakeTime().UTC().Format(backupTimeFormat))
	// 验证时间解析操作是否成功
	isNil(err, t)

	// 获取第一个备份文件路径
	backup := backupFile(dir)
	// 将数据写入第一个备份文件
	err = os.WriteFile(backup, data, 07)
	// 验证写入操作是否成功
	isNil(err, t)

	// 模拟时间前进
	newFakeTime()

	// 这将为我们提供与从文件名中的时间戳获得的时间相同精度的时间。
	t2, err := time.Parse(backupTimeFormat, fakeTime().UTC().Format(backupTimeFormat))
	// 验证时间解析操作是否成功
	isNil(err, t)

	// 获取第二个备份文件路径
	backup2 := backupFile(dir)
	// 将数据写入第二个备份文件
	err = os.WriteFile(backup2, data, 07)
	// 验证写入操作是否成功
	isNil(err, t)

	// 创建一个 LogRotateX 实例，指定日志文件路径
	l := &LogRotateX{Filename: filename}
	// 获取旧日志文件列表
	files, err := l.oldLogFiles()
	// 验证获取操作是否成功
	isNil(err, t)
	// 验证旧日志文件列表的长度是否为 2
	equals(2, len(files), t)

	// 应该按最新文件优先排序，即 t2 应该是第一个
	equals(t2, files[0].timestamp, t)
	equals(t1, files[1].timestamp, t)
}

// TestTimeFromName 测试从文件名中解析时间的功能，验证不同格式的文件名是否能正确解析出时间。
func TestTimeFromName(t *testing.T) {
	// 创建一个 LogRotateX 实例，指定日志文件路径
	l := &LogRotateX{Filename: "/var/log/myfoo/foo.log"}
	// 获取日志文件名前缀和扩展名
	prefix, ext := l.prefixAndExt()

	// 定义测试用例
	tests := []struct {
		filename string
		want     time.Time
		wantErr  bool
	}{
		{"foo-2014-05-04T14-44-33.555.log", time.Date(2014, 5, 4, 14, 44, 33, 555000000, time.UTC), false},
		{"foo-2014-05-04T14-44-33.555", time.Time{}, true},
		{"2014-05-04T14-44-33.555.log", time.Time{}, true},
		{"foo.log", time.Time{}, true},
	}

	// 遍历测试用例
	for _, test := range tests {
		// 从文件名中解析时间
		got, err := l.timeFromName(test.filename, prefix, ext)
		// 验证解析结果是否与预期一致
		equals(got, test.want, t)
		// 验证解析是否返回预期的错误
		equals(err != nil, test.wantErr, t)
	}
}

// TestLocalTime 测试 LogRotateX 在启用 LocalTime 选项时的行为。
// 预期结果是在写入数据并触发日志轮转后，主日志文件包含最新写入的数据，备份文件包含旧的数据，且备份文件的时间戳使用本地时间。
func TestLocalTime(t *testing.T) {
	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 设置 megabyte 变量的值为 1
	megabyte = 1

	// 创建一个临时目录用于测试，目录名包含测试名称
	dir := makeTempDir("TestLocalTime", t)
	// 测试结束后删除临时目录
	defer func() { _ = os.RemoveAll(dir) }()

	// 创建一个 LogRotateX 实例，指定日志文件路径、最大文件大小，并启用 LocalTime 选项
	l := &LogRotateX{
		Filename:  logFile(dir),
		MaxSize:   10,
		LocalTime: true,
	}
	// 测试结束后关闭日志文件
	defer func() { _ = l.Close() }()
	// 定义要写入日志文件的初始数据
	b := []byte("boo!")
	// 尝试将初始数据写入日志文件
	n, err := l.Write(b)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与初始数据的长度一致
	equals(len(b), n, t)

	// 定义要追加到日志文件的新数据
	b2 := []byte("fooooooo!")
	// 尝试将新数据写入日志文件
	n2, err := l.Write(b2)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与新数据的长度一致
	equals(len(b2), n2, t)

	// 验证主日志文件的内容是否为新写入的数据
	existsWithContent(logFile(dir), b2, t)
	// 验证使用本地时间生成的备份文件的内容是否为初始数据
	existsWithContent(backupFileLocal(dir), b, t)
}

// TestDefaultFilename 测试 LogRotateX 在使用默认文件名时的行为。
// 该测试会创建一个 LogRotateX 实例，不指定文件名，期望其使用默认文件名。
// 然后向该实例写入数据，并验证写入操作是否成功，以及文件内容是否正确。
func TestDefaultFilename(t *testing.T) {
	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 获取系统临时目录
	dir := os.TempDir()
	// 构建默认的日志文件名，格式为 程序名-logrotatex.log
	filename := filepath.Join(dir, filepath.Base(os.Args[0])+"-logrotatex.log")
	// 测试结束后删除该日志文件
	defer func() { _ = os.Remove(filename) }()
	// 创建一个 LogRotateX 实例，不指定文件名，使用默认配置
	l := &LogRotateX{}
	// 测试结束后关闭日志文件
	defer func() { _ = l.Close() }()
	// 定义要写入日志文件的数据
	b := []byte("boo!")
	// 尝试将数据写入日志文件
	n, err := l.Write(b)

	// 验证写入操作是否没有出错
	isNil(err, t)
	// 验证实际写入的字节数是否与定义的数据长度一致
	equals(len(b), n, t)
	// 验证日志文件是否存在，并且其内容与写入的数据一致
	existsWithContent(filename, b, t)
}

// TestAutoRotate 测试日志自动轮转功能。当写入的数据使得日志文件达到最大大小时，
// 旧的日志文件应该被移动到备份文件，并且主日志文件应该只包含最后一次写入的数据。
func TestAutoRotate(t *testing.T) {
	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 设置 megabyte 变量的值为 1
	megabyte = 1

	// 创建一个临时目录用于测试，目录名包含测试名称
	dir := makeTempDir("TestAutoRotate", t)
	// 测试结束后删除临时目录
	defer func() { _ = os.RemoveAll(dir) }()

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 LogRotateX 实例，指定日志文件路径和最大文件大小
	l := &LogRotateX{
		Filename: filename,
		MaxSize:  10,
	}
	// 测试结束后关闭日志文件
	defer func() { _ = l.Close() }()
	// 定义要写入日志文件的初始数据
	b := []byte("boo!")
	// 尝试将初始数据写入日志文件
	n, err := l.Write(b)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与初始数据的长度一致
	equals(len(b), n, t)

	// 验证日志文件是否存在，并且其内容与初始数据一致
	existsWithContent(filename, b, t)
	// 验证临时目录中文件数量是否为 1，即只存在一个日志文件
	fileCount(dir, 1, t)

	// 将模拟时间设置为两天后
	newFakeTime()

	// 定义要追加到日志文件的新数据
	b2 := []byte("foooooo!")
	// 尝试将新数据写入日志文件
	n, err = l.Write(b2)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与新数据的长度一致
	equals(len(b2), n, t)

	// 旧的日志文件应该被移动到备份文件，并且主日志文件应该只包含最后一次写入的数据
	existsWithContent(filename, b2, t)

	// 备份文件将使用当前的模拟时间，并且包含旧的日志内容
	existsWithContent(backupFile(dir), b, t)

	// 验证临时目录中文件数量是否为 2，即存在主日志文件和备份文件
	fileCount(dir, 2, t)
}

// TestFirstWriteRotate 测试首次写入时触发日志轮转的情况。
// 该测试会先向日志文件写入初始数据，然后模拟时间流逝，
// 再写入新数据，验证是否触发日志轮转，旧日志是否被备份。
func TestFirstWriteRotate(t *testing.T) {
	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 设置 megabyte 变量的值为 1
	megabyte = 1
	// 创建一个临时目录用于测试，目录名包含测试名称
	dir := makeTempDir("TestFirstWriteRotate", t)
	// 测试结束后删除临时目录
	defer func() { _ = os.RemoveAll(dir) }()

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 LogRotateX 实例，指定日志文件路径和最大文件大小
	l := &LogRotateX{
		Filename: filename,
		MaxSize:  10,
	}
	// 测试结束后关闭日志文件
	defer func() { _ = l.Close() }()

	// 定义要写入现有文件的初始数据
	start := []byte("boooooo!")
	// 将初始数据写入日志文件，文件权限设置为 0600
	err := os.WriteFile(filename, start, 0600)
	// 验证写入操作是否成功
	isNil(err, t)

	// 模拟时间前进
	newFakeTime()

	// 写入新数据，这将触发日志轮转
	b := []byte("fooo!")
	// 尝试将新数据写入日志文件
	n, err := l.Write(b)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与新数据的长度一致
	equals(len(b), n, t)

	// 验证日志文件的内容是否为新写入的数据
	existsWithContent(filename, b, t)
	// 验证备份文件的内容是否为初始数据
	existsWithContent(backupFile(dir), start, t)

	// 验证临时目录中文件数量是否为 2，即存在主日志文件和备份文件
	fileCount(dir, 2, t)
}
