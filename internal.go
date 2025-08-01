// internal.go 文件包含了日志轮转功能的核心实现。
//
// 该文件提供了以下核心功能：
// - 日志文件的打开、关闭和轮转操作
// - 日志文件的备份和命名管理
// - 旧日志文件的清理和压缩
// - 日志文件大小和数量的控制
//
// 主要的实现包括：
// - rotate: 执行日志文件轮转的核心逻辑
// - openNew: 创建新的日志文件
// - backupName: 生成备份文件名
// - millRunOnce: 清理和压缩旧日志文件
// - oldLogFiles: 获取旧日志文件列表
// - compressLogFile: 压缩日志文件
package logrotatex

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// backupTimeFormat 是备份文件的时间戳格式, 用于在文件名中嵌入时间信息。
	backupTimeFormat = "2006-01-02T15-04-05.000"

	// compressSuffix 是压缩文件的后缀, 用于标识已压缩的日志文件。
	compressSuffix = ".zip"

	// defaultMaxSize 是日志文件的最大默认大小(单位: MB), 在未明确设置时使用此值。
	defaultMaxSize = 10

	// 4KB - 最小缓冲区
	minBufferSize = 4 * 1024

	// 128KB - 最大缓冲区，避免过度内存使用
	maxBufferSize = 128 * 1024
)

// close 是 LogRotateX 类型的实例方法, 用于安全地关闭当前打开的日志文件。
//
// 该方法执行以下操作：
// 1. 检查是否有文件需要关闭(l.file != nil)
// 2. 调用文件的 Close 方法执行实际关闭操作
// 3. 将文件句柄置为 nil, 防止重复关闭
// 4. 返回关闭过程中可能产生的错误
//
// 返回值：
//   - 如果成功关闭文件或没有文件需要关闭, 返回 nil
//   - 如果关闭文件时发生错误, 返回相应的 error
func (l *LogRotateX) close() error {
	// 检查 l.file 是否为 nil, 如果是则直接返回 nil, 表示没有文件需要关闭
	if l.file == nil {
		return nil
	}
	// 调用 l.file 的 Close 方法, 尝试关闭文件, 并将返回的错误赋值给 err 变量
	err := l.file.Close()

	// 将 l.file 置为 nil, 表示文件已经关闭
	l.file = nil

	// 返回关闭文件时可能产生的错误
	return err
}

// rotate 是 LogRotateX 结构体的一个方法，用于执行日志文件的轮转操作。
//
// 日志轮转是日志管理的核心功能，当当前日志文件达到一定条件时（如大小限制），
// 需要将其重命名为备份文件并创建一个新的日志文件继续写入。
//
// 该方法执行以下操作：
// 1. 调用 close 方法关闭当前的日志文件
// 2. 调用 openNew 方法打开一个新的日志文件
// 3. 调用 mill 方法处理日志文件的后续逻辑（如压缩、清理等）
//
// 返回值：
//   - 如果所有操作都成功完成，返回 nil
//   - 如果在关闭当前文件或打开新文件时发生错误，返回相应的 error
func (l *LogRotateX) rotate() error {
	// 调用 close 方法关闭当前的日志文件。
	// 如果关闭过程中发生错误, 返回该错误。
	if err := l.close(); err != nil {
		return err
	}
	// 调用 openNew 方法打开一个新的日志文件。
	// 如果打开过程中发生错误, 返回该错误。
	if err := l.openNew(); err != nil {
		return err
	}
	// 调用 mill 方法处理日志文件的轮转逻辑。
	// 该方法可能包括压缩旧日志文件、删除过期日志文件等操作。
	l.mill()
	// 如果上述操作都成功, 返回 nil 表示没有错误。
	return nil
}

// openNew 用于打开一个新的日志文件用于写入, 并将旧的日志文件移出当前路径。
// 该方法确保日志目录存在，处理现有文件的重命名，并创建新的日志文件。
//
// 返回值：
//   - 如果成功打开新的日志文件，返回 nil
func (l *LogRotateX) openNew() error {
	// 确保日志文件所在目录存在，使用更安全的目录权限
	// 如果目录不存在则创建，如果已存在则不执行任何操作
	if err := os.MkdirAll(l.dir(), 0700); err != nil {
		return fmt.Errorf("logrotatex: 无法创建日志文件所需目录: %w", err)
	}

	// 获取日志文件的完整路径
	name := l.filename()

	// 获取文件的权限模式
	mode := l.FilePerm
	// 如果未设置FilePerm, 则使用默认值0600
	if mode == 0 {
		mode = os.FileMode(0600)
	}

	// 获取文件信息
	info, err := os.Stat(name)
	if err == nil {
		// 如果旧日志文件存在, 复制其权限模式
		mode = info.Mode()

		// 将现有的日志文件重命名为备份文件
		newname := backupName(name, l.LocalTime)
		if renameErr := os.Rename(name, newname); renameErr != nil {
			return fmt.Errorf("logrotatex: 无法重命名日志文件: %w", renameErr)
		}

		// 在非 Linux 系统上, 此操作无效
		if chownErr := chown(name, info); chownErr != nil {
			return fmt.Errorf("logrotatex: 无法设置文件所有者: %w", chownErr)
		}
	}

	// 使用 truncate 打开文件, 确保文件存在且可写入。
	// 如果文件已存在（可能是其他进程创建的）, 则清空内容。
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("logrotatex: 无法打开新的日志文件: %w", err)
	}

	l.file = f // 将打开的文件赋值给 LogRotateX 的 file 字段
	l.size = 0 // 重置文件大小为 0
	return nil
}

// backupName 根据给定的文件名创建一个新的备份文件名。
// 如果指定使用本地时间, 则在文件名和扩展名之间插入本地时间的时间戳；
// 否则插入 UTC 时间的时间戳。
//
// 参数:
//   - name: 原始文件名
//   - local: 是否使用本地时间, true 表示使用本地时间, false 表示使用 UTC 时间
//
// 返回值:
//   - 新的备份文件名
func backupName(name string, local bool) string {
	// 获取文件所在的目录
	dir := filepath.Dir(name)
	// 获取文件的基本名称（包含扩展名）
	filename := filepath.Base(name)

	// 更安全地处理文件名和扩展名
	ext := filepath.Ext(filename)
	prefix := strings.TrimSuffix(filename, ext)

	// 如果文件名以点号结尾但没有扩展名（例如"logfile."），确保正确处理
	if len(ext) > 0 && ext == filename {
		// 处理纯扩展名文件（例如".gitignore"）
		prefix = ""
	} else if len(prefix) == 0 && len(ext) > 0 {
		// 处理以点号开头的文件（例如".logfile"）
		prefix = ext
		ext = ""
	}

	// 获取当前时间
	t := currentTime()
	// 如果未指定使用本地时间, 则将时间转换为 UTC
	if !local {
		t = t.UTC()
	}

	// 格式化时间戳
	timestamp := t.Format(backupTimeFormat)

	// 生成新的备份文件名
	return filepath.Join(dir, fmt.Sprintf("%s_%s%s", prefix, timestamp, ext))
}

// openExistingOrNew 确保日志文件已正确打开。
// 如果文件已打开则直接返回；如果未打开则尝试打开现有文件或创建新文件。
// 如果文件存在且当前写入操作不会使文件大小超过 MaxSize, 则直接打开该文件。
// 如果文件不存在, 或者写入操作会使文件大小超过 MaxSize, 则创建一个新的日志文件。
//
// 参数:
//   - writeLen: 预计写入的数据长度
//
// 返回值:
//   - 如果文件已打开或成功打开现有文件或创建新文件, 返回 nil
func (l *LogRotateX) openExistingOrNew(writeLen int) error {
	// 如果文件已经打开，直接返回
	if l.file != nil {
		return nil
	}

	// 确保日志文件的大小信息是最新的
	l.mill()

	// 获取日志文件的完整路径
	filename := l.filename()
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		// 如果文件不存在, 直接创建新文件
		return l.openNew()
	}
	if err != nil {
		// 如果获取文件信息失败, 返回错误
		return fmt.Errorf("logrotatex: 获取日志文件信息时出错: %w", err)
	}

	// 检查写入操作是否会超出最大文件大小限制
	if info.Size()+int64(writeLen) > l.max() {
		// 如果会超出限制, 则执行日志文件的轮转操作
		return l.rotate()
	}

	// 以追加模式打开现有日志文件
	// 使用更安全的默认权限0600
	filePerm := l.FilePerm
	if filePerm == 0 {
		filePerm = os.FileMode(0600) // 修复：使用更安全的默认权限
	}
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, filePerm)
	if err != nil {
		// 如果打开现有日志文件失败, 则通过openNew创建新文件
		return l.openNew()
	}
	// 更新日志对象的文件句柄和当前文件大小
	l.file = file
	l.size = info.Size()
	return nil
}

// filename 生成日志文件的名称。
// 如果在 LogRotateX 结构体中指定了 Filename, 则直接返回该名称。
// 如果未指定, 则根据当前程序的名称生成一个默认的日志文件名, 并将其存储在系统的临时目录中。
//
// 返回值:
//   - 如果指定了 Filename, 返回该值
//   - 否则返回默认的日志文件名
func (l *LogRotateX) filename() string {
	// 如果已经指定了日志文件名, 则直接返回
	if l.Filename != "" {
		return l.Filename
	}
	// 生成默认的日志文件名, 格式为: 程序名_logrotatex.log
	name := filepath.Base(os.Args[0]) + "_logrotatex.log"

	// 将日志文件存储在系统的临时目录中
	return filepath.Join(os.TempDir(), name)
}

// millRunOnce 执行一次日志文件的压缩和清理操作。
// 根据配置的 MaxBackups、MaxAge 和 Compress 选项，执行以下操作：
// 1. 获取所有符合条件的旧日志文件
// 2. 保留最新的 MaxBackups 个日志文件，移除多余文件
// 3. 移除超过 MaxAge 天的旧日志文件
// 4. 压缩未压缩的日志文件（如果启用了压缩）
//
// 返回:
//   - 操作过程中发生的错误，如果有多个错误，将返回聚合错误
func (l *LogRotateX) millRunOnce() error {
	// 快速路径: 如果没有设置备份保留数量, 备份保留天数, 启用压缩功能, 则直接返回
	if l.MaxBackups == 0 && l.MaxAge == 0 && !l.Compress {
		return nil
	}

	// 获取所有旧的日志文件信息（按时间戳降序排列）
	files, err := l.oldLogFiles()
	if err != nil {
		return fmt.Errorf("logrotatex: 获取旧日志文件失败: %w", err)
	}

	// 定义需要压缩和移除的日志文件列表
	var compress, remove []logInfo

	// 处理备份保留数量规则
	if l.MaxBackups > 0 && l.MaxBackups < len(files) {
		// 直接保留最新的 MaxBackups 个文件，移除多余的文件
		remove = append(remove, files[l.MaxBackups:]...)
		files = files[:l.MaxBackups]
	}

	// 处理备份保留天数规则
	if l.MaxAge > 0 {
		// 计算截止时间戳
		maxAgeDuration := time.Duration(int64(24*time.Hour) * int64(l.MaxAge)) // 计算当前时间
		cutoffTime := currentTime().Add(-maxAgeDuration)                       // 计算截止时间

		var remaining []logInfo
		for _, f := range files {
			// 如果文件时间戳早于截止时间，则加入保留列表
			if f.timestamp.After(cutoffTime) {
				// 添加到保留列表
				remaining = append(remaining, f)
			} else {
				// 否则加入移除列表
				remove = append(remove, f)
			}
		}
		files = remaining
	}

	// 处理压缩文件
	if l.Compress {
		for _, f := range files {
			// 如果文件未被压缩，则加入压缩列表
			if !strings.HasSuffix(f.Name(), compressSuffix) {
				compress = append(compress, f)
			}
		}
	}

	// 收集所有错误
	var errors []error

	// 执行文件移除操作
	for _, f := range remove {
		// 合并路径
		filePath := filepath.Join(l.dir(), f.Name())

		// 移除文件
		if err := os.Remove(filePath); err != nil {
			errors = append(errors, fmt.Errorf("logrotatex: 移除日志文件 %s 失败: %w", filePath, err))
		}
	}

	// 执行文件压缩操作
	for _, f := range compress {
		// 合并路径
		filePath := filepath.Join(l.dir(), f.Name())
		// 合并压缩文件名
		compressPath := filePath + compressSuffix

		// 压缩文件
		if err := compressLogFile(filePath, compressPath); err != nil {
			errors = append(errors, fmt.Errorf("logrotatex: 压缩日志文件 %s 失败: %w", filePath, err))
		}
	}

	// 如果有错误，返回聚合错误
	if len(errors) > 0 {
		var errMsg strings.Builder
		errMsg.WriteString("logrotatex: millRunOnce 执行过程中发生多个错误:\n")
		for i, err := range errors {
			errMsg.WriteString(fmt.Sprintf("  %d. %v\n", i+1, err))
		}
		return fmt.Errorf("logrotatex: %s", errMsg.String())
	}

	return nil
}

// millRun 在一个独立的 goroutine 中运行, 用于管理日志文件的压缩和清理操作。
// 当日志文件发生轮转时, 会触发该 goroutine 执行一次清理操作。
// 该goroutine会在收到done信号时安全退出。
func (l *LogRotateX) millRun() {
	defer func() {
		// 确保在goroutine退出时进行清理
		if r := recover(); r != nil {
			fmt.Printf("logrotatex: millRun panic recovered: %v\n", r)
		}
	}()

	for {
		select {
		case <-l.millCh:
			// 执行一次日志文件的压缩和清理操作
			if err := l.millRunOnce(); err != nil {
				// 使用更好的错误记录方式
				fmt.Printf("logrotatex: millRunOnce 执行失败: %v\n", err)
			}
		case <-l.millDone:
			// 收到退出信号，安全退出goroutine
			return
		}
	}
}

// mill 负责在日志文件轮转后执行压缩和清理操作。
// 如果尚未启动管理 goroutine, 则会启动它。
func (l *LogRotateX) mill() {
	l.startMill.Do(func() {
		// 创建通道用于goroutine通信
		l.millCh = make(chan bool, 1)
		l.millDone = make(chan struct{})
		l.millStarted.Store(true)
		// 启动一个独立的 goroutine 来执行日志文件的压缩和清理操作
		go l.millRun()
	})

	// 如果goroutine已经停止，则不发送信号
	if !l.millStarted.Load() {
		return
	}

	// 向通道发送一个信号, 触发一次日志文件的压缩和清理操作
	select {
	case l.millCh <- true:
	default:
		// 通道已满, 说明已有清理操作在等待执行
	}
}

// oldLogFiles 返回存储在当前日志文件所在目录中的所有备份日志文件列表,
// 并按修改时间(ModTime)对这些文件进行排序
// 使用优化的文件扫描策略，减少不必要的系统调用和内存分配
//
// 返回值:
//   - []logInfo: 包含所有备份日志文件的列表
//   - error: 如果在读取日志文件目录时发生错误, 则返回相应的错误信息
func (l *LogRotateX) oldLogFiles() ([]logInfo, error) {
	// 读取日志文件所在目录中的所有文件
	files, err := os.ReadDir(l.dir())
	if err != nil {
		return nil, fmt.Errorf("logrotatex: 无法读取日志文件目录: %w", err)
	}

	// 如果目录为空，直接返回
	if len(files) == 0 {
		return nil, nil
	}

	// 获取日志文件的前缀和扩展名（只计算一次）
	prefix, ext := l.prefixAndExt()
	currentFileName := filepath.Base(l.filename())
	compressedExt := ext + compressSuffix

	// 第一遍扫描：快速过滤，只统计符合条件的文件数量
	candidateCount := 0
	for _, f := range files {
		if f.IsDir() || f.Name() == currentFileName {
			continue
		}
		fileName := f.Name()
		if (prefix == "" || strings.HasPrefix(fileName, prefix)) &&
			(strings.HasSuffix(fileName, ext) || strings.HasSuffix(fileName, compressedExt)) {
			candidateCount++
		}
	}

	// 如果没有候选文件，直接返回
	if candidateCount == 0 {
		return nil, nil
	}

	// 精确分配内存，避免重新分配
	logFiles := make([]logInfo, 0, candidateCount)
	processedTimestamps := make(map[time.Time]bool, candidateCount)

	// 第二遍扫描：处理符合条件的文件
	for _, f := range files {
		// 快速过滤：跳过目录和当前日志文件
		if f.IsDir() || f.Name() == currentFileName {
			continue
		}

		fileName := f.Name()

		// 快速过滤：检查文件名前缀和扩展名
		if prefix != "" && !strings.HasPrefix(fileName, prefix) {
			continue
		}
		if !strings.HasSuffix(fileName, ext) && !strings.HasSuffix(fileName, compressedExt) {
			continue
		}

		// 尝试从文件名中解析时间戳（避免重复字符串操作）
		var timestamp time.Time
		var parseErr error

		if strings.HasSuffix(fileName, compressedExt) {
			timestamp, parseErr = l.timeFromName(fileName, prefix, compressedExt)
		} else {
			timestamp, parseErr = l.timeFromName(fileName, prefix, ext)
		}

		// 如果解析失败或时间戳已处理过，跳过
		if parseErr != nil || processedTimestamps[timestamp] {
			continue
		}

		// 只有在确认需要时才获取文件信息（延迟获取）
		info, err := f.Info()
		if err != nil {
			continue
		}

		logFiles = append(logFiles, logInfo{timestamp, info})
		processedTimestamps[timestamp] = true
	}

	// 按文件的修改时间对日志文件进行排序
	sort.Sort(byFormatTime(logFiles))

	return logFiles, nil
}

// timeFromName 从文件名中提取格式化的时间戳。
// 通过移除文件名的前缀和扩展名, 避免文件名混淆 time.Parse 的解析结果。
//
// 参数:
//   - filename: 文件名
//   - prefix: 文件名前缀
//   - ext: 文件名扩展名
//
// 返回值:
//   - time.Time: 解析得到的时间戳
//   - error: 解析错误
func (l *LogRotateX) timeFromName(filename, prefix, ext string) (time.Time, error) {
	// 如果前缀为空，则直接从文件名开始解析时间戳
	if prefix == "" {
		// 检查文件名是否以指定的扩展名结尾
		if !strings.HasSuffix(filename, ext) {
			return time.Time{}, fmt.Errorf("logrotatex: 扩展名不匹配")
		}
		// 提取时间戳部分
		ts := filename[:len(filename)-len(ext)]
		// 解析时间戳
		return time.Parse(backupTimeFormat, ts)
	}
	// 检查文件名是否以指定的前缀开头
	if !strings.HasPrefix(filename, prefix) {
		return time.Time{}, fmt.Errorf("logrotatex: 前缀不匹配")
	}
	// 检查文件名是否以指定的扩展名结尾
	if !strings.HasSuffix(filename, ext) {
		return time.Time{}, fmt.Errorf("logrotatex: 扩展名不匹配")
	}

	// 计算时间戳的起始和结束位置
	startPos := len(prefix) + 1 // 跳过前缀和分隔符 "_"
	endPos := len(filename) - len(ext)

	// 检查边界条件，防止数组越界
	if startPos >= len(filename) || startPos >= endPos {
		return time.Time{}, fmt.Errorf("logrotatex: 文件名格式不正确")
	}

	// 提取时间戳部分
	ts := filename[startPos:endPos]
	// 解析时间戳
	return time.Parse(backupTimeFormat, ts)
}

// max 返回日志文件在轮转前的最大大小（以字节为单位）。
// 如果未设置最大大小（即 l.MaxSize 为 0），则使用默认值 defaultMaxSize * megabyte。
// 否则，将 l.MaxSize 从 MB 转换为字节。
//
// 返回值:
//   - int64: 日志文件的最大大小（以字节为单位）
func (l *LogRotateX) max() int64 {
	// 如果未设置最大大小, 则使用默认值
	if l.MaxSize == 0 {
		return int64(defaultMaxSize * megabyte)
	}
	// 将最大大小从 MB 转换为字节
	return int64(l.MaxSize) * int64(megabyte)
}

// dir 返回当前日志文件所在的目录路径。
// 通过调用 filepath.Dir(l.filename()) 获取日志文件的目录部分。
//
// 返回值:
//   - string: 日志文件所在的目录路径
func (l *LogRotateX) dir() string {
	return filepath.Dir(l.filename())
}

// prefixAndExt 从 LogRotateX 的日志文件名中提取文件名部分和扩展名部分。
// 文件名部分是去掉扩展名后的部分, 扩展名部分是文件的后缀。
//
// 返回值:
//   - prefix: 文件名部分
//   - ext: 扩展名部分
func (l *LogRotateX) prefixAndExt() (prefix, ext string) {
	filename := filepath.Base(l.filename())    // 获取日志文件的基本名称
	ext = filepath.Ext(filename)               // 提取文件的扩展名
	prefix = filename[:len(filename)-len(ext)] // 提取文件名部分并添加分隔符

	// 如果文件名没有前缀，则使用程序名作为前缀
	if prefix == "" {
		prefix = filepath.Base(os.Args[0])
	}

	return prefix, ext
}

// compressLogFile 压缩指定的日志文件, 并在成功后删除原始未压缩的日志文件。
// 参数:
//
//	src - 源日志文件路径
//	dst - 目标压缩文件路径(应包含.zip扩展名)
//
// 返回值:
//
//	error - 操作过程中遇到的错误
func compressLogFile(src, dst string) (err error) {
	// 打开源日志文件
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("logrotatex: 无法打开日志文件: %w", err)
	}

	// 获取源日志文件的状态信息
	fi, err := os.Stat(src)
	if err != nil {
		_ = f.Close() // 确保在错误时关闭文件
		return fmt.Errorf("logrotatex: 无法获取日志文件的状态信息: %w", err)
	}

	// 创建目标ZIP文件
	zipFile, err := os.Create(dst)
	if err != nil {
		_ = f.Close() // 确保在错误时关闭文件
		return fmt.Errorf("logrotatex: 创建压缩文件失败: %w", err)
	}

	// 立即设置目标压缩文件的所有者信息
	if chownErr := chown(dst, fi); chownErr != nil {
		_ = f.Close()
		_ = zipFile.Close()
		_ = os.Remove(dst) // 清理未完成的文件
		return fmt.Errorf("logrotatex: 无法设置压缩日志文件的所有者: %w", chownErr)
	}

	// 创建ZIP写入器
	zipWriter := zip.NewWriter(zipFile)

	// 为源文件创建ZIP文件头
	header, err := zip.FileInfoHeader(fi)
	if err != nil {
		_ = f.Close()
		_ = zipWriter.Close()
		_ = zipFile.Close()
		_ = os.Remove(dst) // 清理未完成的文件
		return fmt.Errorf("logrotatex: 创建ZIP文件头失败: %w", err)
	}

	// 设置文件头的名称为源文件名
	header.Name = filepath.Base(src)

	// 设置压缩方法为Deflate
	header.Method = zip.Deflate

	// 创建ZIP文件写入器
	fileWriter, err := zipWriter.CreateHeader(header)
	if err != nil {
		_ = f.Close()
		_ = zipWriter.Close()
		_ = zipFile.Close()
		_ = os.Remove(dst) // 清理未完成的文件
		return fmt.Errorf("logrotatex: 创建ZIP文件写入器失败: %w", err)
	}

	// 根据文件大小设置缓冲区大小
	bufferSize := getBufferSize(fi.Size())

	// 创建带缓冲的读取器
	bufferedReader := bufio.NewReaderSize(f, bufferSize)

	// 使用缓冲区进行文件复制，提高性能
	buffer := make([]byte, bufferSize)
	if _, err := io.CopyBuffer(fileWriter, bufferedReader, buffer); err != nil {
		_ = f.Close()
		_ = zipWriter.Close()
		_ = zipFile.Close()
		_ = os.Remove(dst) // 清理未完成的文件
		return fmt.Errorf("logrotatex: 写入ZIP文件失败: %w", err)
	}

	// 确保所有资源都已关闭
	_ = f.Close()
	_ = zipWriter.Close()
	_ = zipFile.Close()

	// 删除原始未压缩的日志文件
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("logrotatex: 删除原始日志文件失败: %w", err)
	}

	return nil
}

// getBufferSize 根据文件大小和系统内存情况返回合适的缓冲区大小
// 使用自适应算法，在性能和内存使用之间找到平衡点
// 参数:
//
//	fileSize - 文件大小(字节)
//
// 返回值:
//
//	int - 建议的缓冲区大小
func getBufferSize(fileSize int64) int {
	// 对于空文件或极小文件，使用最小缓冲区
	if fileSize <= 0 {
		return minBufferSize
	}

	if fileSize <= minBufferSize {
		return minBufferSize // 确保不会返回小于最小缓冲区的值
	}

	// 自适应算法：缓冲区大小基于文件大小，但限制在合理范围内
	bufferSize := int(fileSize / 16) // 文件大小的1/16作为基础

	// 确保缓冲区大小是4KB的倍数，提高I/O效率
	bufferSize = (bufferSize + 4095) & ^4095

	// 限制在合理范围内，确保不会返回0或负数
	if bufferSize < minBufferSize {
		return minBufferSize
	}
	if bufferSize > maxBufferSize {
		return maxBufferSize
	}

	return bufferSize
}

// logInfo 是一个便捷结构体，用于返回文件名及其嵌入的时间戳。
// 它包含了日志文件的时间戳信息和文件系统信息，用于日志轮转时的文件管理。
type logInfo struct {
	// timestamp 是从文件名中解析出的时间戳
	timestamp time.Time
	// FileInfo 包含文件的基本信息（大小、修改时间等）
	os.FileInfo
}

// byFormatTime 是一个自定义的排序类型, 用于按文件名中格式化的时间对日志文件进行排序。
// 它实现了 sort.Interface 接口，可以被 sort.Sort 函数使用。
type byFormatTime []logInfo

// Less 方法用于比较两个日志文件的时间戳, 按时间从新到旧排序。
// 这意味着索引较小的元素时间戳较新。
// 参数:
//   - i: 第一个元素的索引
//   - j: 第二个元素的索引
//
// 返回值:
//   - bool: 如果第一个元素的时间戳晚于第二个元素的时间戳，则返回 true
func (b byFormatTime) Less(i, j int) bool {
	return b[i].timestamp.After(b[j].timestamp)
}

// Swap 方法用于交换两个日志文件的位置。
// 参数:
//   - i: 第一个元素的索引
//   - j: 第二个元素的索引
func (b byFormatTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

// Len 方法返回日志文件列表的长度。
// 返回值:
//   - int: 日志文件列表的长度
func (b byFormatTime) Len() int {
	return len(b)
}

// validatePath 验证文件路径的安全性，防止路径遍历攻击
// 参数:
//   - path string: 要验证的文件路径
//
// 返回值:
//   - error: 如果路径不安全则返回错误，否则返回 nil
func validatePath(path string) error {
	if path == "" {
		return fmt.Errorf("路径不能为空")
	}

	// 1. 使用 filepath.Clean 清理路径，移除多余的分隔符和相对路径元素
	cleanPath := filepath.Clean(path)

	// 2. 检查是否包含路径遍历攻击模式
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("检测到路径遍历攻击，路径包含 '..' 元素: %s", path)
	}

	// 3. 检查是否为绝对路径中的危险路径
	if filepath.IsAbs(cleanPath) {
		// 获取绝对路径
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return fmt.Errorf("无法获取绝对路径: %w", err)
		}

		// 检查是否试图访问系统敏感目录
		dangerousPaths := []string{
			// === 核心系统目录（绝对不能碰）===
			"/etc",                                   // 系统配置
			"/boot",                                  // 启动文件
			"/usr/bin", "/usr/sbin", "/sbin", "/bin", // 系统可执行文件
			"/proc", "/sys", "/dev", // 虚拟文件系统
			"/root", // 超级用户目录

			// === 系统库目录（高风险）===
			"/lib", "/lib64", "/usr/lib", "/usr/lib64",
			"/lib/modules", // 内核模块

			// === 系统运行时（高风险）===
			"/run", "/var/run", // 运行时文件
			"/var/lock", // 锁文件

			// === 特别危险的系统日志 ===
			"/var/log/kern.log", // 内核日志
			"/var/log/auth.log", // 认证日志
			"/var/log/secure",   // 安全日志
			"/var/log/messages", // 系统消息
			"/var/log/syslog",   // 系统日志

			// === 关键系统文件 ===
			"/etc/passwd", "/etc/shadow", "/etc/group",
			"/etc/sudoers", "/etc/ssh",
		}

		for _, dangerous := range dangerousPaths {
			if strings.HasPrefix(absPath, dangerous) {
				return fmt.Errorf("不允许访问系统敏感目录: %s", absPath)
			}
		}
	}

	// 4. 检查文件名中的危险字符
	filename := filepath.Base(cleanPath)
	if strings.ContainsAny(filename, "<>:\"|?*") {
		return fmt.Errorf("文件名包含非法字符: %s", filename)
	}

	// 5. 检查路径长度限制
	if len(cleanPath) > 4096 {
		return fmt.Errorf("路径长度超过限制 (4096 字符): %d", len(cleanPath))
	}

	return nil
}

// sanitizePath 清理并返回安全的文件路径
// 参数:
//   - path string: 原始文件路径
//
// 返回值:
//   - string: 清理后的安全路径
//   - error: 如果路径不安全则返回错误
func sanitizePath(path string) (string, error) {
	if err := validatePath(path); err != nil {
		return "", err
	}

	// 使用 filepath.Clean 清理路径
	cleanPath := filepath.Clean(path)

	// 如果是相对路径，确保它不会跳出当前工作目录
	if !filepath.IsAbs(cleanPath) {
		// 获取当前工作目录
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("无法获取当前工作目录: %w", err)
		}

		// 将相对路径转换为绝对路径
		absPath := filepath.Join(wd, cleanPath)
		cleanPath = filepath.Clean(absPath)

		// 确保最终路径仍在工作目录下
		if !strings.HasPrefix(cleanPath, wd) {
			return "", fmt.Errorf("路径试图跳出工作目录: %s", path)
		}
	}

	return cleanPath, nil
}
