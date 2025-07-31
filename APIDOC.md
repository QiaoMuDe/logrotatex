package logrotatex // import "gitee.com/MM-Q/logrotatex"

logrotatex 旨在成为日志记录基础设施的一部分。 它并非一个一体化的解决方案, 而是日志记录栈底层的一个可插拔组件, 仅用于控制日志写入的文件。

logrotatex 可以与任何能够写入 io.Writer 的日志记录包配合使用, 包括标准库的 log 包。

logrotatex 假定只有一个进程在向输出文件写入日志。 在同一台机器上的多个进程使用相同的 logrotatex 配置会导致异常行为。

TYPES

type LogRotateX struct {
	// Filename 是写入日志的文件。备份日志文件将保留在同一目录中。如果该值为空, 则使用 os.TempDir() 下的 <进程名>-logrotatex.log。
	Filename string `json:"filename" yaml:"filename"`

	// MaxSize 最大单个日志文件的大小（以 MB 为单位）。默认值为 10 MB。
	MaxSize int `json:"maxsize" yaml:"maxsize"`

	// MaxAge 最大保留日志文件的天数。默认情况下, 不会删除旧日志文件。
	MaxAge int `json:"maxage" yaml:"maxage"`

	// MaxBackups 最大保留日志文件的数量。默认情况下, 不会删除旧日志文件。
	MaxBackups int `json:"maxbackups" yaml:"maxbackups"`

	// LocalTime 决定是否使用本地时间记录日志文件的轮转时间。默认使用 UTC 时间。
	LocalTime bool `json:"localtime" yaml:"localtime"`

	// Compress 决定轮转后的日志文件是否应使用 gzip 进行压缩。默认不进行压缩。
	Compress bool `json:"compress" yaml:"compress"`

	// Has unexported fields.
}
    LogRotateX 是一个 io.WriteCloser, 它会将日志写入指定的文件名。

    首次调用 Write 方法时, LogRotateX 会打开或创建日志文件。如果文件已存在且大小小于 MaxSize 兆字节, logrotatex
    会打开该文件并追加写入。 如果文件已存在且大小大于或等于 MaxSize 兆字节, 该文件会被重命名, 重命名时会在文件名的扩展名之前（如果没有扩展名,
    则在文件名末尾）插入当前时间戳。然后会使用原始文件名创建一个新的日志文件。

    每当写入操作会导致当前日志文件大小超过 MaxSize 兆字节时, 当前文件会被关闭、重命名, 并使用原始文件名创建一个新的日志文件。因此,
    你提供给 LogRotateX 的文件名始终是“当前”的日志文件。

    备份文件使用提供给 LogRotateX 的日志文件名, 格式为 `name-timestamp.ext`, 其中 name 是不带扩展名的文件名,
    timestamp 是日志轮转时的时间, 格式为 `2006-01-02T15-04-05.000`, ext 是原始扩展名。 例如, 如果你的
    LogRotateX.Filename 是 `/var/log/foo/server.log`, 在 2016 年 11 月 11 日下午 6:30
    创建的备份文件名将是 `/var/log/foo/server-2016-11-04T18-30-00.000.log`。

    # 清理旧日志文件

    每当创建新的日志文件时, 可能会删除旧的日志文件。根据编码的时间戳, 最近的文件会被保留, 最多保留数量等于 MaxBackups（如果
    MaxBackups 为 0, 则保留所有文件）。 任何编码时间戳早于 MaxAge 天的文件都会被删除, 无论 MaxBackups
    的设置如何。请注意, 时间戳中编码的时间是轮转时间, 可能与该文件最后一次写入的时间不同。

    如果 MaxBackups 和 MaxAge 都为 0, 则不会删除任何旧日志文件。

func (l *LogRotateX) Close() error
    Close 是 LogRotateX 类型的 Close 方法, 用于关闭日志记录器

func (l *LogRotateX) Rotate() error
    Rotate 是 LogRotateX 类型的一个方法, 用于执行日志文件的轮转操作。

func (l *LogRotateX) Write(p []byte) (n int, err error)
    Write 方法实现了 io.Writer 接口。如果一次写入操作会使日志文件大小超过 MaxSize, 则关闭当前文件, 将其重命名并包含当前时间戳,
    然后使用原始文件名创建一个新的日志文件。 如果写入内容的长度大于 MaxSize, 则返回一个错误。

