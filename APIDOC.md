package logrotatex // import "gitee.com/MM-Q/logrotatex"

buffered_writer.go - 带缓冲批量写入器 实现简洁高效的批量写入优化，通过三重条件触发减少系统调用开销。

Package logrotatex 提供了一个日志轮转功能的实现，用于管理日志文件的大小和数量。 它是一个轻量级的组件，可以与任何支持 io.Writer
接口的日志库配合使用。

主要功能： - 自动轮转日志文件，防止单个文件过大 - 支持设置最大文件大小、保留文件数量和保留天数 - 支持日志文件压缩 - 线程安全的设计，适用于并发环境

注意事项： - 假设只有一个进程在向输出文件写入日志 - 多个进程使用相同的配置可能导致异常行为

VARIABLES

var NewBW = NewBufferedWriter
    NewBW 是 NewBufferedWriter 的简写形式，用于创建新的 BufferedWriter 实例。

var NewLRX = NewLogRotateX
    NewLRX 是 NewLogRotateX 的简写形式，用于创建新的 LogRotateX 实例。


FUNCTIONS

func WrapWriter(w io.Writer) io.WriteCloser
    WrapWriter 将 io.Writer 包装为不可关闭的 io.WriteCloser (具体类型为 noCloseWC)

    参数:
      - w: 要包装的 io.Writer

    返回值:
      - io.WriteCloser: 不可关闭的 WriteCloser 包装器


TYPES

type BufCfg struct {
	MaxBufferSize int           // 最大缓冲区大小，默认64KB (0 表示禁用缓冲区大小触发条件)
	MaxWriteCount int           // 最大写入次数，默认500次 (0 表示禁用写入次数触发条件)
	FlushInterval time.Duration // 刷新间隔，默认1秒 (0 表示禁用刷新间隔触发条件)
}
    BufCfg 缓冲写入器配置

func DefBufCfg() *BufCfg
    DefBufCfg 默认缓冲写入器配置

    注意:
      - 默认缓冲区大小为64KB，最大写入次数为500次，刷新间隔为1秒

type BufferedWriter struct {
	// Has unexported fields.
}
    BufferedWriter 带缓冲批量写入器 可以包装任何写入器和关闭器，提供批量写入功能

func NewBufferedWriter(wc io.WriteCloser, config *BufCfg) *BufferedWriter
    NewBufferedWriter 创建新的带缓冲批量写入器

    参数:
      - wc: 底层写入+关闭器（必需）
      - config: 配置（可选，如果为空，使用默认值）

    返回值:
      - *BufferedWriter: 新的带缓冲批量写入器实例

func NewStdoutBW(config *BufCfg) *BufferedWriter
    NewStdoutBW 创建面向标准输出的带缓冲写入器（不会关闭 stdout） 仅接收配置结构体，使用 os.Stdout 作为底层输出。

    注意：
      - 调用 Close() 不会关闭标准输出，适合长期运行的场景。

func (bw *BufferedWriter) BufferSize() int
    BufferSize 返回当前缓冲区中的字节数

func (bw *BufferedWriter) Close() error
    Close 关闭缓冲写入器

func (bw *BufferedWriter) Flush() error
    Flush 手动刷新缓冲区

func (bw *BufferedWriter) IsClosed() bool
    IsClosed 返回缓冲写入器是否已关闭

func (bw *BufferedWriter) TimeSinceLastFlush() time.Duration
    TimeSinceLastFlush 返回距离上次刷新的时间

func (bw *BufferedWriter) Write(p []byte) (n int, err error)
    Write 实现 io.Writer 接口 将数据写入缓冲区，达到刷新条件时自动批量写入

    参数:
      - p: 要写入的数据

    返回值:
      - n: 实际写入的字节数
      - err: 写入错误（如果有）

func (bw *BufferedWriter) WriteCount() int
    WriteCount 返回当前缓冲区中的写入次数

type LogRotateX struct {
	// LogFilePath 是写入日志的文件路径。备份日志文件将保留在同一目录中。
	// 如果该值为空，则使用 os.TempDir() 下的 <程序名>_logrotatex.log。
	LogFilePath string `json:"logfilepath" yaml:"logfilepath"`

	// MaxSize 是单个日志文件的最大大小（以 MB 为单位）。默认值为 10 MB。
	// 超过此大小的日志文件将被轮转。
	MaxSize int `json:"maxsize" yaml:"maxsize"`

	// MaxAge 是保留日志文件的天数，超过此天数的文件将被删除。
	// 默认值为 0，表示不按时间删除旧日志文件。
	MaxAge int `json:"maxage" yaml:"maxage"`

	// MaxFiles 是最大保留的历史日志文件数量，超过此数量的旧文件将被删除。
	// 默认值为 0，表示不限制文件数量。
	MaxFiles int `json:"maxfiles" yaml:"maxfiles"`

	// LocalTime 决定是否使用本地时间记录日志文件的轮转时间。
	// 默认使用 UTC 时间。
	LocalTime bool `json:"localtime" yaml:"localtime"`

	// Compress 决定轮转后的日志文件是否应使用 zip 进行压缩。
	// 默认不进行压缩。
	Compress bool `json:"compress" yaml:"compress"`

	// Has unexported fields.
}
    LogRotateX 是一个 io.WriteCloser，它会将日志写入指定的文件名。

    首次调用 Write 方法时，LogRotateX 会打开或创建日志文件。如果文件已存在且大小小于 MaxSize
    兆字节， logrotatex 会打开该文件并追加写入。如果文件已存在且大小大于或等于 MaxSize 兆字节，
    该文件会被重命名，重命名时会在文件名的扩展名之前（如果没有扩展名，则在文件名末尾）插入当前时间戳。 然后会使用原始文件名创建一个新的日志文件。

    每当写入操作会导致当前日志文件大小超过 MaxSize 兆字节时，当前文件会被关闭、重命名， 并使用原始文件名创建一个新的日志文件。因此，你提供给
    LogRotateX 的文件名始终是"当前"的日志文件。

    日志文件使用提供给 LogRotateX 的日志文件名，格式为 `name_timestamp.ext`， 其中 name
    是不带扩展名的文件名，timestamp 是日志轮转时的时间，格式为 `20060102150405`， ext 是原始扩展名。例如，如果你的
    LogRotateX.LogFilePath 是 `/var/log/foo/server.log`， 在 2016 年 11 月 11 日下午
    6:30 创建的备份文件名将是 `/var/log/foo/server_20161104183000.log`。

    # 清理旧日志文件

    每当创建新的日志文件时，可能会删除旧的日志文件。根据编码的时间戳，最近的文件会被保留， 最多保留数量等于 MaxFiles（如果
    MaxFiles 为 0，则保留所有文件）。 任何编码时间戳早于 MaxAge 天的文件都会被删除，无论 MaxFiles 的设置如何。
    请注意，时间戳中编码的时间是轮转时间，可能与该文件最后一次写入的时间不同。

    如果 MaxFiles 和 MaxAge 都为 0，则不会删除任何旧日志文件。

func NewLogRotateX(logFilePath string) *LogRotateX
    NewLogRotateX 创建一个新的 LogRotateX 实例，使用默认配置。

    参数:
      - logFilePath: 日志文件路径

    返回值:
      - *LogRotateX: 配置好的实例

    默认配置: MaxSize=10MB, MaxAge=0, MaxSize=0, LocalTime=true, Compress=false

func (l *LogRotateX) Close() error
    Close 关闭日志文件

    返回值:
      - error: 关闭失败时返回错误，否则返回 nil

func (l *LogRotateX) Sync() error
    Sync 强制将缓冲区数据同步到磁盘。

    返回值:
      - error: 同步失败时返回错误，否则返回 nil

func (l *LogRotateX) Write(p []byte) (n int, err error)
    Write 实现 io.Writer 接口，向日志文件写入数据。 当文件大小超过限制时自动执行轮转。

    参数:
      - p: 要写入的数据

    返回值:
      - n: 实际写入的字节数
      - err: 写入失败时返回错误

