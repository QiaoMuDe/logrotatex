package logrotatex // import "gitee.com/MM-Q/logrotatex"

Package logrotatex 提供了一个日志轮转功能的实现，用于管理日志文件的大小和数量。 它是一个轻量级的组件，可以与任何支持 io.Writer
接口的日志库配合使用。

主要功能： - 自动轮转日志文件，防止单个文件过大 - 支持设置最大文件大小、保留文件数量和保留天数 - 支持日志文件压缩 - 线程安全的设计，适用于并发环境

注意事项： - 假设只有一个进程在向输出文件写入日志 - 多个进程使用相同的配置可能导致异常行为

TYPES

type LogRotateX struct {
	//  ========== 配置字段 ==========
	// Filename 是写入日志的文件。备份日志文件将保留在同一目录中。如果该值为空, 则使用 os.TempDir() 下的 <程序名>_logrotatex.log。
	Filename string `json:"filename" yaml:"filename"`

	// MaxSize 最大单个日志文件的大小（以 MB 为单位）。默认值为 10 MB。
	MaxSize int `json:"maxsize" yaml:"maxsize"`

	// MaxAge 最大保留日志文件的天数。默认情况下, 不会删除旧日志文件。
	MaxAge int `json:"maxage" yaml:"maxage"`

	// MaxBackups 最大保留日志文件的数量。默认情况下, 不会删除旧日志文件。
	MaxBackups int `json:"maxbackups" yaml:"maxbackups"`

	// ========== 行为选项 ==========
	// LocalTime 决定是否使用本地时间记录日志文件的轮转时间。默认使用 UTC 时间。
	LocalTime bool `json:"localtime" yaml:"localtime"`

	// Compress 决定轮转后的日志文件是否应使用 zip 进行压缩。默认不进行压缩。
	Compress bool `json:"compress" yaml:"compress"`

	// FilePerm 是日志文件的权限模式。默认值为 0600。
	FilePerm os.FileMode `json:"fileperm" yaml:"fileperm"`

	// Has unexported fields.
}
    LogRotateX 是一个 io.WriteCloser，它会将日志写入指定的文件名。

    首次调用 Write 方法时，LogRotateX 会打开或创建日志文件。如果文件已存在且大小小于 MaxSize
    兆字节， logrotatex 会打开该文件并追加写入。如果文件已存在且大小大于或等于 MaxSize 兆字节，
    该文件会被重命名，重命名时会在文件名的扩展名之前（如果没有扩展名，则在文件名末尾）插入当前时间戳。 然后会使用原始文件名创建一个新的日志文件。

    每当写入操作会导致当前日志文件大小超过 MaxSize 兆字节时，当前文件会被关闭、重命名， 并使用原始文件名创建一个新的日志文件。因此，你提供给
    LogRotateX 的文件名始终是"当前"的日志文件。

    备份文件使用提供给 LogRotateX 的日志文件名，格式为 `name-timestamp.ext`， 其中 name
    是不带扩展名的文件名，timestamp 是日志轮转时的时间，格式为 `2006-01-02T15-04-05.000`， ext
    是原始扩展名。例如，如果你的 LogRotateX.Filename 是 `/var/log/foo/server.log`， 在 2016 年 11
    月 11 日下午 6:30 创建的备份文件名将是 `/var/log/foo/server-2016-11-04T18-30-00.000.log`。

    # 清理旧日志文件

    每当创建新的日志文件时，可能会删除旧的日志文件。根据编码的时间戳，最近的文件会被保留， 最多保留数量等于 MaxBackups（如果
    MaxBackups 为 0，则保留所有文件）。 任何编码时间戳早于 MaxAge 天的文件都会被删除，无论 MaxBackups 的设置如何。
    请注意，时间戳中编码的时间是轮转时间，可能与该文件最后一次写入的时间不同。

    如果 MaxBackups 和 MaxAge 都为 0，则不会删除任何旧日志文件。

func NewLogRotateX(filename string) *LogRotateX
    NewLogRotateX 创建一个新的 LogRotateX 实例，使用指定的文件路径和合理的默认配置。
    该构造函数会验证和清理文件路径，确保路径安全性，并设置推荐的默认值。 如果路径不安全或创建失败，此函数会立即 panic，确保问题能够快速被发现。

    参数:
      - filename string: 日志文件的路径，会进行安全验证和清理

    返回值:
      - *LogRotateX: 配置好的 LogRotateX 实例

    默认配置:
      - MaxSize: 10MB (单个日志文件最大大小)
      - MaxAge: 0天 (日志文件最大保留时间, 0表示不清理历史文件)
      - MaxBackups: 0个 (最大备份文件数量, 0表示不清理备份文件)
      - LocalTime: true (使用本地时间)
      - Compress: false (禁用压缩)
      - FilePerm: 0600 (文件权限，所有者读写，组和其他用户只读)

    注意: 如果文件路径不安全或创建失败，此函数会 panic

func (l *LogRotateX) Close() error
    Close 是 LogRotateX 类型的 Close 方法, 用于关闭日志记录器。
    该方法会关闭当前打开的日志文件，释放相关资源，并停止后台goroutine。 此操作是线程安全的，使用 sync.Once
    防止重复调用，并通过上下文控制超时。 在异常情况下确保文件句柄正确关闭，防止资源泄漏。

    返回值:
      - error: 如果在关闭文件时发生错误，则返回该错误；否则返回 nil。

func (l *LogRotateX) CurrentFile() string
    CurrentFile 返回当前正在写入的日志文件路径。 该方法返回的是主日志文件的路径，即用户在创建 LogRotateX 时指定的文件名。
    这个路径始终指向"当前"的日志文件，轮转后的备份文件会使用不同的文件名。

    注意事项：
      - 此方法不需要加锁，因为 Filename 字段在创建后不会改变
      - 返回的路径是绝对路径（经过 sanitizePath 处理）
      - 即使文件尚未创建，也会返回预期的文件路径

    返回值:
      - string: 当前日志文件的完整路径

func (l *LogRotateX) GetCurrentSize() int64
    GetCurrentSize 返回当前日志文件的大小（以字节为单位）。 该方法返回当前正在写入的日志文件已经写入的字节数。
    这个值会在每次写入操作后更新，可以用来监控文件大小或判断是否接近轮转阈值。

    注意事项：
      - 此操作是线程安全的，使用互斥锁保护
      - 返回的是内存中维护的大小计数，而不是实际查询文件系统
      - 如果文件尚未创建或打开，返回值为 0

    返回值:
      - int64: 当前文件大小（字节）

func (l *LogRotateX) GetMaxSize() int64
    GetMaxSize 返回配置的最大文件大小（以字节为单位）。 该方法将用户配置的 MaxSize（以 MB 为单位）转换为字节数返回。
    当日志文件大小达到或超过此值时，会触发日志轮转操作。

    注意事项：
      - 此方法不需要加锁，因为 MaxSize 是配置值，通常不会在运行时改变
      - 返回值是以字节为单位的 int64 类型
      - 转换公式：MaxSize(MB) * 1024 * 1024 = 字节数

    返回值:
      - int64: 最大文件大小（字节）

func (l *LogRotateX) Rotate() error
    Rotate 是 LogRotateX 类型的一个方法, 用于执行日志文件的轮转操作。 该方法会关闭当前日志文件，将其重命名为带有时间戳的备份文件，
    然后创建一个新的日志文件用于后续写入。 此操作是线程安全的，使用互斥锁保护。

    返回值:
      - error: 如果在执行轮转操作时发生错误，则返回该错误；否则返回 nil。

func (l *LogRotateX) Sync() error
    Sync 强制将缓冲区数据同步到磁盘。 该方法会调用底层文件的 Sync() 方法，确保所有写入的数据都被刷新到磁盘上，
    而不是仅仅保存在操作系统的缓冲区中。这对于确保数据持久性很重要， 特别是在系统可能意外关闭的情况下。

    注意事项：
      - 此操作是线程安全的，使用互斥锁保护
      - 如果当前没有打开的文件，则直接返回 nil
      - 频繁调用此方法可能会影响写入性能

    返回值:
      - error: 如果同步操作失败，则返回相应的错误；否则返回 nil

func (l *LogRotateX) Write(p []byte) (n int, err error)
    Write 实现了 io.Writer 接口，用于向日志文件写入数据。 该方法会处理日志轮转逻辑，确保单个日志文件不会超过设定的最大大小。

    参数:
      - p []byte: 要写入的日志数据

    返回值:
      - n int: 实际写入的字节数
      - err error: 如果写入过程中发生错误，则返回该错误；否则返回 nil

