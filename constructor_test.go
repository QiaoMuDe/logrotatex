// constructor_test.go 包含了logrotatex包构造函数的测试用例。
// 该文件测试了Logger实例的创建过程，包括各种配置参数的验证、
// 默认值设置、错误处理等，确保构造函数能够正确初始化Logger对象。

package logrotatex

import (
	"os"
	"path/filepath"
	"testing"
)

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
