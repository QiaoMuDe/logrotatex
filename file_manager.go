// file_manager.go 实现了logrotatex包的文件管理功能。
// 该文件提供了日志文件的创建、打开、关闭、重命名等基础操作，
// 以及文件状态检查和元数据管理功能，是日志轮转系统的核心组件。

package logrotatex

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// millRunOnce 执行一次日志文件的压缩和清理操作。
// 根据 MaxBackups、MaxAge 和 Compress 配置处理旧日志文件。
//
// 返回值:
//   - error: 操作失败时返回错误，否则返回 nil
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

// millRun 在独立的 goroutine 中运行，处理日志文件的压缩和清理操作。
// 收到 context 取消信号时安全退出。
func (l *LogRotateX) millRun() {
	defer func() {
		// 标记goroutine已完成
		l.millWg.Done()
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
		case <-l.millCtx.Done():
			// 收到context取消信号，安全退出goroutine
			return
		}
	}
}

// mill 触发日志文件的压缩和清理操作。
// 如果管理 goroutine 未启动则启动它。
func (l *LogRotateX) mill() {
	l.startMill.Do(func() {
		// 创建context用于goroutine生命周期管理
		l.millCtx, l.millCancel = context.WithCancel(context.Background())
		l.millCh = make(chan bool, 1)
		l.millStarted.Store(true)
		l.millWg.Add(1)
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

// oldLogFiles 返回当前目录中的所有备份日志文件，按时间戳排序。
//
// 返回值:
//   - []logInfo: 备份日志文件列表
//   - error: 读取目录失败时返回错误
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

	// 预估容量，避免频繁扩容
	estimatedCapacity := len(files) / 4
	if estimatedCapacity < 10 {
		estimatedCapacity = 10
	}

	logFiles := make([]logInfo, 0, estimatedCapacity)
	timestampSet := make(map[time.Time]bool, estimatedCapacity)

	// 单次扫描：O(n)时间复杂度
	for _, f := range files {
		// 快速过滤：跳过目录和当前日志文件
		if f.IsDir() || f.Name() == currentFileName {
			continue
		}

		fileName := f.Name()

		// 快速前缀检查
		if prefix != "" && !strings.HasPrefix(fileName, prefix) {
			continue
		}

		// 确定文件类型和扩展名
		var targetExt string

		if strings.HasSuffix(fileName, compressedExt) {
			targetExt = compressedExt
		} else if strings.HasSuffix(fileName, ext) {
			targetExt = ext
		} else {
			continue // 不匹配任何扩展名，跳过
		}

		// 解析时间戳（优化版本）
		timestamp, parseErr := l.fastTimeFromName(fileName, prefix, targetExt)
		if parseErr != nil {
			continue
		}

		// 检查时间戳重复（防止重复处理同一时间戳的文件）
		if timestampSet[timestamp] {
			continue
		}

		// 获取文件信息（延迟到确认需要时）
		info, err := f.Info()
		if err != nil {
			continue
		}

		// 添加到结果集
		logFiles = append(logFiles, logInfo{timestamp, info})
		timestampSet[timestamp] = true
	}

	// 按时间戳排序（从新到旧）
	sort.Sort(byFormatTime(logFiles))

	return logFiles, nil
}

// fastTimeFromName 从文件名中快速解析时间戳。
func (l *LogRotateX) fastTimeFromName(filename, prefix, ext string) (time.Time, error) {
	// 计算时间戳的起始和结束位置
	var startPos, endPos int

	if prefix == "" {
		startPos = 0
		endPos = len(filename) - len(ext)
	} else {
		startPos = len(prefix) + 1 // 跳过前缀和分隔符 "_"
		endPos = len(filename) - len(ext)
	}

	// 边界检查
	if startPos >= endPos || startPos < 0 || endPos > len(filename) {
		return time.Time{}, fmt.Errorf("logrotatex: 文件名格式不正确")
	}

	// 直接从计算位置提取时间戳
	timestampStr := filename[startPos:endPos]

	// 解析时间戳
	return time.Parse(backupTimeFormat, timestampStr)
}

// timeFromName 从文件名中提取时间戳。
//
// 参数:
//   - filename: 文件名
//   - prefix: 文件名前缀
//   - ext: 文件扩展名
//
// 返回值:
//   - time.Time: 解析得到的时间戳
//   - error: 解析失败时返回错误
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

// compressLogFile 压缩指定的日志文件，成功后删除原文件。
//
// 参数:
//   - src: 源日志文件路径
//   - dst: 目标压缩文件路径
//
// 返回值:
//   - error: 压缩失败时返回错误，否则返回 nil
func compressLogFile(src, dst string) (err error) {
	// 获取源日志文件的状态信息（在打开文件之前）
	fi, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("logrotatex: 无法获取日志文件的状态信息: %w", err)
	}

	// 打开源日志文件
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("logrotatex: 无法打开日志文件: %w", err)
	}
	// 使用defer确保源文件在函数退出时一定会被关闭
	defer func() { _ = f.Close() }()

	// 创建目标ZIP文件
	zipFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("logrotatex: 创建压缩文件失败: %w", err)
	}
	// 使用defer确保ZIP文件在函数退出时一定会被关闭
	defer func() { _ = zipFile.Close() }()

	// 创建ZIP写入器
	zipWriter := zip.NewWriter(zipFile)
	// 使用defer确保ZIP写入器在函数退出时一定会被关闭
	defer func() { _ = zipWriter.Close() }()

	// 设置目标压缩文件的所有者信息
	if chownErr := chown(dst, fi); chownErr != nil {
		// 确保所有资源都已关闭
		_ = f.Close()
		_ = zipWriter.Close()
		_ = zipFile.Close()
		// 清理未完成的文件
		_ = os.Remove(dst)
		return fmt.Errorf("logrotatex: 无法设置压缩日志文件的所有者: %w", chownErr)
	}

	// 为源文件创建ZIP文件头
	header, err := zip.FileInfoHeader(fi)
	if err != nil {
		// 确保所有资源都已关闭
		_ = f.Close()
		_ = zipWriter.Close()
		_ = zipFile.Close()
		// 清理未完成的文件
		_ = os.Remove(dst)
		return fmt.Errorf("logrotatex: 创建ZIP文件头失败: %w", err)
	}

	// 设置文件头的名称为源文件名
	header.Name = filepath.Base(src)
	// 设置压缩方法为Deflate
	header.Method = zip.Deflate

	// 创建ZIP文件写入器
	fileWriter, err := zipWriter.CreateHeader(header)
	if err != nil {
		// 确保所有资源都已关闭
		_ = f.Close()
		_ = zipWriter.Close()
		_ = zipFile.Close()
		// 清理未完成的文件
		_ = os.Remove(dst)
		return fmt.Errorf("logrotatex: 创建ZIP文件写入器失败: %w", err)
	}

	// 根据文件大小设置缓冲区大小
	bufferSize := getBufferSize(fi.Size())

	// 创建带缓冲的读取器
	bufferedReader := bufio.NewReaderSize(f, bufferSize)

	// 使用缓冲区进行文件复制，提高性能
	buffer := make([]byte, bufferSize)
	if _, err := io.CopyBuffer(fileWriter, bufferedReader, buffer); err != nil {
		// 确保所有资源都已关闭
		_ = f.Close()
		_ = zipWriter.Close()
		_ = zipFile.Close()
		// 清理未完成的文件
		_ = os.Remove(dst)
		return fmt.Errorf("logrotatex: 写入ZIP文件失败: %w", err)
	}

	// 确保所有资源都已关闭
	_ = f.Close()
	_ = zipWriter.Close()
	_ = zipFile.Close()

	// 所有写入操作完成后，删除原始未压缩的日志文件
	// 注意：这里不能使用defer，因为需要在所有文件句柄关闭后才能删除
	// defer函数会在return之前执行，此时文件句柄还未关闭
	// 所以我们在defer中处理删除操作
	defer func() {
		if err == nil {
			_ = os.Remove(src)
		}
	}()

	return nil
}
