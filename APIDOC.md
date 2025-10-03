# Package logrotatex

Package logrotatex 提供日志轮转功能，管理日志文件大小和数量，是轻量级组件，可与支持 `io.Writer` 接口的日志库配合使用。

主要功能：
- 自动轮转日志文件，防止单个文件过大
- 支持设置最大文件大小、保留文件数量和保留天数
- 支持日志文件压缩
- 线程安全设计，适用于并发环境

注意事项：
- 假设只有一个进程向输出文件写入日志
- 多进程使用相同配置可能导致异常行为

## Variables

- `NewBW`：`NewBufferedWriter` 简写，创建 `BufferedWriter` 实例
- `NewLRX`：`NewLogRotateX` 简写，创建 `LogRotateX` 实例

## Functions

### WrapWriter

将 `io.Writer` 包装为不可关闭的 `io.WriteCloser`

```go
func WrapWriter(w io.Writer) io.WriteCloser
```

- 参数：`w` - 要包装的 `io.Writer`
- 返回值：不可关闭的 `WriteCloser` 包装器

## Types

### BufCfg

缓冲写入器配置

```go
type BufCfg struct {
	MaxBufferSize int           // 最大缓冲区大小，默认64KB
	MaxWriteCount int           // 最大写入次数，默认500次
	FlushInterval time.Duration // 刷新间隔，默认1秒
}
```

#### DefBufCfg

获取默认缓冲写入器配置

```go
func DefBufCfg() *BufCfg
```

注意事项：默认缓冲区大小64KB，最大写入次数500次，刷新间隔1秒

### BufferedWriter

带缓冲批量写入器，包装写入器和关闭器，提供批量写入功能

```go
type BufferedWriter struct {
	// Has unexported fields.
}
```

#### NewBufferedWriter

创建新的带缓冲批量写入器

```go
func NewBufferedWriter(wc io.WriteCloser, config *BufCfg) *BufferedWriter
```

- 参数：
  - `wc`：底层写入+关闭器（必需）
  - `config`：配置（可选，为空则使用默认值）
- 返回值：新的带缓冲批量写入器实例

#### NewStdoutBW

创建面向标准输出的带缓冲写入器，不会关闭 stdout

```go
func NewStdoutBW(config *BufCfg) *BufferedWriter
```

注意事项：调用 `Close()` 不会关闭标准输出，适合长期运行场景

#### BufferSize

返回当前缓冲区中的字节数

```go
func (bw *BufferedWriter) BufferSize() int
```

#### Close

关闭缓冲写入器

```go
func (bw *BufferedWriter) Close() error
```

#### Flush

手动刷新缓冲区

```go
func (bw *BufferedWriter) Flush() error
```

#### IsClosed

返回缓冲写入器是否已关闭

```go
func (bw *BufferedWriter) IsClosed() bool
```

#### TimeSinceLastFlush

返回距离上次刷新的时间

```go
func (bw *BufferedWriter) TimeSinceLastFlush() time.Duration
```

#### Write

将数据写入缓冲区，达到刷新条件时自动批量写入

```go
func (bw *BufferedWriter) Write(p []byte) (n int, err error)
```

- 参数：`p` - 要写入的数据
- 返回值：
  - `n`：实际写入的字节数
  - `err`：写入错误（如果有）

#### WriteCount

返回当前缓冲区中的写入次数

```go
func (bw *BufferedWriter) WriteCount() int
```

### LogRotateX

实现日志轮转功能的 `io.WriteCloser`

```go
type LogRotateX struct {
	LogFilePath string `json:"logfilepath" yaml:"logfilepath"` // 日志文件路径
	Async       bool   `json:"async" yaml:"async"`             // 是否启用异步清理
	MaxSize     int    `json:"maxsize" yaml:"maxsize"`         // 单个日志文件最大大小（MB）
	MaxAge      int    `json:"maxage" yaml:"maxage"`           // 保留日志文件天数
	MaxFiles    int    `json:"maxfiles" yaml:"maxfiles"`       // 最大保留历史日志文件数量
	LocalTime   bool   `json:"localtime" yaml:"localtime"`     // 是否使用本地时间记录轮转时间
	Compress    bool   `json:"compress" yaml:"compress"`       // 轮转后日志文件是否压缩
	// Has unexported fields.
}
```

#### NewLogRotateX

创建新的 `LogRotateX` 实例，使用默认配置

```go
func NewLogRotateX(logFilePath string) *LogRotateX
```

- 参数：`logFilePath` - 日志文件路径
- 返回值：配置好的 `LogRotateX` 实例
- 默认配置：`MaxSize=10MB`，`MaxAge=0`，`MaxFiles=0`，`LocalTime=true`，`Compress=false`

#### Close

关闭日志文件

```go
func (l *LogRotateX) Close() error
```

- 返回值：关闭失败返回错误，成功返回 nil

#### Sync

强制将缓冲区数据同步到磁盘

```go
func (l *LogRotateX) Sync() error
```

- 返回值：同步失败返回错误，成功返回 nil

#### Write

向日志文件写入数据，文件大小超过限制时自动轮转

```go
func (l *LogRotateX) Write(p []byte) (n int, err error)
```

- 参数：`p` - 要写入的数据
- 返回值：
  - `n`：实际写入的字节数
  - `err`：写入失败返回错误