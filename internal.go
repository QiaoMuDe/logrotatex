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

	"gitee.com/MM-Q/comprx"
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

// getDefaultLogFilePath 生成默认的日志文件路径
//
// 返回值:
//   - string: 默认的日志文件完整路径
func getDefaultLogFilePath() string {
	// 获取程序名，如果为空则使用默认值
	progName := filepath.Base(os.Args[0])
	if progName == "" || progName == "." || progName == "/" {
		progName = "logrotatex"
	}
	return filepath.Join(os.TempDir(), progName+defaultLogSuffix)
}

// initDefaults 初始化 LogRotateX 实例的默认值。
// 该方法确保无论是通过构造函数创建还是直接通过结构体字面量创建，
// 都能获得一致的初始化行为。
// 注意：该方法只会执行一次，避免重复初始化。
//
// 返回值:
//   - error: 初始化失败时返回错误，否则返回 nil
func (l *LogRotateX) initDefaults() error {
	var initErr error

	// 使用 sync.Once 确保初始化只执行一次
	l.once.Do(func() {
		// 如果 LogFilePath 为空，设置默认值
		if l.LogFilePath == "" {
			l.LogFilePath = getDefaultLogFilePath()
		}

		// 确保 LogFilePath 是干净的路径
		l.LogFilePath = filepath.Clean(l.LogFilePath)
		l.LogFilePath = strings.TrimSpace(l.LogFilePath)

		// 再次验证文件路径（防御性编程）
		if l.LogFilePath == "" || l.LogFilePath == "." {
			initErr = fmt.Errorf("log file path cannot be empty")
			return
		}

		// 确保目录存在
		dir := filepath.Dir(l.LogFilePath)
		if err := os.MkdirAll(dir, defaultDirPerm); err != nil {
			initErr = fmt.Errorf("failed to create log directory: %w", err)
			return
		}

		// 初始化最大文件大小
		if l.MaxSize <= 0 {
			l.MaxSize = defaultMaxSize
		}

		// 初始化最大保留时间
		if l.MaxAge < 0 {
			l.MaxAge = 0
		}

		// 初始化最大备份文件数
		if l.MaxFiles < 0 {
			l.MaxFiles = 0
		}

		// 初始化内部文件权限
		if l.filePerm == 0 {
			l.filePerm = defaultFilePerm
		}

		// 初始化内部文件大小
		if l.size == 0 {
			l.size = 0
		}

		// 显式设置原子布尔的初始值为 false
		l.closed.Store(false)
		l.cleanupRunning.Store(false)
		l.rerunNeeded.Store(false)

		// 初始化压缩类型, 如果为空, 则设置为默认值 zip
		if l.CompressType.String() == "" {
			l.CompressType = comprx.CompressTypeZip
		}
	})

	return initErr
}

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

	// 返回默认路径
	return getDefaultLogFilePath()
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
// 使用与 genTimeName 一致的解析方式，避免多次字符串操作。
//
// 返回值:
//   - prefix: 文件名前缀
//   - ext: 文件扩展名( 包含点号)
func (l *LogRotateX) prefixAndExt() (prefix, ext string) {
	filename := filepath.Base(l.filename()) // 获取日志文件的基本名称

	// 使用与 genTimeName 一致的解析方式
	lastDot := strings.LastIndex(filename, ".")

	if lastDot == -1 {
		// 没有扩展名
		return filename, ""
	}

	// 返回前缀和扩展名
	return filename[:lastDot], filename[lastDot:]
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

	// 直接关闭文件，文件关闭操作通常不会 panic
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close log file: %w", err)
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
		return fmt.Errorf("failed to open new file during rotation: %w", err)
	}

	// 清理操作：按开关选择同步或异步
	if l.Async {
		// 异步：不阻塞轮转/写入
		l.cleanupAsync()
	} else {
		// 同步：保持兼容
		if err := l.cleanupSync(); err != nil {
			fmt.Printf("cleanup failed during rotation: %v\n", err)
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
		return fmt.Errorf("unable to create required directory for log file: %w", err)
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
				return fmt.Errorf("unable to create date directory: %w", err)
			}
		}

		// 重命名文件到新路径
		if renameErr := os.Rename(name, newname); renameErr != nil {
			return fmt.Errorf("unable to rename log file: %w", renameErr)
		}

		// // 在非 Linux 系统上, 此操作无效
		// if chownErr := chown(name, info); chownErr != nil {
		// 	return fmt.Errorf("unable to set file owner: %w", chownErr)
		// }
	}

	// 使用 truncate 打开文件, 确保文件存在且可写入。
	// 如果文件已存在( 可能是其他进程创建的), 则清空内容。
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("unable to open new log file: %w", err)
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
			fmt.Printf("warning - failed to close old file: %v\n", closeErr)
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

	// 一次性解析文件名各部分，避免多次字符串操作
	lastDot := strings.LastIndex(filename, ".")
	var prefix, ext string

	switch lastDot {
	case -1:
		// 没有扩展名
		prefix = filename
		ext = ""
	case 0:
		// 以点号开头的文件（如.gitignore）
		prefix = filename
		ext = ""
	default:
		// 正常情况：有扩展名
		prefix = filename[:lastDot]
		ext = filename[lastDot:]
	}

	// 获取当前时间
	t := currentTime()
	// 如果未指定使用本地时间, 则将时间转换为 UTC
	if !local {
		t = t.UTC()
	}

	// 格式化时间戳
	timestamp := t.Format(backupTimeFormat)

	// 生成带时间戳的文件名部分
	timedName := fmt.Sprintf("%s_%s%s", prefix, timestamp, ext)

	// 如果启用日期目录，生成日期目录名
	if dateDirLayout {
		dateDir := t.Format("2006-01-02")
		return filepath.Join(dir, dateDir, timedName)
	}

	// 生成新的文件名
	return filepath.Join(dir, timedName)
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
		return fmt.Errorf("error getting log file info: %w", err)
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
			fmt.Printf("warning - failed to close old file: %v\n", closeErr)
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

	// 检查是否跨天（直接比较时间分量）
	nowYear, nowMonth, nowDay := now.Date()
	lastYear, lastMonth, lastDay := l.lastRotationDate.Date()

	// 如果年、月、日有任何不匹配，说明跨天了
	if nowYear != lastYear || nowMonth != lastMonth || nowDay != lastDay {
		// 跨天了，更新上次轮转日期
		l.lastRotationDate = now
		return true
	}

	return false
}
