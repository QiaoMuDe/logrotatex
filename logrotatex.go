
// logrotatex 旨在成为日志记录基础设施的一部分。
// 它并非一个一体化的解决方案，而是日志记录栈底层的一个可插拔组件，仅用于控制日志写入的文件。
//
// logrotatex 可以与任何能够写入 io.Writer 的日志记录包配合使用，包括标准库的 log 包。
//
// logrotatex 假定只有一个进程在向输出文件写入日志。
// 在同一台机器上的多个进程使用相同的 logrotatex 配置会导致异常行为。
package logrotatex

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// backupTimeFormat 是备份文件的时间戳格式，用于在文件名中嵌入时间信息。
	backupTimeFormat = "2006-01-02T15-04-05.000"

	// compressSuffix 是压缩文件的后缀，用于标识已压缩的日志文件。
	compressSuffix = ".gz"

	// defaultMaxSize 是日志文件的最大默认大小（单位：MB），在未明确设置时使用此值。
	defaultMaxSize = 10
)

// LogRotateX 是一个 io.WriteCloser，它写入指定的文件名。
var _ io.WriteCloser = (*LogRotateX)(nil)

// LogRotateX 是一个 io.WriteCloser，它会将日志写入指定的文件名。
//
// 首次调用 Write 方法时，LogRotateX 会打开或创建日志文件。如果文件已存在且大小小于 MaxSize 兆字节，logrotatex 会打开该文件并追加写入。
// 如果文件已存在且大小大于或等于 MaxSize 兆字节，该文件会被重命名，重命名时会在文件名的扩展名之前（如果没有扩展名，则在文件名末尾）插入当前时间戳。然后会使用原始文件名创建一个新的日志文件。
//
// 每当写入操作会导致当前日志文件大小超过 MaxSize 兆字节时，当前文件会被关闭、重命名，并使用原始文件名创建一个新的日志文件。因此，你提供给 LogRotateX 的文件名始终是“当前”的日志文件。
//
// 备份文件使用提供给 LogRotateX 的日志文件名，格式为 `name-timestamp.ext`，其中 name 是不带扩展名的文件名，timestamp 是日志轮转时的时间，格式为 `2006-01-02T15-04-05.000`，ext 是原始扩展名。
// 例如，如果你的 LogRotateX.Filename 是 `/var/log/foo/server.log`，在 2016 年 11 月 11 日下午 6:30 创建的备份文件名将是 `/var/log/foo/server-2016-11-04T18-30-00.000.log`。
//
// # 清理旧日志文件
//
// 每当创建新的日志文件时，可能会删除旧的日志文件。根据编码的时间戳，最近的文件会被保留，最多保留数量等于 MaxBackups（如果 MaxBackups 为 0，则保留所有文件）。
// 任何编码时间戳早于 MaxAge 天的文件都会被删除，无论 MaxBackups 的设置如何。请注意，时间戳中编码的时间是轮转时间，可能与该文件最后一次写入的时间不同。
//
// 如果 MaxBackups 和 MaxAge 都为 0，则不会删除任何旧日志文件。
type LogRotateX struct {
	// Filename 是写入日志的文件。备份日志文件将保留在同一目录中。如果该值为空，则使用 os.TempDir() 下的 <进程名>-logrotatex.log。
	Filename string `json:"filename" yaml:"filename"`

	// MaxSize 最大单个日志文件的大小（以 MB 为单位）。默认值为 10 MB。
	MaxSize int `json:"maxsize" yaml:"maxsize"`

	// MaxAge 最大保留日志文件的天数。默认情况下，不会删除旧日志文件。
	MaxAge int `json:"maxage" yaml:"maxage"`

	// MaxBackups 最大保留日志文件的数量。默认情况下，不会删除旧日志文件。
	MaxBackups int `json:"maxbackups" yaml:"maxbackups"`

	// LocalTime 决定是否使用本地时间记录日志文件的轮转时间。默认使用 UTC 时间。
	LocalTime bool `json:"localtime" yaml:"localtime"`

	// Compress 决定轮转后的日志文件是否应使用 gzip 进行压缩。默认不进行压缩。
	Compress bool `json:"compress" yaml:"compress"`

	// size 是当前日志文件的大小（以字节为单位）。
	size int64

	// file 是当前打开的日志文件。
	file *os.File

	// mu 是互斥锁，用于保护文件操作。
	mu sync.Mutex

	// millCh 是一个通道，用于通知 LogRotateX 进行压缩和删除旧日志文件。
	millCh chan bool

	// startMill 是一个 sync.Once，用于确保只启动一次压缩和删除旧日志文件的 goroutine。
	startMill sync.Once
}

var (
	// currentTime 是一个函数，用于返回当前时间。它是一个变量，这样测试时可以对其进行模拟。
	currentTime = time.Now

	// megabyte 是用于将MB转换为字节的常量。
	megabyte = 1024 * 1024
)

// Write 方法实现了 io.Writer 接口。如果一次写入操作会使日志文件大小超过 MaxSize，
// 则关闭当前文件，将其重命名并包含当前时间戳，然后使用原始文件名创建一个新的日志文件。
// 如果写入内容的长度大于 MaxSize，则返回一个错误。
func (l *LogRotateX) Write(p []byte) (n int, err error) {
	// 加锁以确保并发安全，防止多个 goroutine 同时操作文件
	l.mu.Lock()
	// 函数返回时解锁，保证锁一定会被释放
	defer l.mu.Unlock()

	// 计算要写入的数据长度
	writeLen := int64(len(p))
	// 检查写入的数据长度是否超过了允许的最大文件大小
	if writeLen > l.max() {
		// 如果超过最大文件大小，返回错误信息
		return 0, fmt.Errorf("写入长度 %d 超过最大文件大小 %d", writeLen, l.max())
	}

	// 检查当前日志文件是否未打开
	if l.file == nil {
		// 如果文件未打开，尝试打开现有文件或创建新文件
		if err = l.openExistingOrNew(len(p)); err != nil {
			// 若打开或创建文件失败，返回错误
			return 0, err
		}
	}

	// 检查写入数据后文件大小是否会超过最大限制
	if l.size+writeLen > l.max() {
		// 如果会超过最大限制，执行日志轮转操作
		if rotateErr := l.rotate(); rotateErr != nil {
			// 若轮转操作失败，返回错误
			return 0, rotateErr
		}
	}

	// 向日志文件写入数据
	n, err = l.file.Write(p)
	// 更新当前日志文件的大小
	l.size += int64(n)

	// 返回写入的字节数和可能的错误
	return n, err
}

// Close 是 LogRotateX 类型的 Close 方法，用于关闭日志记录器
func (l *LogRotateX) Close() error {
	// 加锁，确保在并发环境下只有一个 goroutine 能够执行下面的代码
	l.mu.Lock()
	// 在函数结束时解锁，保证锁一定会被释放
	defer l.mu.Unlock()
	// 调用 LogRotateX 的 close 方法，执行具体的关闭操作，并返回可能出现的错误
	return l.close()
}

// close 是 LogRotateX 类型的实例方法，用于关闭日志文件
func (l *LogRotateX) close() error {
	// 检查 l.file 是否为 nil，如果是则直接返回 nil，表示没有文件需要关闭
	if l.file == nil {
		return nil
	}
	// 调用 l.file 的 Close 方法，尝试关闭文件，并将返回的错误赋值给 err 变量
	err := l.file.Close()
	// 将 l.file 置为 nil，表示文件已经关闭
	l.file = nil
	// 返回关闭文件时可能产生的错误
	return err
}

// Rotate 是 LogRotateX 类型的一个方法，用于执行日志文件的轮转操作。
func (l *LogRotateX) Rotate() error {
	// 使用互斥锁来确保日志轮转操作的线程安全。
	l.mu.Lock()
	// 在函数结束时自动解锁，以确保即使在发生错误时也能正确释放锁。
	defer l.mu.Unlock()
	// 调用具体的日志轮转实现方法 rotate，并返回其结果。
	return l.rotate()
}

// rotate 是 LogRotateX 结构体的一个方法，用于执行日志文件的轮转操作。
func (l *LogRotateX) rotate() error {
	// 调用 close 方法关闭当前的日志文件。
	// 如果关闭过程中发生错误，返回该错误。
	if err := l.close(); err != nil {
		return err
	}
	// 调用 openNew 方法打开一个新的日志文件。
	// 如果打开过程中发生错误，返回该错误。
	if err := l.openNew(); err != nil {
		return err
	}
	// 调用 mill 方法处理日志文件的轮转逻辑。
	// 该方法可能包括压缩旧日志文件、删除过期日志文件等操作。
	l.mill()
	// 如果上述操作都成功，返回 nil 表示没有错误。
	return nil
}

// openNew 用于打开一个新的日志文件用于写入，并将旧的日志文件移出当前路径。
func (l *LogRotateX) openNew() error {
	// 确保日志文件所在目录存在，如果不存在则创建
	err := os.MkdirAll(l.dir(), 0755)
	if err != nil {
		return fmt.Errorf("无法创建日志文件所需目录: %s", err)
	}

	// 获取日志文件的完整路径
	name := l.filename()

	// 获取文件的权限模式
	mode := os.FileMode(0600)

	// 获取文件信息
	info, err := os.Stat(name)
	if err == nil {
		// 如果旧日志文件存在，复制其权限模式
		mode = info.Mode()
		// 将现有的日志文件重命名为备份文件
		newname := backupName(name, l.LocalTime)
		if renameErr := os.Rename(name, newname); renameErr != nil {
			return fmt.Errorf("无法重命名日志文件: %s", renameErr)
		}

		// 在非 Linux 系统上，此操作无效
		if chownErr := chown(name, info); chownErr != nil {
			return err
		}
	}

	// 使用 truncate 打开文件，确保文件存在且可写入。
	// 如果文件已存在（可能是其他进程创建的），则清空内容。
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("无法打开新的日志文件: %s", err)
	}
	l.file = f // 将打开的文件赋值给 LogRotateX 的 file 字段
	l.size = 0 // 重置文件大小为 0
	return nil
}

// backupName 根据给定的文件名创建一个新的备份文件名。
// 如果指定使用本地时间，则在文件名和扩展名之间插入本地时间的时间戳；
// 否则插入 UTC 时间的时间戳。
func backupName(name string, local bool) string {
	// 获取文件所在的目录
	dir := filepath.Dir(name)
	// 获取文件的基本名称（包含扩展名）
	filename := filepath.Base(name)
	// 获取文件的扩展名
	ext := filepath.Ext(filename)
	// 获取文件名前缀（去掉扩展名的部分）
	prefix := filename[:len(filename)-len(ext)]

	// 获取当前时间
	t := currentTime()
	// 如果未指定使用本地时间，则将时间转换为 UTC
	if !local {
		t = t.UTC()
	}

	// 格式化时间戳
	timestamp := t.Format(backupTimeFormat)
	// 拼接新的备份文件名
	return filepath.Join(dir, fmt.Sprintf("%s-%s%s", prefix, timestamp, ext))
}

// openExistingOrNew 尝试打开现有的日志文件用于写入。
// 如果文件存在且当前写入操作不会使文件大小超过 MaxSize，则直接打开该文件。
// 如果文件不存在，或者写入操作会使文件大小超过 MaxSize，则创建一个新的日志文件。
func (l *LogRotateX) openExistingOrNew(writeLen int) error {
	// 确保日志文件的大小信息是最新的
	l.mill()

	// 获取日志文件的完整路径
	filename := l.filename()
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		// 如果文件不存在，直接创建新文件
		return l.openNew()
	}
	if err != nil {
		// 如果获取文件信息失败，返回错误
		return fmt.Errorf("获取日志文件信息时出错: %s", err)
	}

	// 检查写入操作是否会超出最大文件大小限制
	if info.Size()+int64(writeLen) >= l.max() {
		// 如果会超出限制，则执行日志文件的轮转操作
		return l.rotate()
	}

	// 以追加模式打开现有日志文件
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		// 如果打开现有日志文件失败，忽略错误并创建新文件
		return l.openNew()
	}
	// 更新日志对象的文件句柄和当前文件大小
	l.file = file
	l.size = info.Size()
	return nil
}

// filename 生成日志文件的名称。
// 如果在 LogRotateX 结构体中指定了 Filename，则直接返回该名称。
// 如果未指定，则根据当前程序的名称生成一个默认的日志文件名，并将其存储在系统的临时目录中。
func (l *LogRotateX) filename() string {
	// 如果已经指定了日志文件名，则直接返回
	if l.Filename != "" {
		return l.Filename
	}
	// 生成默认的日志文件名，格式为：程序名-logrotatex.log
	name := filepath.Base(os.Args[0]) + "-logrotatex.log"
	// 将日志文件存储在系统的临时目录中
	return filepath.Join(os.TempDir(), name)
}

// millRunOnce 执行一次日志文件的压缩和清理操作。
// 如果启用了压缩功能，则对旧的日志文件进行压缩。
// 同时，根据 MaxBackups 和 MaxAge 的设置，移除过期的日志文件。
func (l *LogRotateX) millRunOnce() error {
	// 如果没有设置保留备份数量、最大日志文件年龄，且未启用压缩功能，则直接返回
	if l.MaxBackups == 0 && l.MaxAge == 0 && !l.Compress {
		return nil
	}

	// 获取所有旧的日志文件信息
	files, err := l.oldLogFiles()
	if err != nil {
		return err
	}

	// 定义需要压缩和移除的日志文件列表
	var compress, remove []logInfo

	// 如果设置了最大保留备份数量
	if l.MaxBackups > 0 && l.MaxBackups < len(files) {
		preserved := make(map[string]bool) // 用于记录保留的日志文件
		var remaining []logInfo            // 保留的日志文件列表
		for _, f := range files {
			// 如果是压缩文件，则忽略压缩后缀，只保留原始文件名
			fn := f.Name()
			fn = strings.TrimSuffix(fn, compressSuffix)
			preserved[fn] = true

			// 如果超出最大保留数量，则将多余的文件加入移除列表
			if len(preserved) > l.MaxBackups {
				remove = append(remove, f)
			} else {
				remaining = append(remaining, f)
			}
		}
		files = remaining // 更新文件列表为保留的文件
	}
	// 如果设置了最大日志文件年龄
	if l.MaxAge > 0 {
		diff := time.Duration(int64(24*time.Hour) * int64(l.MaxAge)) // 计算最大年龄对应的时间差
		cutoff := currentTime().Add(-1 * diff)                       // 计算截止时间

		var remaining []logInfo // 保留的日志文件列表
		for _, f := range files {
			// 如果文件的时间戳早于截止时间，则加入移除列表
			if f.timestamp.Before(cutoff) {
				remove = append(remove, f)
			} else {
				remaining = append(remaining, f)
			}
		}
		files = remaining // 更新文件列表为保留的文件
	}

	// 如果启用了压缩功能
	if l.Compress {
		for _, f := range files {
			// 如果文件未被压缩，则加入压缩列表
			if !strings.HasSuffix(f.Name(), compressSuffix) {
				compress = append(compress, f)
			}
		}
	}

	// 移除过期的日志文件
	for _, f := range remove {
		errRemove := os.Remove(filepath.Join(l.dir(), f.Name()))
		if err == nil && errRemove != nil {
			err = errRemove
		}
	}
	// 压缩未压缩的日志文件
	for _, f := range compress {
		fn := filepath.Join(l.dir(), f.Name())
		errCompress := compressLogFile(fn, fn+compressSuffix)
		if err == nil && errCompress != nil {
			err = errCompress
		}
	}

	return err
}

// millRun 在一个独立的 goroutine 中运行，用于管理日志文件的压缩和清理操作。
// 当日志文件发生轮转时，会触发该 goroutine 执行一次清理操作。
func (l *LogRotateX) millRun() {
	for range l.millCh {
		// 执行一次日志文件的压缩和清理操作
		_ = l.millRunOnce()
	}
}

// mill 负责在日志文件轮转后执行压缩和清理操作。
// 如果尚未启动管理 goroutine，则会启动它。
func (l *LogRotateX) mill() {
	l.startMill.Do(func() {
		// 创建一个缓冲通道，用于触发日志文件的压缩和清理操作
		l.millCh = make(chan bool, 1)
		// 启动一个独立的 goroutine 来执行日志文件的压缩和清理操作
		go l.millRun()
	})
	// 向通道发送一个信号，触发一次日志文件的压缩和清理操作
	select {
	case l.millCh <- true:
	default:
	}
}

// oldLogFiles 返回存储在当前日志文件所在目录中的所有备份日志文件列表，
// 并按修改时间（ModTime）对这些文件进行排序。
func (l *LogRotateX) oldLogFiles() ([]logInfo, error) {
	// 读取日志文件所在目录中的所有文件
	files, err := os.ReadDir(l.dir())
	if err != nil {
		return nil, fmt.Errorf("无法读取日志文件目录: %s", err)
	}
	logFiles := []logInfo{}

	// 获取日志文件的前缀和扩展名
	prefix, ext := l.prefixAndExt()

	for _, f := range files {
		if f.IsDir() {
			// 如果是目录，则跳过
			continue
		}

		// 跳过当前正在写入的日志文件
		if f.Name() == filepath.Base(l.filename()) {
			continue
		}

		// 获取文件的信息
		info, err := f.Info()
		if err != nil {
			continue
		}

		// 尝试从文件名中解析时间戳（未压缩文件）
		if t, err := l.timeFromName(f.Name(), prefix, ext); err == nil {
			logFiles = append(logFiles, logInfo{t, info})
			continue
		}
		// 尝试从文件名中解析时间戳（压缩文件）
		if t, err := l.timeFromName(f.Name(), prefix, ext+compressSuffix); err == nil {
			logFiles = append(logFiles, logInfo{t, info})
			continue
		}
		// 如果无法解析时间戳，则说明该文件不是由 logrotatex 生成的备份文件
	}

	// 按文件的修改时间对日志文件进行排序
	sort.Sort(byFormatTime(logFiles))

	return logFiles, nil
}

// timeFromName 从文件名中提取格式化的时间戳。
// 通过移除文件名的前缀和扩展名，避免文件名混淆 time.Parse 的解析结果。
func (l *LogRotateX) timeFromName(filename, prefix, ext string) (time.Time, error) {
	// 检查文件名是否以指定的前缀开头
	if !strings.HasPrefix(filename, prefix) {
		return time.Time{}, errors.New("前缀不匹配")
	}
	// 检查文件名是否以指定的扩展名结尾
	if !strings.HasSuffix(filename, ext) {
		return time.Time{}, errors.New("扩展名不匹配")
	}
	// 提取时间戳部分
	ts := filename[len(prefix) : len(filename)-len(ext)]
	// 解析时间戳
	return time.Parse(backupTimeFormat, ts)
}

// max 返回日志文件在轮转前的最大大小（以字节为单位）。
func (l *LogRotateX) max() int64 {
	// 如果未设置最大大小，则使用默认值
	if l.MaxSize == 0 {
		return int64(defaultMaxSize * megabyte)
	}
	// 将最大大小从 MB 转换为字节
	return int64(l.MaxSize) * int64(megabyte)
}

// dir 返回当前日志文件所在的目录路径。
func (l *LogRotateX) dir() string {
	return filepath.Dir(l.filename())
}

// prefixAndExt 从 LogRotateX 的日志文件名中提取文件名部分和扩展名部分。
// 文件名部分是去掉扩展名后的部分，扩展名部分是文件的后缀。
func (l *LogRotateX) prefixAndExt() (prefix, ext string) {
	filename := filepath.Base(l.filename())          // 获取日志文件的基本名称
	ext = filepath.Ext(filename)                     // 提取文件的扩展名
	prefix = filename[:len(filename)-len(ext)] + "-" // 提取文件名部分并添加分隔符
	return prefix, ext
}

// compressLogFile 压缩指定的日志文件，并在成功后删除原始未压缩的日志文件。
func compressLogFile(src, dst string) (err error) {
	// 打开源日志文件
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("无法打开日志文件: %v", err)
	}
	defer f.Close()

	// 获取源日志文件的状态信息
	fi, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("无法获取日志文件的状态信息: %v", err)
	}

	// 设置目标压缩文件的所有者信息（如果需要）
	if chownErr := chown(dst, fi); chownErr != nil {
		return fmt.Errorf("无法设置压缩日志文件的所有者: %v", err)
	}

	// 如果目标压缩文件已存在，假设这是之前尝试压缩时创建的
	gzf, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fi.Mode())
	if err != nil {
		return fmt.Errorf("无法打开压缩日志文件: %v", err)
	}
	defer gzf.Close()

	// 创建 gzip 压缩写入器
	gz := gzip.NewWriter(gzf)

	defer func() {
		// 如果在压缩过程中发生错误，删除已创建的压缩文件
		if err != nil {
			os.Remove(dst)
			err = fmt.Errorf("压缩日志文件失败: %v", err)
		}
	}()

	// 将源文件的内容复制到 gzip 压缩写入器中
	if _, err := io.Copy(gz, f); err != nil {
		return err
	}
	// 关闭 gzip 压缩写入器
	if err := gz.Close(); err != nil {
		return err
	}
	// 关闭目标压缩文件
	if err := gzf.Close(); err != nil {
		return err
	}

	// 关闭源日志文件
	if err := f.Close(); err != nil {
		return err
	}
	// 删除原始未压缩的日志文件
	if err := os.Remove(src); err != nil {
		return err
	}

	return nil
}

// logInfo 是一个便捷结构体，用于返回文件名及其嵌入的时间戳。
type logInfo struct {
	timestamp   time.Time // 文件名中嵌入的时间戳
	os.FileInfo           // 嵌入的 os.FileInfo 结构体
}

// byFormatTime 是一个自定义的排序类型，用于按文件名中格式化的时间对日志文件进行排序。
type byFormatTime []logInfo

// Less 方法用于比较两个日志文件的时间戳，按时间从新到旧排序。
func (b byFormatTime) Less(i, j int) bool {
	return b[i].timestamp.After(b[j].timestamp)
}

// Swap 方法用于交换两个日志文件的位置。
func (b byFormatTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

// Len 方法返回日志文件列表的长度。
func (b byFormatTime) Len() int {
	return len(b)
}
