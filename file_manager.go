// file_manager.go 实现了logrotatex包的文件管理功能。
// 该文件提供了日志文件的创建、打开、关闭、重命名等基础操作，
// 以及文件状态检查和元数据管理功能，是日志轮转系统的核心组件。

package logrotatex

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gitee.com/MM-Q/comprx"
)

// scanConfig 日志文件扫描配置
type scanConfig struct {
	prefix        string             // 日志文件前缀
	ext           string             // 日志文件扩展名
	compressedExt string             // 压缩文件扩展名
	timestampSet  map[time.Time]bool // 时间戳去重集合 (nil 表示不检查)
}

// cleanupSync 同步执行日志文件的压缩和清理操作。
// 根据 MaxBackups、MaxAge 和 Compress 配置处理旧日志文件。
//
// 返回值:
//   - error: 操作失败时返回错误，否则返回 nil
func (l *LogRotateX) cleanupSync() error {
	// 若已关闭，直接跳过
	if l.closed.Load() {
		return nil
	}

	// 快速路径: 如果没有设置保留数量, 保留天数, 且不启用压缩, 则直接返回
	if l.MaxFiles <= 0 && l.MaxAge <= 0 && !l.Compress {
		return nil
	}

	// 获取所有旧的日志文件信息 (按时间戳降序排列)
	files, err := l.oldLogFiles()
	if err != nil {
		return fmt.Errorf("logrotatex: failed to get old log files: %w", err)
	}

	// 定义需要压缩和移除的日志文件列表
	var compress, remove []logInfo

	// 获取需要删除的文件
	remove = l.getFilesToRemove(files)

	// 处理压缩文件
	if l.Compress {
		for _, f := range files {
			// 如果文件未被压缩，则加入压缩列表
			if !strings.HasSuffix(f.Name(), l.CompressType.String()) {
				compress = append(compress, f)
			}
		}
	}

	// 执行清理操作
	return l.executeCleanup(remove, compress)
}

// executeCleanup 执行文件删除和压缩操作
//
// 参数:
//   - remove: 需要删除的文件列表
//   - compress: 需要压缩的文件列表
//
// 返回值:
//   - error: 操作失败时返回错误，否则返回 nil
func (l *LogRotateX) executeCleanup(remove, compress []logInfo) error {
	// 若已关闭，直接跳过
	if l.closed.Load() {
		return nil
	}

	// 收集所有错误
	var errors []error

	// 执行文件移除操作
	if len(remove) > 0 {
		for _, f := range remove {
			// 获取文件的完整路径
			filePath := l.getFilePath(f)

			// 移除文件
			if err := os.Remove(filePath); err != nil {
				errors = append(errors, fmt.Errorf("logrotatex: failed to remove log file %s: %w", filePath, err))
			}
		}
	}

	// 执行文件压缩操作
	if len(compress) > 0 {
		for _, f := range compress {
			// 获取文件的完整路径
			filePath := l.getFilePath(f)
			// 基础文件名（不包含扩展名）
			baseName := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
			// 压缩文件路径, 格式: 父目录/基础文件名.压缩类型
			compressPath := filepath.Join(filepath.Dir(filePath), baseName+l.CompressType.String())

			// 创建压缩配置
			opts := comprx.Options{
				CompressionLevel:      comprx.CompressionLevelDefault, // 默认压缩级别
				OverwriteExisting:     true,                           // 覆盖已存在的压缩文件
				ProgressEnabled:       false,                          // 不显示进度条
				ProgressStyle:         comprx.ProgressStyleDefault,    // 默认进度条样式
				DisablePathValidation: false,                          // 禁用路径验证
			}

			// 压缩文件
			if err := comprx.PackOptions(compressPath, filePath, opts); err != nil {
				errors = append(errors, fmt.Errorf("logrotatex: failed to compress log file %s: %w", filePath, err))
				continue // 压缩失败就跳过，保留原文件
			}

			// 删除原文件
			if err := os.Remove(filePath); err != nil {
				errors = append(errors, fmt.Errorf("logrotatex: failed to delete original file %s: %w", filePath, err))
			}
		}
	}

	// 清理空日期目录
	l.cleanupEmptyDirs()

	// 如果有错误，返回聚合错误
	if len(errors) > 0 {
		var errMsg strings.Builder
		errMsg.WriteString("logrotatex: multiple errors occurred during cleanup execution:\n")
		for i, err := range errors {
			errMsg.WriteString(fmt.Sprintf("  %d. %v\n", i+1, err))
		}
		return fmt.Errorf("%s", errMsg.String())
	}

	return nil
}

// getFilePath 获取日志文件的完整路径
// 支持日期目录模式和传统模式
//
// 参数:
//   - f: 日志文件信息
//
// 返回值:
//   - string: 文件的完整路径
func (l *LogRotateX) getFilePath(f logInfo) string {
	// 如果是日期目录模式，文件路径需要包含日期目录
	if l.DateDirLayout {
		// 从文件名中解析日期
		timestamp := f.timestamp
		dateDir := timestamp.Format("2006-01-02")
		return filepath.Join(l.dir(), dateDir, f.Name())
	}
	// 传统模式
	return filepath.Join(l.dir(), f.Name())
}

// cleanupEmptyDirs 清理空的日期目录
// 当某个日期目录下的所有文件都被删除后，删除该空目录
func (l *LogRotateX) cleanupEmptyDirs() {
	// 如果未启用日期目录模式，直接返回
	if !l.DateDirLayout {
		return
	}

	// 读取根目录
	files, err := os.ReadDir(l.dir())
	if err != nil {
		return
	}

	for _, f := range files {
		// 跳过文件和当前日志文件
		if !f.IsDir() {
			continue
		}

		dirPath := filepath.Join(l.dir(), f.Name())

		// 检查目录是否为空
		dirFiles, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		// 如果目录为空，删除它
		if len(dirFiles) == 0 {
			_ = os.Remove(dirPath)
		}
	}
}

// cleanupAsync 触发异步清理 (单协程、合并触发)
func (l *LogRotateX) cleanupAsync() {
	// 关闭后不再调度
	if l.closed.Load() {
		return
	}

	// 快速路径: 无需清理直接返回
	if l.MaxFiles <= 0 && l.MaxAge <= 0 && !l.Compress {
		return
	}

	// 尝试启动单协程 (CAS 0->1)
	if l.cleanupRunning.CompareAndSwap(false, true) {
		l.wg.Go(func() {
			l.runCleanupLoop()
		})
		return
	}

	// 已在运行: 真正快速失败，只做一次从 false->true 的标记
	_ = l.rerunNeeded.CompareAndSwap(false, true)
}

// 单协程清理循环: 每轮都现查现算，错误仅打印
func (l *LogRotateX) runCleanupLoop() {
	defer func() {
		// panic 保护，防止 wg 和 running 状态失配
		if r := recover(); r != nil {
			fmt.Printf("logrotatex: panic in async cleanup: %v\n", r)
		}

		l.cleanupRunning.Store(false) // 退出时重置运行状态
	}()

	for {
		// 关闭后退出
		if l.closed.Load() {
			return
		}

		// 1) 最新文件状态
		files, err := l.oldLogFiles()
		if err != nil {
			fmt.Printf("logrotatex: failed to get old log files: %v\n", err)

			// 如果没有新的触发需求，直接退出循环，避免空转
			if !l.rerunNeeded.Load() {
				break
			}

			// 有新的触发需求: 轻微退避，避免忙等
			time.Sleep(150 * time.Millisecond)

			// 消费掉一次“需要重跑”的信号并继续下一轮
			_ = l.rerunNeeded.Swap(false)
			continue
		}

		// 2) 删除列表
		var remove []logInfo
		if files != nil {
			remove = l.getFilesToRemove(files)
		}

		// 3) 压缩列表
		var compress []logInfo
		if l.Compress && files != nil {
			for _, f := range files {
				if !strings.HasSuffix(f.Name(), l.CompressType.String()) {
					compress = append(compress, f)
				}
			}
		}

		// 4) 执行清理
		if err := l.executeCleanup(remove, compress); err != nil {
			fmt.Printf("logrotatex: async cleanup error: %v\n", err)
		}

		// 5) 是否重跑 (合并触发: 多次触发只续跑一轮)
		if l.rerunNeeded.Swap(false) {
			continue
		}
		break
	}
}

// processLogFile 处理单个日志文件，提取为通用逻辑
//
// 参数:
//   - f: 文件条目
//   - cfg: 扫描配置
//
// 返回值:
//   - logInfo: 日志文件信息 (如果有效)
//   - bool: 是否有效
func (l *LogRotateX) processLogFile(f os.DirEntry, cfg scanConfig) (logInfo, bool) {
	fileName := f.Name()

	// 快速前缀检查
	if cfg.prefix != "" && !strings.HasPrefix(fileName, cfg.prefix) {
		return logInfo{}, false
	}

	// 确定文件类型和扩展名
	var targetExt string
	if strings.HasSuffix(fileName, cfg.compressedExt) {
		targetExt = cfg.compressedExt
	} else if strings.HasSuffix(fileName, cfg.ext) {
		targetExt = cfg.ext
	} else {
		return logInfo{}, false
	}

	// 解析时间戳
	timestamp, parseErr := l.fastTimeFromName(fileName, cfg.prefix, targetExt)
	if parseErr != nil {
		return logInfo{}, false
	}

	// 检查时间戳重复 (如果配置了 timestampSet)
	if cfg.timestampSet != nil && cfg.timestampSet[timestamp] {
		return logInfo{}, false
	}

	// 获取文件信息
	info, err := f.Info()
	if err != nil {
		return logInfo{}, false
	}

	return logInfo{timestamp, info}, true
}

// oldLogFiles 返回当前目录中的所有备份日志文件，按时间戳排序。
// 支持日期目录模式，单线程扫描所有目录。
//
// 返回值:
//   - []logInfo: 备份日志文件列表
//   - error: 读取目录失败时返回错误
func (l *LogRotateX) oldLogFiles() ([]logInfo, error) {
	// 读取日志文件所在目录中的所有文件
	files, err := os.ReadDir(l.dir())
	if err != nil {
		return nil, fmt.Errorf("logrotatex: unable to read log file directory: %w", err)
	}

	// 如果目录为空，直接返回
	if len(files) == 0 {
		return nil, nil
	}

	// 获取日志文件的前缀和扩展名 (只计算一次)
	prefix, ext := l.prefixAndExt()
	currentFileName := filepath.Base(l.filename())
	compressedExt := ext + l.CompressType.String()

	// 预估容量，避免频繁扩容
	estimatedCapacity := len(files) / 4
	if estimatedCapacity < 10 {
		estimatedCapacity = 10
	}

	logFiles := make([]logInfo, 0, estimatedCapacity)
	timestampSet := make(map[time.Time]bool, estimatedCapacity)

	// 创建扫描配置
	cfg := scanConfig{
		prefix:        prefix,
		ext:           ext,
		compressedExt: compressedExt,
		timestampSet:  timestampSet,
	}

	// 扫描根目录和日期目录
	for _, f := range files {
		// 跳过当前日志文件
		if f.Name() == currentFileName {
			continue
		}

		if f.IsDir() {
			// 扫描日期目录
			dirPath := filepath.Join(l.dir(), f.Name())
			dirFiles, err := l.scanDateDir(dirPath, cfg)
			if err != nil {
				continue // 跳过无法读取的目录
			}
			// 合并结果
			for _, df := range dirFiles {
				logFiles = append(logFiles, df)
				timestampSet[df.timestamp] = true
			}
		} else {
			// 处理根目录文件 (支持混合模式)
			if logInfo, ok := l.processLogFile(f, cfg); ok {
				logFiles = append(logFiles, logInfo)
				timestampSet[logInfo.timestamp] = true
			}
		}
	}

	// 按时间戳排序 (从新到旧)
	sort.Sort(byFormatTime(logFiles))

	return logFiles, nil
}

// scanDateDir 扫描单个日期目录，返回其中的日志文件
//
// 参数:
//   - dirPath: 日期目录路径
//   - cfg: 扫描配置
//
// 返回值:
//   - []logInfo: 日志文件列表
//   - error: 读取目录失败时返回错误
func (l *LogRotateX) scanDateDir(dirPath string, cfg scanConfig) ([]logInfo, error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var logFiles []logInfo

	for _, f := range files {
		// 跳过子目录 (只处理一层)
		if f.IsDir() {
			continue
		}

		// 处理文件 (processLogFile 内部会检查时间戳重复)
		if logInfo, ok := l.processLogFile(f, cfg); ok {
			logFiles = append(logFiles, logInfo)
		}
	}

	return logFiles, nil
}

// getFilesToRemove 根据配置的清理规则，返回需要删除的文件列表
//
// 支持三种清理场景:
//  1. 数量+天数组合 (MaxBackups>0, MaxAge>0)
//  2. 只按数量保留 (MaxBackups>0, MaxAge=0)
//  3. 只按天数保留 (MaxBackups=0, MaxAge>0)
//
// 参数:
//   - files: 所有日志文件信息列表
//
// 返回值:
//   - []logInfo: 需要删除的日志文件列表
func (l *LogRotateX) getFilesToRemove(files []logInfo) []logInfo {
	// 快速失败: 没有文件
	if len(files) == 0 {
		return nil
	}

	// 快速失败: 没有设置任何清理规则
	hasBackupRule := l.MaxFiles > 0
	hasAgeRule := l.MaxAge > 0

	if !hasBackupRule && !hasAgeRule {
		return nil
	}

	var keep []logInfo

	// 场景1: 数量+天数组合
	if hasBackupRule && hasAgeRule {
		keep = l.keepByDaysAndCount(files, l.MaxAge, l.MaxFiles)
		return l.calculateRemoveList(files, keep)
	}

	// 场景2: 只按数量保留
	if hasBackupRule {
		if l.MaxFiles >= len(files) {
			return nil // 文件数量不超过限制，无需删除
		}
		keep = files[:l.MaxFiles]
		return l.calculateRemoveList(files, keep)
	}

	// 场景3: 只按天数保留
	if hasAgeRule {
		cutoffTime := currentTime().Add(-time.Duration(l.MaxAge) * 24 * time.Hour)
		for _, f := range files {
			// 如果文件时间早于最大保留天数，则保留
			if f.timestamp.After(cutoffTime) {
				keep = append(keep, f)
			}
		}
		return l.calculateRemoveList(files, keep)
	}

	return nil
}

// keepByDaysAndCount 实现场景1的逻辑: 先按天数筛选，再每天保留指定数量
//
// 参数:
//   - files: 所有日志文件信息列表
//   - maxAge: 最大保留天数
//   - maxBackups: 每天保留的最大文件数量
//
// 返回值:
//   - []logInfo: 需要保留的日志文件列表
func (l *LogRotateX) keepByDaysAndCount(files []logInfo, maxAge, maxBackups int) []logInfo {
	cutoffTime := currentTime().Add(-time.Duration(maxAge) * 24 * time.Hour)

	// 按天分组
	dayGroups := make(map[string][]logInfo)
	for _, f := range files {
		if f.timestamp.After(cutoffTime) {
			dayKey := f.timestamp.Format("2006-01-02") // 按日期分组
			dayGroups[dayKey] = append(dayGroups[dayKey], f)
		}
	}

	var keep []logInfo
	for _, dayFiles := range dayGroups {
		// 对每天的文件按时间排序 (从新到旧)
		sort.Slice(dayFiles, func(i, j int) bool {
			return dayFiles[i].timestamp.After(dayFiles[j].timestamp)
		})

		// 每天保留最新的maxBackups个文件
		keepCount := maxBackups
		if keepCount > len(dayFiles) {
			keepCount = len(dayFiles)
		}

		// 保留最新的文件
		keep = append(keep, dayFiles[:keepCount]...)
	}

	return keep
}

// calculateRemoveList 计算需要删除的文件列表
//
// 参数:
//   - allFiles: 所有日志文件信息列表
//   - keepFiles: 需要保留的日志文件列表
//
// 返回值:
//   - []logInfo: 需要删除的日志文件列表
func (l *LogRotateX) calculateRemoveList(allFiles, keepFiles []logInfo) []logInfo {
	if len(keepFiles) == 0 {
		return allFiles // 没有要保留的，全部删除
	}

	// 创建保留文件的映射表
	keepSet := make(map[string]bool, len(keepFiles))
	for _, f := range keepFiles {
		keepSet[f.Name()] = true
	}

	// 找出需要删除的文件
	var remove []logInfo
	for _, f := range allFiles {
		if !keepSet[f.Name()] {
			remove = append(remove, f)
		}
	}

	return remove
}

// fastTimeFromName 从文件名中快速解析时间戳 (纯数字格式优化版) 。
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

	// 增强的边界检查
	if startPos < 0 || endPos > len(filename) || startPos >= endPos {
		return time.Time{}, fmt.Errorf("logrotatex: invalid filename format: %s", filename)
	}

	// 检查时间戳长度
	timestampLen := endPos - startPos
	if timestampLen != expectedTimestampLen {
		return time.Time{}, fmt.Errorf("logrotatex: invalid timestamp length %d, expected %d in file: %s",
			timestampLen, expectedTimestampLen, filename)
	}

	// 提取时间戳字符串
	timestampStr := filename[startPos:endPos]

	// 快速验证: 确保都是数字
	if !isAllDigits(timestampStr) {
		return time.Time{}, fmt.Errorf("logrotatex: timestamp contains non-digit characters: %s", timestampStr)
	}

	// 解析时间戳
	t, err := time.Parse(backupTimeFormat, timestampStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("logrotatex: failed to parse timestamp %s: %w", timestampStr, err)
	}

	return t, nil
}

// isAllDigits 快速检查字符串是否全为数字
func isAllDigits(s string) bool {
	// 空字符串被认为不是全数字
	if len(s) == 0 {
		return false
	}

	// 检查每个字符是否为数字
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
