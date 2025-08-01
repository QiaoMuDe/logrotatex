package logrotatex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidatePath 测试 validatePath 函数
func TestValidatePath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
		errorMsg    string
	}{
		// 正常路径测试
		{
			name:        "正常相对路径",
			path:        "logs/app.log",
			expectError: false,
		},
		{
			name:        "正常绝对路径",
			path:        "/tmp/app.log",
			expectError: false,
		},
		{
			name:        "正常Windows路径",
			path:        "C:\\logs\\app.log",
			expectError: false,
		},

		// 路径遍历攻击测试
		{
			name:        "路径遍历攻击 - 双点",
			path:        "../../../etc/passwd",
			expectError: true,
			errorMsg:    "路径遍历攻击",
		},
		{
			name:        "路径遍历攻击 - 混合路径",
			path:        "logs/../../../etc/passwd",
			expectError: true,
			errorMsg:    "路径遍历攻击",
		},
		{
			name:        "路径遍历攻击 - 绝对路径",
			path:        "/var/log/../../../etc/passwd",
			expectError: filepath.Separator != '\\', // 仅在非Windows系统上期望错误
			errorMsg:    "路径遍历攻击",
		},

		// 空路径测试
		{
			name:        "空路径",
			path:        "",
			expectError: true,
			errorMsg:    "路径不能为空",
		},

		// 系统敏感目录测试 (仅在Unix系统上测试)
		{
			name:        "访问/etc目录",
			path:        "/etc/test.log",
			expectError: filepath.Separator != '\\', // 仅在非Windows系统上期望错误
			errorMsg:    "不允许访问系统敏感目录",
		},
		{
			name:        "访问/proc目录",
			path:        "/proc/test.log",
			expectError: filepath.Separator != '\\',
			errorMsg:    "不允许访问系统敏感目录",
		},
		{
			name:        "访问/sys目录",
			path:        "/sys/test.log",
			expectError: filepath.Separator != '\\',
			errorMsg:    "不允许访问系统敏感目录",
		},
		{
			name:        "访问/dev目录",
			path:        "/dev/test.log",
			expectError: filepath.Separator != '\\',
			errorMsg:    "不允许访问系统敏感目录",
		},
		{
			name:        "访问/boot目录",
			path:        "/boot/test.log",
			expectError: filepath.Separator != '\\',
			errorMsg:    "不允许访问系统敏感目录",
		},
		{
			name:        "访问/root目录",
			path:        "/root/test.log",
			expectError: filepath.Separator != '\\',
			errorMsg:    "不允许访问系统敏感目录",
		},

		// 危险字符测试
		{
			name:        "文件名包含<字符",
			path:        "test<script.log",
			expectError: true,
			errorMsg:    "文件名包含非法字符",
		},
		{
			name:        "文件名包含>字符",
			path:        "test>script.log",
			expectError: true,
			errorMsg:    "文件名包含非法字符",
		},
		{
			name:        "文件名包含:字符",
			path:        "test:script.log",
			expectError: true,
			errorMsg:    "文件名包含非法字符",
		},
		{
			name:        "文件名包含\"字符",
			path:        "test\"script.log",
			expectError: true,
			errorMsg:    "文件名包含非法字符",
		},
		{
			name:        "文件名包含|字符",
			path:        "test|script.log",
			expectError: true,
			errorMsg:    "文件名包含非法字符",
		},
		{
			name:        "文件名包含?字符",
			path:        "test?script.log",
			expectError: true,
			errorMsg:    "文件名包含非法字符",
		},
		{
			name:        "文件名包含*字符",
			path:        "test*script.log",
			expectError: true,
			errorMsg:    "文件名包含非法字符",
		},

		// 路径长度测试
		{
			name:        "超长路径",
			path:        strings.Repeat("a", 4097) + ".log",
			expectError: true,
			errorMsg:    "路径长度超过限制",
		},
		{
			name:        "正常长度路径",
			path:        strings.Repeat("a", 100) + ".log",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path)

			if tt.expectError {
				if err == nil {
					t.Errorf("期望返回错误，但没有错误")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("期望错误消息包含 '%s', 实际错误 = '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("期望没有错误，但返回错误: %v", err)
				}
			}
		})
	}
}

// TestSanitizePath 测试 sanitizePath 函数
func TestSanitizePath(t *testing.T) {
	// 获取当前工作目录用于测试
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("无法获取工作目录: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		expectError bool
		errorMsg    string
		checkFunc   func(string) bool // 用于验证返回的路径
	}{
		// 正常路径清理测试
		{
			name:        "清理多余斜杠",
			path:        "logs//app.log",
			expectError: false,
			checkFunc: func(result string) bool {
				expected := filepath.FromSlash("logs/app.log")
				return strings.HasSuffix(result, expected)
			},
		},
		{
			name:        "清理当前目录引用",
			path:        "logs/./app.log",
			expectError: false,
			checkFunc: func(result string) bool {
				expected := filepath.FromSlash("logs/app.log")
				return strings.HasSuffix(result, expected)
			},
		},
		{
			name:        "清理混合路径问题",
			path:        "logs//./app.log",
			expectError: false,
			checkFunc: func(result string) bool {
				expected := filepath.FromSlash("logs/app.log")
				return strings.HasSuffix(result, expected)
			},
		},
		{
			name:        "相对路径转绝对路径",
			path:        "app.log",
			expectError: false,
			checkFunc: func(result string) bool {
				return filepath.IsAbs(result) && strings.HasPrefix(result, wd)
			},
		},
		{
			name:        "绝对路径保持不变",
			path: func() string {
				if filepath.Separator == '\\' {
					return "C:\\temp\\app.log" // Windows绝对路径
				}
				return "/tmp/app.log" // Unix绝对路径
			}(),
			expectError: false,
			checkFunc: func(result string) bool {
				if filepath.Separator == '\\' {
					return result == "C:\\temp\\app.log"
				}
				return result == "/tmp/app.log"
			},
		},

		// 错误情况测试
		{
			name:        "路径遍历攻击",
			path:        "../../../etc/passwd",
			expectError: true,
			errorMsg:    "路径遍历攻击",
		},
		{
			name:        "空路径",
			path:        "",
			expectError: true,
			errorMsg:    "路径不能为空",
		},
		{
			name:        "跳出工作目录",
			path:        strings.Repeat("../", 20) + "test.log",
			expectError: true,
			errorMsg:    "路径遍历攻击", // 实际返回的是路径遍历攻击错误
		},
		{
			name:        "系统敏感目录",
			path:        "/etc/test.log",
			expectError: filepath.Separator != '\\', // 仅在非Windows系统上期望错误
			errorMsg:    "不允许访问系统敏感目录",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sanitizePath(tt.path)

			if tt.expectError {
				if err == nil {
					t.Errorf("期望返回错误，但没有错误")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("期望错误消息包含 '%s', 实际错误 = '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("期望没有错误，但返回错误: %v", err)
					return
				}

				// 验证返回的路径
				if tt.checkFunc != nil && !tt.checkFunc(result) {
					t.Errorf("路径验证失败，输入: '%s', 输出: '%s'", tt.path, result)
				}

				// 验证返回的路径是清理过的
				if result != filepath.Clean(result) {
					t.Errorf("返回的路径应该是清理过的，期望: '%s', 实际: '%s'", filepath.Clean(result), result)
				}
			}
		})
	}
}

// TestSanitizePath_EdgeCases 测试边界情况
func TestSanitizePath_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "单个文件名",
			path:     "app.log",
			expected: "app.log",
		},
		{
			name:     "只有目录",
			path:     "logs/",
			expected: "logs",
		},
		{
			name:     "复杂嵌套路径",
			path:     "a/b/c/d/e/f.log",
			expected: "a/b/c/d/e/f.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sanitizePath(tt.path)
			if err != nil {
				t.Errorf("不期望错误，但返回: %v", err)
				return
			}

			// 验证路径后缀 (跨平台兼容)
			expected := filepath.FromSlash(tt.expected)
			if !strings.HasSuffix(result, expected) {
				t.Errorf("期望路径后缀 '%s', 实际路径 = '%s'", expected, result)
			}
		})
	}
}

// TestPathValidation_Performance 性能测试
func TestPathValidation_Performance(t *testing.T) {
	testPaths := []string{
		"logs/app.log",
		"deep/nested/directory/structure/file.log",
		"simple.log",
		"/tmp/absolute.log",
	}

	for _, path := range testPaths {
		t.Run("validatePath_"+path, func(t *testing.T) {
			for i := 0; i < 1000; i++ {
				err := validatePath(path)
				if err != nil {
					t.Errorf("不期望错误: %v", err)
					break
				}
			}
		})

		t.Run("sanitizePath_"+path, func(t *testing.T) {
			for i := 0; i < 1000; i++ {
				_, err := sanitizePath(path)
				if err != nil {
					t.Errorf("不期望错误: %v", err)
					break
				}
			}
		})
	}
}

// BenchmarkValidatePath 基准测试 validatePath 函数
func BenchmarkValidatePath(b *testing.B) {
	testPath := "logs/app.log"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = validatePath(testPath)
	}
}

// BenchmarkSanitizePath 基准测试 sanitizePath 函数
func BenchmarkSanitizePath(b *testing.B) {
	testPath := "logs//./app.log"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = sanitizePath(testPath)
	}
}

// TestPathValidation_Concurrent 并发测试
func TestPathValidation_Concurrent(t *testing.T) {
	testPaths := []string{
		"logs/app1.log",
		"logs/app2.log",
		"logs/app3.log",
		"logs/app4.log",
	}

	// 并发测试 validatePath
	t.Run("validatePath_concurrent", func(t *testing.T) {
		for _, path := range testPaths {
			go func(p string) {
				for i := 0; i < 100; i++ {
					if err := validatePath(p); err != nil {
						t.Errorf("并发测试失败: %v", err)
					}
				}
			}(path)
		}
	})

	// 并发测试 sanitizePath
	t.Run("sanitizePath_concurrent", func(t *testing.T) {
		for _, path := range testPaths {
			go func(p string) {
				for i := 0; i < 100; i++ {
					if _, err := sanitizePath(p); err != nil {
						t.Errorf("并发测试失败: %v", err)
					}
				}
			}(path)
		}
	})
}
