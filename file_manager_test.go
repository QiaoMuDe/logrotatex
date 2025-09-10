package logrotatex

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockFileInfo 实现os.FileInfo接口用于测试
type mockFileInfo struct {
	name string
	size int64
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m mockFileInfo) ModTime() time.Time { return time.Now() }
func (m mockFileInfo) IsDir() bool        { return false }
func (m mockFileInfo) Sys() interface{}   { return nil }

// 创建测试用的logInfo
func createTestLogInfo(name string, timestamp time.Time) logInfo {
	return logInfo{
		timestamp: timestamp,
		FileInfo:  mockFileInfo{name: name, size: 1024},
	}
}

// 创建测试用的LogRotateX实例
func createTestLogRotateX(maxBackups, maxAge int) *LogRotateX {
	return &LogRotateX{
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
	}
}

// 测试场景1: 只按数量保留（MaxBackups>0, MaxAge=0）
func TestGetFilesToRemove_OnlyByCount(t *testing.T) {
	now := time.Now()

	// 创建测试文件列表（按时间从新到旧排序）
	files := []logInfo{
		createTestLogInfo("app-2024-01-05.log", now.Add(-1*time.Hour)), // 最新
		createTestLogInfo("app-2024-01-04.log", now.Add(-2*time.Hour)),
		createTestLogInfo("app-2024-01-03.log", now.Add(-3*time.Hour)),
		createTestLogInfo("app-2024-01-02.log", now.Add(-4*time.Hour)),
		createTestLogInfo("app-2024-01-01.log", now.Add(-5*time.Hour)), // 最旧
	}

	tests := []struct {
		name           string
		maxBackups     int
		expectedKeep   int
		expectedRemove []string
	}{
		{
			name:           "保留最新3个文件",
			maxBackups:     3,
			expectedKeep:   3,
			expectedRemove: []string{"app-2024-01-02.log", "app-2024-01-01.log"},
		},
		{
			name:           "保留数量大于文件总数",
			maxBackups:     10,
			expectedKeep:   5,
			expectedRemove: []string{},
		},
		{
			name:           "保留1个文件",
			maxBackups:     1,
			expectedKeep:   1,
			expectedRemove: []string{"app-2024-01-04.log", "app-2024-01-03.log", "app-2024-01-02.log", "app-2024-01-01.log"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := createTestLogRotateX(tt.maxBackups, 0)
			remove := l.getFilesToRemove(files)

			// 验证删除文件数量
			if len(remove) != len(tt.expectedRemove) {
				t.Errorf("期望删除 %d 个文件，实际删除 %d 个", len(tt.expectedRemove), len(remove))
			}

			// 验证删除的具体文件
			removeNames := make(map[string]bool)
			for _, f := range remove {
				removeNames[f.Name()] = true
			}

			for _, expectedName := range tt.expectedRemove {
				if !removeNames[expectedName] {
					t.Errorf("期望删除文件 %s，但未找到", expectedName)
				}
			}
		})
	}
}

// 测试场景2: 只按天数保留（MaxBackups=0, MaxAge>0）
func TestGetFilesToRemove_OnlyByAge(t *testing.T) {
	now := time.Now()

	// 创建测试文件列表（跨越多天）
	files := []logInfo{
		createTestLogInfo("app-2024-01-10.log", now.Add(-1*time.Hour)),  // 今天，保留
		createTestLogInfo("app-2024-01-09.log", now.Add(-25*time.Hour)), // 1天前，保留
		createTestLogInfo("app-2024-01-08.log", now.Add(-49*time.Hour)), // 2天前，保留
		createTestLogInfo("app-2024-01-07.log", now.Add(-73*time.Hour)), // 3天前，删除
		createTestLogInfo("app-2024-01-06.log", now.Add(-97*time.Hour)), // 4天前，删除
	}

	tests := []struct {
		name           string
		maxAge         int
		expectedRemove []string
	}{
		{
			name:           "保留最近3天",
			maxAge:         3,
			expectedRemove: []string{"app-2024-01-07.log", "app-2024-01-06.log"},
		},
		{
			name:           "保留最近1天",
			maxAge:         1,
			expectedRemove: []string{"app-2024-01-09.log", "app-2024-01-08.log", "app-2024-01-07.log", "app-2024-01-06.log"},
		},
		{
			name:           "保留最近7天",
			maxAge:         7,
			expectedRemove: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := createTestLogRotateX(0, tt.maxAge)
			remove := l.getFilesToRemove(files)

			// 验证删除文件数量
			if len(remove) != len(tt.expectedRemove) {
				t.Errorf("期望删除 %d 个文件，实际删除 %d 个", len(tt.expectedRemove), len(remove))
			}

			// 验证删除的具体文件
			removeNames := make(map[string]bool)
			for _, f := range remove {
				removeNames[f.Name()] = true
			}

			for _, expectedName := range tt.expectedRemove {
				if !removeNames[expectedName] {
					t.Errorf("期望删除文件 %s，但未找到", expectedName)
				}
			}
		})
	}
}

// 测试场景3: 数量+天数组合（MaxBackups>0, MaxAge>0）
func TestGetFilesToRemove_ByCountAndAge(t *testing.T) {
	now := time.Now()

	// 创建测试文件列表（每天多个文件）
	files := []logInfo{
		// 今天的文件
		createTestLogInfo("app-2024-01-10T09-00-00.log", now.Add(-1*time.Hour)),
		createTestLogInfo("app-2024-01-10T06-00-00.log", now.Add(-4*time.Hour)),
		createTestLogInfo("app-2024-01-10T03-00-00.log", now.Add(-7*time.Hour)),

		// 1天前的文件
		createTestLogInfo("app-2024-01-09T18-00-00.log", now.Add(-25*time.Hour)),
		createTestLogInfo("app-2024-01-09T12-00-00.log", now.Add(-31*time.Hour)),
		createTestLogInfo("app-2024-01-09T06-00-00.log", now.Add(-37*time.Hour)),

		// 2天前的文件
		createTestLogInfo("app-2024-01-08T15-00-00.log", now.Add(-49*time.Hour)),

		// 4天前的文件（超过3天，应该被删除）
		createTestLogInfo("app-2024-01-06T10-00-00.log", now.Add(-97*time.Hour)),
	}

	tests := []struct {
		name           string
		maxBackups     int
		maxAge         int
		expectedRemove []string
		description    string
	}{
		{
			name:       "保留3天内每天最新2个文件",
			maxBackups: 2,
			maxAge:     3,
			expectedRemove: []string{
				"app-2024-01-10T03-00-00.log", // 今天第3个文件
				"app-2024-01-09T06-00-00.log", // 1天前第3个文件
				"app-2024-01-06T10-00-00.log", // 超过3天
			},
			description: "先筛选3天内文件，再每天保留最新2个",
		},
		{
			name:       "保留2天内每天最新1个文件",
			maxBackups: 1,
			maxAge:     2,
			expectedRemove: []string{
				"app-2024-01-10T06-00-00.log", // 今天第2个文件
				"app-2024-01-10T03-00-00.log", // 今天第3个文件
				"app-2024-01-09T12-00-00.log", // 1天前第2个文件
				"app-2024-01-09T06-00-00.log", // 1天前第3个文件
				"app-2024-01-08T15-00-00.log", // 超过2天
				"app-2024-01-06T10-00-00.log", // 超过2天
			},
			description: "先筛选2天内文件，再每天保留最新1个",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := createTestLogRotateX(tt.maxBackups, tt.maxAge)
			remove := l.getFilesToRemove(files)

			t.Logf("测试场景: %s", tt.description)
			t.Logf("配置: MaxBackups=%d, MaxAge=%d", tt.maxBackups, tt.maxAge)

			// 验证删除文件数量
			if len(remove) != len(tt.expectedRemove) {
				t.Errorf("期望删除 %d 个文件，实际删除 %d 个", len(tt.expectedRemove), len(remove))
				t.Logf("实际删除的文件:")
				for _, f := range remove {
					t.Logf("  - %s", f.Name())
				}
			}

			// 验证删除的具体文件
			removeNames := make(map[string]bool)
			for _, f := range remove {
				removeNames[f.Name()] = true
			}

			for _, expectedName := range tt.expectedRemove {
				if !removeNames[expectedName] {
					t.Errorf("期望删除文件 %s，但未找到", expectedName)
				}
			}
		})
	}
}

// 测试边界情况
func TestGetFilesToRemove_EdgeCases(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		files      []logInfo
		maxBackups int
		maxAge     int
		expected   int
	}{
		{
			name:       "空文件列表",
			files:      []logInfo{},
			maxBackups: 3,
			maxAge:     7,
			expected:   0,
		},
		{
			name: "没有清理规则",
			files: []logInfo{
				createTestLogInfo("app.log", now),
			},
			maxBackups: 0,
			maxAge:     0,
			expected:   0,
		},
		{
			name: "单个文件，按数量保留",
			files: []logInfo{
				createTestLogInfo("app.log", now),
			},
			maxBackups: 1,
			maxAge:     0,
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := createTestLogRotateX(tt.maxBackups, tt.maxAge)
			remove := l.getFilesToRemove(tt.files)

			if len(remove) != tt.expected {
				t.Errorf("期望删除 %d 个文件，实际删除 %d 个", tt.expected, len(remove))
			}
		})
	}
}

// TestFileScanning_Performance 测试文件扫描性能优化效果
func TestFileScanning_Performance(t *testing.T) {
	logsDir := "logs"
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("创建logs目录失败: %v", err)
	}
	defer func() { _ = os.RemoveAll(logsDir) }()

	// 创建大量测试文件
	fileCount := 1000
	prefix := "app"
	ext := ".log"

	t.Logf("创建 %d 个测试文件...", fileCount)

	// 创建测试文件
	for i := 0; i < fileCount; i++ {
		timestamp := time.Now().Add(-time.Duration(i) * time.Hour)
		filename := fmt.Sprintf("%s_%s%s", prefix, timestamp.Format(backupTimeFormat), ext)
		filePath := filepath.Join(logsDir, filename)

		file, createErr := os.Create(filePath)
		if createErr != nil {
			t.Fatalf("创建测试文件失败: %v", createErr)
		}
		if _, writeErr := fmt.Fprintf(file, "测试数据 %d", i); writeErr != nil {
			if closeErr := file.Close(); closeErr != nil {
				t.Logf("关闭文件时出错: %v", closeErr)
			}
			t.Fatalf("写入测试文件失败: %v", writeErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			t.Fatalf("关闭测试文件失败: %v", closeErr)
		}
	}

	// 创建LogRotateX实例
	logPath := filepath.Join(logsDir, "app.log")
	logger := &LogRotateX{
		Filename:   logPath,
		MaxSize:    1,
		MaxBackups: 10,
		MaxAge:     30,
	}

	// 性能测试
	t.Run("优化后的文件扫描性能", func(t *testing.T) {
		start := time.Now()

		// 执行多次扫描以获得平均性能
		iterations := 10
		for i := 0; i < iterations; i++ {
			files, err := logger.oldLogFiles()
			if err != nil {
				t.Fatalf("文件扫描失败: %v", err)
			}

			if i == 0 {
				t.Logf("扫描到 %d 个日志文件", len(files))
			}
		}

		elapsed := time.Since(start)
		avgTime := elapsed / time.Duration(iterations)

		t.Logf("总耗时: %v", elapsed)
		t.Logf("平均单次扫描耗时: %v", avgTime)
		t.Logf("处理 %d 个文件的平均性能: %.2f 文件/毫秒",
			fileCount, float64(fileCount)/float64(avgTime.Nanoseconds())*1000000)

		// 性能断言：平均单次扫描应该在合理时间内完成
		if avgTime > 100*time.Millisecond {
			t.Errorf("文件扫描性能不达标，期望 < 100ms，实际: %v", avgTime)
		}
	})
}
