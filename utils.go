package logrotatex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
	name := filepath.Base(os.Args[0]) + defaultLogSuffix

	// 将日志文件存储在系统的临时目录中
	return filepath.Join(os.TempDir(), name)
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
		for dangerousPath, description := range dangerousPathsMap {
			if strings.HasPrefix(absPath, dangerousPath) {
				return fmt.Errorf("不允许访问系统敏感目录 %s (%s): %s",
					dangerousPath, description, absPath)
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
