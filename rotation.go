// rotation.go 实现了logrotatex包的日志轮转核心逻辑。
// 该文件包含了日志文件轮转的具体实现，包括轮转条件判断、文件重命名、
// 备份文件管理、清理策略等关键功能，是日志轮转系统的核心模块。

package logrotatex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// close 是 LogRotateX 类型的实例方法, 用于安全地关闭当前打开的日志文件。
// 该方法增强了错误处理和资源管理，确保在异常情况下文件句柄能正确关闭，防止资源泄漏。
//
// 该方法执行以下操作：
// 1. 检查是否有文件需要关闭(l.file != nil)
// 2. 调用文件的 Close 方法执行实际关闭操作
// 3. 将文件句柄置为 nil, 防止重复关闭
// 4. 返回关闭过程中可能产生的错误
//
// 返回值：
//   - 如果成功关闭文件或没有文件需要关闭, 返回 nil
//   - 如果关闭文件时发生错误, 返回相应的 error
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
	// 使用defer确保即使在panic情况下也能正确处理
	var closeErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				closeErr = fmt.Errorf("关闭文件时发生panic: %v", r)
			}
		}()
		closeErr = file.Close()
	}()

	// 返回关闭文件时可能产生的错误
	if closeErr != nil {
		return fmt.Errorf("logrotatex: 关闭日志文件失败: %w", closeErr)
	}

	return nil
}

// rotate 是 LogRotateX 结构体的一个方法，用于执行日志文件的轮转操作。
//
// 日志轮转是日志管理的核心功能，当当前日志文件达到一定条件时（如大小限制），
// 需要将其重命名为备份文件并创建一个新的日志文件继续写入。
//
// 该方法执行以下操作：
// 1. 调用 close 方法关闭当前的日志文件
// 2. 调用 openNew 方法打开一个新的日志文件
// 3. 调用 mill 方法处理日志文件的后续逻辑（如压缩、清理等）
//
// 返回值：
//   - 如果所有操作都成功完成，返回 nil
//   - 如果在关闭当前文件或打开新文件时发生错误，返回相应的 error
func (l *LogRotateX) rotate() error {
	// 调用 close 方法关闭当前的日志文件。
	// 如果关闭过程中发生错误, 返回该错误。
	if err := l.close(); err != nil {
		return err
	}
	// 调用 openNew 方法打开一个新的日志文件。
	// 如果打开过程中发生错误, 返回该错误。
	if err := l.openNew(); err != nil {
		return err
	}
	// 调用 mill 方法处理日志文件的轮转逻辑。
	// 该方法可能包括压缩旧日志文件、删除过期日志文件等操作。
	l.mill()
	// 如果上述操作都成功, 返回 nil 表示没有错误。
	return nil
}

// openNew 用于打开一个新的日志文件用于写入, 并将旧的日志文件移出当前路径。
// 该方法确保日志目录存在，处理现有文件的重命名，并创建新的日志文件。
// 在异常情况下确保文件句柄正确关闭，防止资源泄漏。
//
// 返回值：
//   - 如果成功打开新的日志文件，返回 nil
func (l *LogRotateX) openNew() error {
	// 确保日志文件所在目录存在，使用更安全的目录权限
	// 如果目录不存在则创建，如果已存在则不执行任何操作
	if err := os.MkdirAll(l.dir(), defaultDirPerm); err != nil {
		return fmt.Errorf("logrotatex: 无法创建日志文件所需目录: %w", err)
	}

	// 获取日志文件的完整路径
	name := l.filename()

	// 获取文件的权限模式
	mode := l.FilePerm
	// 如果未设置FilePerm, 则使用默认值0600
	if mode == 0 {
		mode = os.FileMode(defaultFilePerm)
	}

	// 获取文件信息
	info, err := os.Stat(name)
	if err == nil {
		// 如果旧日志文件存在, 复制其权限模式
		mode = info.Mode()

		// 将现有的日志文件重命名为备份文件
		newname := backupName(name, l.LocalTime)
		if renameErr := os.Rename(name, newname); renameErr != nil {
			return fmt.Errorf("logrotatex: 无法重命名日志文件: %w", renameErr)
		}

		// 在非 Linux 系统上, 此操作无效
		if chownErr := chown(name, info); chownErr != nil {
			return fmt.Errorf("logrotatex: 无法设置文件所有者: %w", chownErr)
		}
	}

	// 使用 truncate 打开文件, 确保文件存在且可写入。
	// 如果文件已存在（可能是其他进程创建的）, 则清空内容。
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("logrotatex: 无法打开新的日志文件: %w", err)
	}

	// 如果之前有打开的文件，先关闭它以防止文件句柄泄漏
	if l.file != nil {
		if closeErr := l.file.Close(); closeErr != nil {
			// 即使关闭旧文件失败，也要继续使用新文件，但记录错误
			_ = f.Close() // 关闭新打开的文件
			return fmt.Errorf("logrotatex: 关闭旧日志文件失败: %w", closeErr)
		}
	}

	l.file = f // 将打开的文件赋值给 LogRotateX 的 file 字段
	l.size = 0 // 重置文件大小为 0
	return nil
}

// backupName 根据给定的文件名创建一个新的备份文件名。
// 如果指定使用本地时间, 则在文件名和扩展名之间插入本地时间的时间戳；
// 否则插入 UTC 时间的时间戳。
//
// 参数:
//   - name: 原始文件名
//   - local: 是否使用本地时间, true 表示使用本地时间, false 表示使用 UTC 时间
//
// 返回值:
//   - 新的备份文件名
func backupName(name string, local bool) string {
	// 获取文件所在的目录
	dir := filepath.Dir(name)
	// 获取文件的基本名称（包含扩展名）
	filename := filepath.Base(name)

	// 更安全地处理文件名和扩展名
	ext := filepath.Ext(filename)
	prefix := strings.TrimSuffix(filename, ext)

	// 如果文件名以点号结尾但没有扩展名（例如"logfile."），确保正确处理
	if len(ext) > 0 && ext == filename {
		// 处理纯扩展名文件（例如".gitignore"）
		prefix = ""
	} else if len(prefix) == 0 && len(ext) > 0 {
		// 处理以点号开头的文件（例如".logfile"）
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

	// 生成新的备份文件名
	return filepath.Join(dir, fmt.Sprintf("%s_%s%s", prefix, timestamp, ext))
}

// openExistingOrNew 确保日志文件已正确打开。
// 如果文件已打开则直接返回；如果未打开则尝试打开现有文件或创建新文件。
// 如果文件存在且当前写入操作不会使文件大小超过 MaxSize, 则直接打开该文件。
// 如果文件不存在, 或者写入操作会使文件大小超过 MaxSize, 则创建一个新的日志文件。
// 在异常情况下确保文件句柄正确关闭，防止资源泄漏。
//
// 参数:
//   - writeLen: 预计写入的数据长度
//
// 返回值:
//   - 如果文件已打开或成功打开现有文件或创建新文件, 返回 nil
func (l *LogRotateX) openExistingOrNew(writeLen int) error {
	// 如果文件已经打开，直接返回
	if l.file != nil {
		return nil
	}

	// 确保日志文件的大小信息是最新的
	l.mill()

	// 获取日志文件的完整路径
	filename := l.filename()
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		// 如果文件不存在, 直接创建新文件
		return l.openNew()
	}
	if err != nil {
		// 如果获取文件信息失败, 返回错误
		return fmt.Errorf("logrotatex: 获取日志文件信息时出错: %w", err)
	}

	// 检查写入操作是否会超出最大文件大小限制
	if info.Size()+int64(writeLen) > l.max() {
		// 如果会超出限制, 则执行日志文件的轮转操作
		return l.rotate()
	}

	// 以追加模式打开现有日志文件
	// 使用更安全的默认权限0600
	filePerm := l.FilePerm
	if filePerm == 0 {
		filePerm = os.FileMode(defaultFilePerm) // 修复：使用更安全的默认权限
	}
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, filePerm)
	if err != nil {
		// 如果打开现有日志文件失败, 则通过openNew创建新文件
		return l.openNew()
	}

	// 如果之前有打开的文件，先关闭它以防止文件句柄泄漏
	if l.file != nil {
		if closeErr := l.file.Close(); closeErr != nil {
			// 即使关闭旧文件失败，也要继续使用新文件，但记录错误
			_ = file.Close() // 关闭新打开的文件
			return fmt.Errorf("logrotatex: 关闭旧日志文件失败: %w", closeErr)
		}
	}

	// 更新日志对象的文件句柄和当前文件大小
	l.file = file
	l.size = info.Size()
	return nil
}
