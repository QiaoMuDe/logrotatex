package logrotatex

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// !!!NOTE!!!
//
// 并行运行这些测试几乎肯定会导致偶发性（甚至是经常性）的测试失败，
// 因为所有测试都在操作同一个全局变量，该变量控制着模拟的 time.Now 逻辑。
// 所以，请不要并行运行这些测试。

// 由于所有测试都依赖时间来确定文件名等信息，
// 因此我们需要尽可能地控制时钟，这意味着只有在我们希望时钟变化时，它才会改变。
// fakeCurrentTime 是一个全局变量，用于存储模拟的当前时间，初始值为系统当前时间。
var fakeCurrentTime = time.Now()

// fakeTime 函数用于返回模拟的当前时间。
// 在测试环境中，为了确保测试的可重复性，需要固定时间，
// 该函数会返回预先设置好的 fakeCurrentTime 变量的值。
func fakeTime() time.Time {
	return fakeCurrentTime
}

func TestNewFile(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir("TestNewFile", t)
	defer os.RemoveAll(dir)
	l := &LogRotateX{
		Filename: logFile(dir),
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)
	existsWithContent(logFile(dir), b, t)
	fileCount(dir, 1, t)
}

// TestOpenExisting 测试当 LogRotateX 实例打开一个已存在的日志文件时的行为。
// 预期结果是新写入的数据会追加到现有文件内容之后，且不会创建新的文件。
func TestOpenExisting(t *testing.T) {
	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 创建一个临时目录用于测试，目录名包含测试名称
	dir := makeTempDir("TestOpenExisting", t)
	// 测试结束后删除临时目录
	defer os.RemoveAll(dir)

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 定义要写入现有文件的初始数据
	data := []byte("foo!")
	// 将初始数据写入日志文件，文件权限设置为 0644
	err := os.WriteFile(filename, data, 0644)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证文件是否存在且内容与写入的数据一致
	existsWithContent(filename, data, t)

	// 创建一个 LogRotateX 实例，指定要操作的日志文件路径
	l := &LogRotateX{
		Filename: filename,
	}
	// 测试结束后关闭日志文件
	defer l.Close()
	// 定义要追加到日志文件的新数据
	b := []byte("boo!")
	// 尝试将新数据写入日志文件
	n, err := l.Write(b)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与新数据的长度一致
	equals(len(b), n, t)

	// 验证日志文件的内容是否为初始数据和新数据的组合，即新数据是否追加成功
	existsWithContent(filename, append(data, b...), t)

	// 验证临时目录中文件数量是否为 1，即没有创建新的文件
	fileCount(dir, 1, t)
}

// TestWriteTooLong 测试当写入的数据长度超过日志文件最大大小时的行为。
// 预期结果是写入操作失败，返回错误，且日志文件不会被创建。
func TestWriteTooLong(t *testing.T) {
	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 设置 megabyte 变量的值为 1
	megabyte = 1
	// 创建一个临时目录用于测试，目录名包含测试名称
	dir := makeTempDir("TestWriteTooLong", t)
	// 测试结束后删除临时目录
	defer os.RemoveAll(dir)

	// 创建一个 LogRotateX 实例，指定日志文件路径和最大文件大小
	l := &LogRotateX{
		Filename: logFile(dir),
		MaxSize:  5,
	}
	// 测试结束后关闭日志文件
	defer l.Close()

	// 创建一个字节切片，其长度超过设置的最大文件大小
	b := []byte("booooooooooooooo!")
	// 尝试向日志文件写入数据
	n, err := l.Write(b)
	// 验证写入操作是否返回错误
	notNil(err, t)
	// 验证写入的字节数是否为 0
	equals(0, n, t)
	// 验证返回的错误信息是否符合预期
	equals(err.Error(),
		fmt.Sprintf("写入长度 %d 超过最大文件大小 %d", len(b), l.MaxSize), t)

	// 检查日志文件是否存在
	_, err = os.Stat(logFile(dir))
	// 验证日志文件是否不存在
	assert(os.IsNotExist(err), t, "文件存在，但本不该被创建。")
}

// TestMakeLogDir 测试 LogRotateX 在日志目录不存在时，是否能正确创建目录并写入日志文件。
func TestMakeLogDir(t *testing.T) {
	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 生成一个包含测试名称和当前时间格式的目录名
	dir := time.Now().Format("TestMakeLogDir" + backupTimeFormat)
	// 将生成的目录名与系统临时目录拼接，得到完整的临时目录路径
	dir = filepath.Join(os.TempDir(), dir)
	// 测试结束后，删除该临时目录及其所有内容
	defer os.RemoveAll(dir)
	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 LogRotateX 实例，指定要操作的日志文件路径
	l := &LogRotateX{
		Filename: filename,
	}
	// 测试结束后关闭日志文件
	defer l.Close()
	// 定义要写入日志文件的数据
	b := []byte("boo!")
	// 尝试将数据写入日志文件
	n, err := l.Write(b)
	// 验证写入操作是否没有出错
	isNil(err, t)
	// 验证实际写入的字节数是否与定义的数据长度一致
	equals(len(b), n, t)
	// 验证日志文件是否存在，并且其内容与写入的数据一致
	existsWithContent(logFile(dir), b, t)
	// 验证临时目录中文件数量是否为 1，即只存在一个日志文件
	fileCount(dir, 1, t)
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
	defer os.Remove(filename)
	// 创建一个 LogRotateX 实例，不指定文件名，使用默认配置
	l := &LogRotateX{}
	// 测试结束后关闭日志文件
	defer l.Close()
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
	defer os.RemoveAll(dir)

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 LogRotateX 实例，指定日志文件路径和最大文件大小
	l := &LogRotateX{
		Filename: filename,
		MaxSize:  10,
	}
	// 测试结束后关闭日志文件
	defer l.Close()
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
	defer os.RemoveAll(dir)

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 LogRotateX 实例，指定日志文件路径和最大文件大小
	l := &LogRotateX{
		Filename: filename,
		MaxSize:  10,
	}
	// 测试结束后关闭日志文件
	defer l.Close()

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
	defer os.RemoveAll(dir)

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 LogRotateX 实例，指定日志文件路径、最大文件大小和最大备份数
	l := &LogRotateX{
		Filename:   filename,
		MaxSize:    10,
		MaxBackups: 1,
	}
	// 测试结束后关闭日志文件
	defer l.Close()
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
	defer os.RemoveAll(dir)

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
	defer l.Close()

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
	defer os.RemoveAll(dir)

	// 获取日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 Logger 实例
	l := &LogRotateX{
		Filename: filename,
		MaxSize:  10,
		MaxAge:   1,
	}
	// 测试结束后关闭 Logger
	defer l.Close()
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
	defer os.RemoveAll(dir)

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
	defer os.RemoveAll(dir)

	// 创建一个 LogRotateX 实例，指定日志文件路径、最大文件大小，并启用 LocalTime 选项
	l := &LogRotateX{
		Filename:  logFile(dir),
		MaxSize:   10,
		LocalTime: true,
	}
	// 测试结束后关闭日志文件
	defer l.Close()
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

// TestRotate 测试 LogRotateX 的日志轮转功能。
// 预期结果是在多次触发日志轮转后，备份文件的数量符合最大备份数限制，且主日志文件包含最新写入的数据。
func TestRotate(t *testing.T) {
	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 创建一个临时目录用于测试，目录名包含测试名称
	dir := makeTempDir("TestRotate", t)
	// 测试结束后删除临时目录
	defer os.RemoveAll(dir)

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)

	// 创建一个 LogRotateX 实例，指定日志文件路径、最大备份数和最大文件大小
	l := &LogRotateX{
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	// 测试结束后关闭日志文件
	defer l.Close()
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

	// 触发日志轮转
	err = l.Rotate()
	// 验证日志轮转操作是否成功
	isNil(err, t)

	// 我们需要等待一小段时间，因为文件删除操作在不同的 goroutine 中执行
	<-time.After(10 * time.Millisecond)

	// 获取第一个备份文件的路径
	filename2 := backupFile(dir)
	// 验证第一个备份文件的内容是否为初始数据
	existsWithContent(filename2, b, t)
	// 验证主日志文件的内容是否为空
	existsWithContent(filename, []byte{}, t)
	// 验证临时目录中文件数量是否为 2，即存在主日志文件和一个备份文件
	fileCount(dir, 2, t)
	// 模拟时间前进
	newFakeTime()

	// 再次触发日志轮转
	err = l.Rotate()
	// 验证日志轮转操作是否成功
	isNil(err, t)

	// 我们需要等待一小段时间，因为文件删除操作在不同的 goroutine 中执行
	<-time.After(10 * time.Millisecond)

	// 获取第二个备份文件的路径
	filename3 := backupFile(dir)
	// 验证第二个备份文件的内容是否为空
	existsWithContent(filename3, []byte{}, t)
	// 验证主日志文件的内容是否为空
	existsWithContent(filename, []byte{}, t)
	// 验证临时目录中文件数量是否为 2，符合最大备份数限制
	fileCount(dir, 2, t)

	// 定义要写入日志文件的新数据
	b2 := []byte("foooooo!")
	// 尝试将新数据写入日志文件
	n, err = l.Write(b2)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与新数据的长度一致
	equals(len(b2), n, t)

	// 验证主日志文件的内容是否为最新写入的数据
	existsWithContent(filename, b2, t)
}

// TestCompressOnRotate 测试 LogRotateX 在日志轮转时的压缩功能。
// 预期结果是在触发日志轮转后，旧的日志文件被压缩，原始文件被移除，且压缩文件的内容与原始文件一致。
func TestCompressOnRotate(t *testing.T) {
	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 设置 megabyte 变量的值为 1
	megabyte = 1

	// 创建一个临时目录用于测试，目录名包含测试名称
	dir := makeTempDir("TestCompressOnRotate", t)
	// 测试结束后删除临时目录
	defer os.RemoveAll(dir)

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 LogRotateX 实例，启用压缩功能，指定日志文件路径和最大文件大小
	l := &LogRotateX{
		Compress: true,
		Filename: filename,
		MaxSize:  10,
	}
	// 测试结束后关闭日志文件
	defer l.Close()
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

	// 触发日志轮转
	err = l.Rotate()
	// 验证日志轮转操作是否成功
	isNil(err, t)

	// 旧的日志文件应该被移到一边，主日志文件应该为空
	existsWithContent(filename, []byte{}, t)

	// 我们需要等待一小段时间，因为文件压缩操作在不同的 goroutine 中执行
	<-time.After(300 * time.Millisecond)

	// 日志文件的压缩版本现在应该存在，原始文件应该已被移除
	bc := new(bytes.Buffer)
	gz := gzip.NewWriter(bc)
	_, err = gz.Write(b)
	// 验证写入压缩文件操作是否成功
	isNil(err, t)
	err = gz.Close()
	// 验证关闭压缩文件操作是否成功
	isNil(err, t)
	// 验证压缩文件的内容是否与初始数据一致
	existsWithContent(backupFile(dir)+compressSuffix, bc.Bytes(), t)
	// 验证原始备份文件是否已被移除
	notExist(backupFile(dir), t)

	// 验证临时目录中文件数量是否为 2，包括主日志文件和压缩备份文件
	fileCount(dir, 2, t)
}

// TestCompressOnResume 测试在恢复操作时的日志压缩功能。
// 该测试会创建一个备份文件和一个空的压缩文件，然后写入新数据，
// 验证日志文件是否被正确压缩，并且原始文件是否被删除。
func TestCompressOnResume(t *testing.T) {
	// 将当前时间设置为模拟时间，确保测试的可重复性
	currentTime = fakeTime
	// 设置 megabyte 变量的值为 1
	megabyte = 1

	// 创建一个临时目录用于测试，目录名包含测试名称，测试结束后删除该目录
	dir := makeTempDir("TestCompressOnResume", t)
	defer os.RemoveAll(dir)

	// 获取临时目录下日志文件的完整路径
	filename := logFile(dir)
	// 创建一个 LogRotateX 实例，启用压缩功能，指定日志文件路径和最大文件大小
	l := &LogRotateX{
		Compress: true,
		Filename: filename,
		MaxSize:  10,
	}
	// 测试结束后关闭日志文件
	defer l.Close()

	// 创建一个备份文件和空的 "压缩" 文件。
	filename2 := backupFile(dir)
	// 定义要写入备份文件的数据
	b := []byte("foo!")
	// 将数据写入备份文件，文件权限设置为 0644
	err := os.WriteFile(filename2, b, 0644)
	// 验证写入操作是否成功
	isNil(err, t)
	// 创建一个空的压缩文件，文件权限设置为 0644
	err = os.WriteFile(filename2+compressSuffix, []byte{}, 0644)
	// 验证写入操作是否成功
	isNil(err, t)

	// 模拟时间前进两天
	newFakeTime()

	// 定义要写入日志文件的新数据
	b2 := []byte("boo!")
	// 尝试将新数据写入日志文件
	n, err := l.Write(b2)
	// 验证写入操作是否成功
	isNil(err, t)
	// 验证实际写入的字节数是否与新数据的长度一致
	equals(len(b2), n, t)
	// 验证日志文件是否存在，并且其内容与新数据一致
	existsWithContent(filename, b2, t)

	// 我们需要等待一小段时间，因为文件压缩操作在不同的 goroutine 中执行。
	<-time.After(300 * time.Millisecond)

	// 写入操作应该已经启动了压缩 - 现在应该存在一个压缩版本的日志文件，并且原始文件应该已被删除。
	// 创建一个字节缓冲区用于存储压缩后的数据
	bc := new(bytes.Buffer)
	// 创建一个 gzip 写入器，将数据写入字节缓冲区
	gz := gzip.NewWriter(bc)
	// 尝试将备份文件的数据写入 gzip 写入器
	_, err = gz.Write(b)
	// 验证写入操作是否成功
	isNil(err, t)
	// 关闭 gzip 写入器，完成压缩操作
	err = gz.Close()
	// 验证关闭操作是否成功
	isNil(err, t)
	// 验证压缩文件是否存在，并且其内容与压缩后的数据一致
	existsWithContent(filename2+compressSuffix, bc.Bytes(), t)
	// 验证原始备份文件是否已被删除
	notExist(filename2, t)

	// 验证临时目录中文件数量是否为 2，即存在主日志文件和压缩后的备份文件
	fileCount(dir, 2, t)
}

// TestJson 测试将 JSON 数据反序列化为 LogRotateX 结构体的功能。
// 该测试会定义一个 JSON 数据，然后尝试将其反序列化为 LogRotateX 实例，
// 验证反序列化后的实例的各个字段是否与 JSON 数据中的值一致。
func TestJson(t *testing.T) {
	// 定义一个 JSON 数据，去除第一行的换行符
	data := []byte(`
{
	"filename": "foo",
	"maxsize": 5,
	"maxage": 10,
	"maxbackups": 3,
	"localtime": true,
	"compress": true
}`[1:])

	// 创建一个 LogRotateX 实例
	l := LogRotateX{}
	// 尝试将 JSON 数据反序列化为 LogRotateX 实例
	err := json.Unmarshal(data, &l)
	// 验证反序列化操作是否成功
	isNil(err, t)
	// 验证反序列化后的实例的 Filename 字段是否与 JSON 数据中的值一致
	equals("foo", l.Filename, t)
	// 验证反序列化后的实例的 MaxSize 字段是否与 JSON 数据中的值一致
	equals(5, l.MaxSize, t)
	// 验证反序列化后的实例的 MaxAge 字段是否与 JSON 数据中的值一致
	equals(10, l.MaxAge, t)
	// 验证反序列化后的实例的 MaxBackups 字段是否与 JSON 数据中的值一致
	equals(3, l.MaxBackups, t)
	// 验证反序列化后的实例的 LocalTime 字段是否与 JSON 数据中的值一致
	equals(true, l.LocalTime, t)
	// 验证反序列化后的实例的 Compress 字段是否与 JSON 数据中的值一致
	equals(true, l.Compress, t)
}

// makeTempDir 创建一个在操作系统临时目录下具有半唯一名称的目录。
// 该目录名基于测试名称生成，以避免并行测试之间的冲突，并且在测试结束后必须被清理。
func makeTempDir(name string, t testing.TB) string {
	// 根据测试名称和当前时间生成目录名
	dir := time.Now().Format(name + backupTimeFormat)
	// 将生成的目录名与系统临时目录拼接，得到完整的临时目录路径
	dir = filepath.Join(os.TempDir(), dir)
	// 创建该临时目录，权限设置为 0700，并验证创建操作是否成功
	isNilUp(os.Mkdir(dir, 0700), t, 1)
	return dir
}

// existsWithContent 检查指定文件是否存在，并且其内容是否与预期内容一致。
func existsWithContent(path string, content []byte, t testing.TB) {
	// 获取文件信息
	info, err := os.Stat(path)
	// 验证获取文件信息的操作是否成功
	isNilUp(err, t, 1)
	// 验证文件大小是否与预期内容的长度一致
	equalsUp(int64(len(content)), info.Size(), t, 1)

	// 读取文件内容
	b, err := os.ReadFile(path)
	// 验证读取文件内容的操作是否成功
	isNilUp(err, t, 1)
	// 验证文件内容是否与预期内容一致
	equalsUp(content, b, t, 1)
}

// logFile 返回指定目录下当前模拟时间对应的日志文件的完整路径。
func logFile(dir string) string {
	// 将目录路径和日志文件名拼接，得到完整的日志文件路径
	return filepath.Join(dir, "foobar.log")
}

// backupFile 返回指定目录下当前模拟时间对应的备份文件的完整路径，使用 UTC 时间格式。
func backupFile(dir string) string {
	// 将目录路径、备份文件名前缀、当前模拟时间的 UTC 格式和文件扩展名拼接，得到完整的备份文件路径
	return filepath.Join(dir, "foobar-"+fakeTime().UTC().Format(backupTimeFormat)+".log")
}

// backupFileLocal 返回指定目录下当前模拟时间对应的备份文件的完整路径，使用本地时间格式。
func backupFileLocal(dir string) string {
	// 将目录路径、备份文件名前缀、当前模拟时间的本地时间格式和文件扩展名拼接，得到完整的备份文件路径
	return filepath.Join(dir, "foobar-"+fakeTime().Format(backupTimeFormat)+".log")
}

// fileCount 检查指定目录下的文件数量是否与预期数量一致。
func fileCount(dir string, exp int, t testing.TB) {
	// 读取指定目录下的所有文件和子目录
	files, err := os.ReadDir(dir)
	// 验证读取目录的操作是否成功
	isNilUp(err, t, 1)
	// 确保没有创建其他文件，验证文件数量是否与预期数量一致
	equalsUp(exp, len(files), t, 1)
}

// newFakeTime 将模拟的 "当前时间" 设置为两天后。
func newFakeTime() {
	// 将模拟的当前时间增加两天
	fakeCurrentTime = fakeCurrentTime.Add(time.Hour * 24 * 2)
}

// notExist 检查指定文件是否不存在。
func notExist(path string, t testing.TB) {
	// 获取文件信息
	_, err := os.Stat(path)
	// 验证是否返回 os.IsNotExist 错误，即文件是否不存在
	assertUp(os.IsNotExist(err), t, 1, "expected to get os.IsNotExist, but instead got %v", err)
}

// exists 检查指定文件是否存在。
func exists(path string, t testing.TB) {
	// 获取文件信息
	_, err := os.Stat(path)
	// 验证是否成功获取文件信息，即文件是否存在
	assertUp(err == nil, t, 1, "expected file to exist, but got error from os.Stat: %v", err)
}
