package logrotatex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewLogRotateX 测试 NewLogRotateX 构造函数
func TestNewLogRotateX(t *testing.T) {
	// 创建logs目录用于测试
	logsDir := "logs"
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("创建logs目录失败: %v", err)
	}
	defer func() { _ = os.RemoveAll(logsDir) }()

	tests := []struct {
		name        string
		filename    string
		shouldPanic bool
		panicMsg    string
		checkFunc   func(*LogRotateX) // 用于验证创建的实例
	}{
		{
			name:        "正常路径创建成功",
			filename:    filepath.Join(logsDir, "test.log"),
			shouldPanic: false,
			checkFunc: func(l *LogRotateX) {
				if l.MaxSize != 10 {
					t.Errorf("期望 MaxSize = 10, 实际 = %d", l.MaxSize)
				}
				if l.MaxAge != 0 {
					t.Errorf("期望 MaxAge = 0, 实际 = %d", l.MaxAge)
				}
				if l.MaxBackups != 0 {
					t.Errorf("期望 MaxBackups = 0, 实际 = %d", l.MaxBackups)
				}
				if !l.LocalTime {
					t.Error("期望 LocalTime = true")
				}
				if l.Compress {
					t.Error("期望 Compress = false")
				}
				if l.FilePerm != defaultFilePerm {
					t.Errorf("期望 FilePerm = %o, 实际 = %o", defaultFilePerm, l.FilePerm)
				}
				// 验证路径是否被正确清理
				if !strings.HasSuffix(l.Filename, "test.log") {
					t.Errorf("期望文件名包含 test.log, 实际 = %s", l.Filename)
				}
			},
		},
		{
			name:        "相对路径创建成功",
			filename:    "logs/app.log",
			shouldPanic: false,
			checkFunc: func(l *LogRotateX) {
				// 验证相对路径被转换为绝对路径
				if !filepath.IsAbs(l.Filename) {
					t.Errorf("期望绝对路径, 实际 = %s", l.Filename)
				}
				if !strings.HasSuffix(l.Filename, "app.log") {
					t.Errorf("期望文件名包含 app.log, 实际 = %s", l.Filename)
				}
			},
		},
		{
			name:        "嵌套目录自动创建",
			filename:    filepath.Join(logsDir, "deep", "nested", "dir", "test.log"),
			shouldPanic: false,
			checkFunc: func(l *LogRotateX) {
				// 验证目录是否被创建
				dir := filepath.Dir(l.Filename)
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					t.Errorf("期望目录被创建: %s", dir)
				}
			},
		},
		{
			name:        "路径遍历攻击 - 应该panic",
			filename:    "../../../etc/passwd",
			shouldPanic: true,
			panicMsg:    "路径遍历攻击",
		},
		{
			name:        "空路径 - 应该panic",
			filename:    "",
			shouldPanic: true,
			panicMsg:    "路径不能为空",
		},
		{
			name:        "危险字符 - 应该panic",
			filename:    "test<script>.log",
			shouldPanic: true,
			panicMsg:    "文件名包含非法字符",
		},
		{
			name:        "系统敏感目录 - 应该panic",
			filename:    "/etc/test.log", // 只在Unix系统上测试，Windows会跳过
			shouldPanic: true,
			panicMsg:    "不允许访问系统敏感目录",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 跳过Windows上的Unix敏感目录测试
			if tt.name == "系统敏感目录 - 应该panic" && filepath.Separator == '\\' {
				t.Skip("跳过Windows上的Unix敏感目录测试")
				return
			}

			if tt.shouldPanic {
				// 测试应该panic的情况
				defer func() {
					if r := recover(); r != nil {
						panicStr := r.(string)
						if !strings.Contains(panicStr, tt.panicMsg) {
							t.Errorf("期望panic消息包含 '%s', 实际 = '%s'", tt.panicMsg, panicStr)
						}
					} else {
						t.Errorf("期望panic但没有发生")
					}
				}()
				NewLogRotateX(tt.filename)
			} else {
				// 测试正常创建的情况
				logger := NewLogRotateX(tt.filename)
				if logger == nil {
					t.Fatal("期望创建成功但返回nil")
				}

				// 执行自定义检查函数
				if tt.checkFunc != nil {
					tt.checkFunc(logger)
				}

				// 清理测试文件
				// 调用Close方法确保所有资源被释放
				if err := logger.Close(); err != nil {
					t.Logf("关闭logger失败: %v", err)
				}
			}
		})
	}
}

// TestNewLogRotateX_DirectoryPermissions 测试目录权限
func TestNewLogRotateX_DirectoryPermissions(t *testing.T) {
	logsDir := "logs"
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("创建logs目录失败: %v", err)
	}
	defer func() { _ = os.RemoveAll(logsDir) }()
	testPath := filepath.Join(logsDir, "subdir", "test.log")

	logger := NewLogRotateX(testPath)
	defer func() {
		// 调用Close方法确保所有资源被释放
		if err := logger.Close(); err != nil {
			t.Logf("关闭logger失败: %v", err)
		}
	}()

	// 检查目录是否被创建
	dir := filepath.Dir(logger.Filename)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("目录应该被创建: %v", err)
	}

	// 检查目录权限 (仅在Unix系统上检查精确权限)
	if filepath.Separator != '\\' { // 非Windows系统
		if info.Mode().Perm() != defaultDirPerm {
			t.Errorf("期望目录权限 0700, 实际 = %o", info.Mode().Perm())
		}
	} else {
		// Windows系统只检查目录是否可访问
		if !info.IsDir() {
			t.Error("期望创建的是目录")
		}
	}
}

// TestNewLogRotateX_PathCleaning 测试路径清理功能
func TestNewLogRotateX_PathCleaning(t *testing.T) {
	logsDir := "logs"
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		t.Fatalf("创建logs目录失败: %v", err)
	}
	defer func() { _ = os.RemoveAll(logsDir) }()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "多余斜杠清理",
			input:    filepath.Join(logsDir, "logs//app.log"),
			expected: "logs/app.log",
		},
		{
			name:     "当前目录清理",
			input:    filepath.Join(logsDir, "logs/./app.log"),
			expected: "logs/app.log",
		},
		{
			name:     "混合路径清理",
			input:    filepath.Join(logsDir, "logs//./app.log"),
			expected: "logs/app.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogRotateX(tt.input)
			defer func() {
				// 调用Close方法确保所有资源被释放
				if err := logger.Close(); err != nil {
					t.Logf("关闭logger失败: %v", err)
				}
			}()

			// 验证路径被正确清理 (跨平台兼容)
			expectedPath := filepath.FromSlash(tt.expected)
			if !strings.HasSuffix(logger.Filename, expectedPath) {
				t.Errorf("期望路径后缀 '%s', 实际路径 = '%s'", expectedPath, logger.Filename)
			}
		})
	}
}

// BenchmarkNewLogRotateX 性能基准测试
func BenchmarkNewLogRotateX(b *testing.B) {
	logsDir := "logs"
	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		b.Fatalf("创建logs目录失败: %v", err)
	}
	defer func() { _ = os.RemoveAll(logsDir) }()
	testPath := filepath.Join(logsDir, "bench.log")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger := NewLogRotateX(testPath)
		// 调用Close方法确保所有资源被释放
		if err := logger.Close(); err != nil {
			b.Logf("关闭logger失败: %v", err)
		}
		if err := os.Remove(logger.Filename); err != nil && !os.IsNotExist(err) {
			b.Logf("删除文件失败: %v", err)
		}
	}
}
