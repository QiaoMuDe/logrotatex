// internal.go 包含了logrotatex包的内部实现细节和辅助函数。
// 该文件提供了日志轮转过程中需要的内部工具函数、常量定义
// 和私有方法，支持核心功能的实现但不对外暴露接口。

package logrotatex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// backupTimeFormat 是备份文件的时间戳格式，使用纯数字格式提高性能和兼容性。
	// 格式: YYYYMMDDHHMMSS (年月日时分秒)
	backupTimeFormat = "20060102150405"

	// expectedTimestampLen 是时间戳的长度, 用于验证文件名中的时间戳是否有效。
	// 纯数字时间戳长度: "20060102150405" = 14字符
	expectedTimestampLen = 14

	// defaultMaxSize 是日志文件的最大默认大小(单位: MB), 在未明确设置时使用此值。
	defaultMaxSize = 10

	// defaultLogSuffix 是默认日志文件的后缀名
	defaultLogSuffix = "_logrotatex.log"

	// defaultFilePerm 是日志文件的默认权限模式
	defaultFilePerm = 0600

	// defaultDirPerm 是日志目录的默认权限模式
	defaultDirPerm = 0700
)

// logInfo 是一个便捷结构体，用于返回文件名及其嵌入的时间戳。
// 它包含了日志文件的时间戳信息和文件系统信息，用于日志轮转时的文件管理。
type logInfo struct {
	// timestamp 是从文件名中解析出的时间戳
	timestamp time.Time
	// FileInfo 包含文件的基本信息( 大小、修改时间等)
	os.FileInfo
}

// byFormatTime 是一个自定义的排序类型, 用于按文件名中格式化的时间对日志文件进行排序。
// 它实现了 sort.Interface 接口，可以被 sort.Sort 函数使用。
type byFormatTime []logInfo

// Less 比较两个日志文件的时间戳，按时间从新到旧排序。
//
// 参数:
//   - i, j: 要比较的元素索引
//
// 返回值:
//   - bool: 如果 i 的时间戳晚于 j 则返回 true
func (b byFormatTime) Less(i, j int) bool {
	return b[i].timestamp.After(b[j].timestamp)
}

// Swap 交换两个日志文件的位置。
func (b byFormatTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

// Len 返回日志文件列表的长度。
func (b byFormatTime) Len() int {
	return len(b)
}

// filename 获取当前日志文件的完整路径。
// 如果未指定 LogFilePath，则使用默认文件名：程序名_logrotatex.log
//
// 返回值:
//   - string: 日志文件的完整路径
func (l *LogRotateX) filename() string {
	// 如果已经指定了日志文件名, 则直接返回
	if l.LogFilePath != "" {
		return l.LogFilePath
	}
	// 生成默认的日志文件名, 格式为: 程序名_logrotatex.log
	name := filepath.Base(os.Args[0]) + defaultLogSuffix

	// 将日志文件存储在系统的临时目录中
	return filepath.Join(os.TempDir(), name)
}

// max 返回日志文件轮转的大小阈值。
// 如果未设置 MaxSize，则使用默认值。
//
// 返回值:
//   - int64: 日志文件的最大允许大小，单位为字节
func (l *LogRotateX) max() int64 {
	// 如果未设置最大大小, 则使用默认值
	if l.MaxSize == 0 {
		return int64(defaultMaxSize * megabyte)
	}
	// 将最大大小从 MB 转换为字节
	return int64(l.MaxSize) * int64(megabyte)
}

// dir 获取日志文件所在的目录路径。
//
// 返回值:
//   - string: 日志文件所在的目录路径
func (l *LogRotateX) dir() string {
	return filepath.Dir(l.filename())
}

// prefixAndExt 解析日志文件名，分离前缀和扩展名。
// 如果没有前缀，使用程序名作为默认前缀。
//
// 返回值:
//   - prefix: 文件名前缀
//   - ext: 文件扩展名( 包含点号)
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

// close 安全地关闭当前打开的日志文件，防止资源泄漏。
//
// 返回值:
//   - error: 关闭失败时返回错误，否则返回 nil
func (l *LogRotateX) close() error {
	// 检查 l.file 是否为 nil, 如果是则直接返回 nil, 表示没有文件需要关闭
	if l.file == nil {
		return nil
	}

	// 保存文件句柄的引用，以便在出错时也能将其置为nil
	file := l.file
	// 立即将 l.file 置为 nil, 防止在关闭过程中其他goroutine访问已关闭的文件
	l.file = nil

	// 调用文件的 Close 方法, 尝试关闭文件
	var closeErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				closeErr = fmt.Errorf("logrotatex: panic occurred while closing file: %v", r)
			}
		}()
		closeErr = file.Close()
	}()

	// 返回关闭文件时可能产生的错误
	if closeErr != nil {
		return fmt.Errorf("logrotatex: failed to close log file: %w", closeErr)
	}

	return nil
}

// rotate 执行日志文件轮转操作，关闭当前文件并创建新文件。
//
// 返回值:
//   - error: 轮转失败时返回错误，否则返回 nil
func (l *LogRotateX) rotate() error {
	// 调用 close 方法关闭当前的日志文件。
	if err := l.close(); err != nil {
		return err
	}

	// 调用 openNew 方法打开一个新的日志文件。
	if err := l.openNew(); err != nil {
		return err
	}

	// 清理操作：按开关选择同步或异步
	if l.Async {
		// 异步：不阻塞轮转/写入
		l.cleanupAsync()
	} else {
		// 同步：保持兼容
		if err := l.cleanupSync(); err != nil {
			fmt.Printf("logrotatex: cleanup failed during rotation: %v\n", err)
		}
	}

	return nil
}

// openNew 创建新的日志文件，将现有文件重命名为备份文件。
//
// 返回值:
//   - error: 创建失败时返回错误，否则返回 nil
func (l *LogRotateX) openNew() error {
	// 确保日志文件所在目录存在，使用更安全的目录权限
	// 如果目录不存在则创建，如果已存在则不执行任何操作
	if err := os.MkdirAll(l.dir(), defaultDirPerm); err != nil {
		return fmt.Errorf("logrotatex: unable to create required directory for log file: %w", err)
	}

	// 获取日志文件的完整路径
	name := l.filename()

	// 获取文件的权限模式
	mode := l.filePerm
	// 如果未设置filePerm, 则使用默认值0600
	if mode == 0 {
		mode = os.FileMode(defaultFilePerm)
	}

	// 获取文件信息
	info, err := os.Stat(name)
	if err == nil {
		// 如果旧日志文件存在, 复制其权限模式
		mode = info.Mode()

		// 将现有的日志文件重命名为备份文件
		newname := genTimeName(name, l.LocalTime, l.DateDirLayout)

		// 如果启用日期目录，确保目标日期目录存在
		if l.DateDirLayout {
			dateDir := filepath.Dir(newname)
			if err := os.MkdirAll(dateDir, defaultDirPerm); err != nil {
				return fmt.Errorf("logrotatex: unable to create date directory: %w", err)
			}
		}

		if renameErr := os.Rename(name, newname); renameErr != nil {
			return fmt.Errorf("logrotatex: unable to rename log file: %w", renameErr)
		}

		// 在非 Linux 系统上, 此操作无效
		if chownErr := chown(name, info); chownErr != nil {
			return fmt.Errorf("logrotatex: unable to set file owner: %w", chownErr)
		}
	}

	// 使用 truncate 打开文件, 确保文件存在且可写入。
	// 如果文件已存在( 可能是其他进程创建的), 则清空内容。
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("logrotatex: unable to open new log file: %w", err)
	}

	// 先保存旧文件引用
	oldFile := l.file

	// 立即设置新文件状态( 确保状态一致性)
	l.file = f
	l.size = 0

	// 然后尝试关闭旧文件( 失败也不影响新文件的使用)
	if oldFile != nil {
		if closeErr := oldFile.Close(); closeErr != nil {
			// 记录错误但不返回失败，因为新文件已经可用
			fmt.Printf("logrotatex: warning - failed to close old file: %v\n", closeErr)
		}
	}

	return nil
}

// genTimeName 根据原始文件名生成带时间戳的备份文件名
//
// 参数:
//   - name: 原始文件名
//   - local: 是否使用本地时间, false 使用 UTC 时间
//   - dateDirLayout: 是否启用日期目录布局
//
// 返回值:
//   - string: 带时间戳的备份文件名
func genTimeName(name string, local bool, dateDirLayout bool) string {
	// 获取文件所在的目录
	dir := filepath.Dir(name)

	// 获取文件的基本名称( 包含扩展名)
	filename := filepath.Base(name)

	// 更安全地处理文件名和扩展名
	ext := filepath.Ext(filename)
	prefix := strings.TrimSuffix(filename, ext)

	// 如果文件名以点号结尾但没有扩展名( 例如"logfile.")，确保正确处理
	if len(ext) > 0 && ext == filename {
		// 处理纯扩展名文件( 例如".gitignore")
		prefix = ""
	} else if len(prefix) == 0 && len(ext) > 0 {
		// 处理以点号开头的文件( 例如".logfile")
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

	// 如果启用日期目录，生成日期目录名
	if dateDirLayout {
		dateDir := t.Format("2006-01-02")
		return filepath.Join(dir, dateDir, fmt.Sprintf("%s_%s%s", prefix, timestamp, ext))
	}

	// 生成新的文件名
	return filepath.Join(dir, fmt.Sprintf("%s_%s%s", prefix, timestamp, ext))
}

// openExistingOrNew 确保日志文件已打开，根据文件大小决定是否需要轮转。
//
// 参数:
//   - writeLen: 预计写入的数据长度
//
// 返回值:
//   - error: 打开文件失败时返回错误，否则返回 nil
func (l *LogRotateX) openExistingOrNew(writeLen int) error {
	// 如果文件已经打开，直接返回
	if l.file != nil {
		return nil
	}

	// 获取日志文件的完整路径
	filename := l.filename()
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		// 如果文件不存在, 直接创建新文件
		return l.openNew()
	}
	if err != nil {
		// 如果获取文件信息失败, 返回错误
		return fmt.Errorf("logrotatex: error getting log file info: %w", err)
	}

	// 检查写入操作是否会达到或超出最大文件大小限制
	if info.Size()+int64(writeLen) >= l.max() {
		// 如果会达到或超出限制, 则执行日志文件的轮转操作
		return l.rotate()
	}

	// 以追加模式打开现有日志文件
	filePerm := l.filePerm
	if filePerm == 0 {
		filePerm = os.FileMode(defaultFilePerm)
	}
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, filePerm)
	if err != nil {
		return l.openNew() // 如果打开文件失败, 则创建新文件
	}

	// 先保存旧文件引用
	oldFile := l.file

	// 立即更新日志对象的文件句柄和当前文件大小
	l.file = file
	l.size = info.Size()

	// 然后尝试关闭旧文件( 失败也不影响新文件的使用)
	if oldFile != nil {
		if closeErr := oldFile.Close(); closeErr != nil {
			// 记录错误但不返回失败，因为新文件已经可用
			fmt.Printf("logrotatex: warning - failed to close old file: %v\n", closeErr)
		}
	}

	return nil
}

// shouldRotateByDay 检查是否需要按天轮转
//
// 返回值:
//   - bool: true 表示需要轮转, false 表示不需要轮转
func (l *LogRotateX) shouldRotateByDay() bool {
	// 获取当前时间（考虑 LocalTime 配置）
	now := currentTime()
	if !l.LocalTime {
		now = now.UTC()
	}

	// 如果是首次运行，记录当前日期但不轮转
	if l.lastRotationDate.IsZero() {
		l.lastRotationDate = now
		return false
	}

	// 检查是否跨天（只比较日期部分）
	currentDate := now.Format("2006-01-02")
	lastDate := l.lastRotationDate.Format("2006-01-02")

	if currentDate != lastDate {
		// 跨天了，更新上次轮转日期
		l.lastRotationDate = now
		return true
	}

	return false
}
