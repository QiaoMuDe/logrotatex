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
	"gitee.com/MM-Q/comprx/types"
)

// cleanupSync 同步执行日志文件的压缩和清理操作。
// 根据 MaxBackups、MaxAge 和 Compress 配置处理旧日志文件。
//
// 返回值:
//   - error: 操作失败时返回错误，否则返回 nil
func (l *LogRotateX) cleanupSync() error {
	// 快速路径: 如果没有设置保留数量, 保留天数, 且不启用压缩, 则直接返回
	if l.MaxFiles <= 0 && l.MaxAge <= 0 && !l.Compress {
		return nil
	}

	// 获取所有旧的日志文件信息（按时间戳降序排列）
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
			if !strings.HasSuffix(f.Name(), compressSuffix) {
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
	// 收集所有错误
	var errors []error

	// 执行文件移除操作
	if len(remove) > 0 {
		for _, f := range remove {
			// 合并路径
			filePath := filepath.Join(l.dir(), f.Name())

			// 移除文件
			if err := os.Remove(filePath); err != nil {
				errors = append(errors, fmt.Errorf("logrotatex: failed to remove log file %s: %w", filePath, err))
			}
		}
	}

	// 执行文件压缩操作
	if len(compress) > 0 {
		for _, f := range compress {
			// 合并路径
			filePath := filepath.Join(l.dir(), f.Name())
			// 压缩文件名生成
			baseName := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
			compressPath := filepath.Join(l.dir(), baseName+compressSuffix)

			// 创建压缩配置
			opts := comprx.Options{
				CompressionLevel:      types.CompressionLevelDefault, // 默认压缩级别
				OverwriteExisting:     true,                          // 覆盖已存在的压缩文件
				ProgressEnabled:       false,                         // 不显示进度条
				ProgressStyle:         types.ProgressStyleDefault,    // 默认进度条样式
				DisablePathValidation: false,                         // 禁用路径验证
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

// oldLogFiles 返回当前目录中的所有备份日志文件，按时间戳排序。
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

	// 单次扫描: O(n)时间复杂度
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

// getFilesToRemove 根据配置的清理规则，返回需要删除的文件列表
//
// 支持三种清理场景：
//  1. 数量+天数组合（MaxBackups>0, MaxAge>0）
//  2. 只按数量保留（MaxBackups>0, MaxAge=0）
//  3. 只按天数保留（MaxBackups=0, MaxAge>0）
//
// 参数:
//   - files: 所有日志文件信息列表
//
// 返回值:
//   - []logInfo: 需要删除的日志文件列表
func (l *LogRotateX) getFilesToRemove(files []logInfo) []logInfo {
	// 快速失败：没有文件
	if len(files) == 0 {
		return nil
	}

	// 快速失败：没有设置任何清理规则
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
			if f.timestamp.After(cutoffTime) {
				keep = append(keep, f)
			}
		}
		return l.calculateRemoveList(files, keep)
	}

	return nil
}

// keepByDaysAndCount 实现场景1的逻辑：先按天数筛选，再每天保留指定数量
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
		// 每天保留最新的maxBackups个文件
		// dayFiles已经按时间排序（从新到旧）
		keepCount := maxBackups
		if keepCount > len(dayFiles) {
			keepCount = len(dayFiles)
		}
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

// fastTimeFromName 从文件名中快速解析时间戳（纯数字格式优化版）。
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

	// 快速验证：确保都是数字
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
