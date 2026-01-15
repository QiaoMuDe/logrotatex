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

默认配置：
- **缓冲区大小**：64KB - 平衡内存使用和刷新频率
- **最大写入次数**：500次 - 避免过度频繁的刷新操作  
- **刷新间隔**：1秒 - 内置定时刷新器，确保数据及时写入

**定时刷新器特性**：
- 后台定时器协程每秒检查并刷新缓冲区
- 防止数据长时间滞留，确保低写入频率场景下的数据及时性
- 内置 panic 恢复机制，保障定时器稳定运行
- 支持优雅关闭，停止定时器并刷新剩余数据

### BufferedWriter

带缓冲批量写入器，包装写入器和关闭器，提供批量写入功能。内置定时刷新器确保数据及时写入。

```go
type BufferedWriter struct {
	// Has unexported fields.
}
```

**核心特性**：
- **三重触发刷新**：缓冲区大小、写入次数、时间间隔
- **定时刷新器**：后台协程定期刷新，防止数据滞留
- **线程安全**：支持并发写入，内部使用同步机制
- **错误恢复**：定时器协程内置 panic 恢复机制

#### NewBufferedWriter

创建新的带缓冲批量写入器，自动启动定时刷新器

```go
func NewBufferedWriter(wc io.WriteCloser, config *BufCfg) *BufferedWriter
```

- 参数：
  - `wc`：底层写入+关闭器（必需）
  - `config`：配置（可选，为空则使用默认值）
- 返回值：新的带缓冲批量写入器实例

**创建过程**：
1. 根据配置创建缓冲区（使用默认值或自定义配置）
2. 启动定时刷新器协程，按配置的时间间隔定期刷新
3. 初始化同步机制，确保线程安全
4. 返回配置完成的 BufferedWriter 实例

**定时刷新器**：
- 在后台独立协程中运行，不影响主写入流程
- 根据 `FlushInterval` 配置定期执行刷新操作
- 内置错误处理和恢复机制
- 调用 `Close()` 时自动停止并清理

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
	LogFilePath   string                `json:"logfilepath" yaml:"logfilepath"`   // 日志文件路径
	Async         bool                  `json:"async" yaml:"async"`             // 是否启用异步清理
	MaxSize       int                   `json:"maxsize" yaml:"maxsize"`         // 单个日志文件最大大小（MB）
	MaxAge        int                   `json:"maxage" yaml:"maxage"`           // 保留日志文件天数
	MaxFiles      int                   `json:"maxfiles" yaml:"maxfiles"`       // 最大保留历史日志文件数量
	LocalTime     bool                  `json:"localtime" yaml:"localtime"`     // 是否使用本地时间记录轮转时间
	Compress      bool                  `json:"compress" yaml:"compress"`       // 轮转后日志文件是否压缩
	DateDirLayout bool                  `json:"datedirlayout" yaml:"datedirlayout"` // 是否启用按日期目录存放轮转后的日志
	RotateByDay   bool                  `json:"rotatebyday" yaml:"rotatebyday"`   // 是否启用按天轮转
	CompressType  comprx.CompressType   `json:"compress_type" yaml:"compress_type"` // 压缩类型，默认为zip格式
	// Has unexported fields.
}
```

**字段说明**：

- `LogFilePath`：日志文件路径。如果为空，则使用 `os.TempDir()` 下的 `<程序名>_logrotatex.log`
- `Async`：是否启用异步清理。true 表示异步清理，false 表示同步清理（默认）
- `MaxSize`：单个日志文件最大大小（MB）。超过此大小的日志文件将被轮转（默认 10MB）
- `MaxAge`：保留日志文件天数。超过此天数的文件将被删除（默认 0，表示不删除）
- `MaxFiles`：最大保留历史日志文件数量。超过此数量的旧文件将被删除（默认 0，表示不限制）
- `LocalTime`：是否使用本地时间记录轮转时间。false 使用 UTC 时间（默认 true）
- `Compress`：轮转后日志文件是否压缩。true 表示压缩，false 表示不压缩（默认 false）
- `DateDirLayout`：是否启用按日期目录存放轮转后的日志。true 表示按 `YYYY-MM-DD/` 目录存放，false 表示存放在当前目录（默认 false）
- `RotateByDay`：是否启用按天轮转。true 表示每天自动轮转一次（跨天时触发），false 表示只按文件大小轮转（默认 false）
- `CompressType`：压缩类型，默认为 `comprx.CompressTypeZip`。支持的压缩格式包括：
  - `comprx.CompressTypeZip`：zip 压缩格式
  - `comprx.CompressTypeTar`：tar 压缩格式
  - `comprx.CompressTypeTgz`：tgz 压缩格式
  - `comprx.CompressTypeTarGz`：tar.gz 压缩格式
  - `comprx.CompressTypeGz`：gz 压缩格式
  - `comprx.CompressTypeBz2`：bz2 压缩格式
  - `comprx.CompressTypeBzip2`：bzip2 压缩格式
  - `comprx.CompressTypeZlib`：zlib 压缩格式

**按天轮转特性**：
- **自动轮转**：每天自动轮转一次，跨天时触发
- **双重触发**：可以同时设置按大小轮转，满足任一条件即轮转
- **时间控制**：支持 `LocalTime` 配置，使用本地时间或 UTC 时间
- **灵活组合**：可与 `DateDirLayout` 组合使用，按日期目录存放备份文件

#### NewLogRotateX

创建新的 `LogRotateX` 实例，使用默认配置

```go
func NewLogRotateX(logFilePath string) *LogRotateX
```

- 参数：`logFilePath` - 日志文件路径
- 返回值：配置好的 `LogRotateX` 实例

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