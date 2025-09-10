# LogRotateX API 文档

## 包信息

**包名**: `logrotatex`
**导入路径**: `gitee.com/MM-Q/logrotatex`

## 包描述

Package logrotatex 提供了一个日志轮转功能的实现，用于管理日志文件的大小和数量。它是一个轻量级的组件，可以与任何支持 `io.Writer` 接口的日志库配合使用。

### 主要功能

- 自动轮转日志文件，防止单个文件过大
- 支持设置最大文件大小、保留文件数量和保留天数
- 支持日志文件压缩
- 线程安全的设计，适用于并发环境

### 注意事项

- 假设只有一个进程在向输出文件写入日志
- 多个进程使用相同的配置可能导致异常行为

## 目录

- [类型定义](#类型定义)
  - [LogRotateX](#logrotatex-结构体)
- [构造函数](#构造函数)
  - [NewLogRotateX](#newlogrotatex)
- [方法](#方法)
  - [Write](#write)
  - [Close](#close)
  - [Rotate](#rotate)
  - [Sync](#sync)
  - [CurrentFile](#currentfile)
  - [GetCurrentSize](#getcurrentsize)
  - [GetMaxSize](#getmaxsize)

---

## 类型定义

### LogRotateX 结构体

```go
type LogRotateX struct {
    // Filename 是写入日志的文件。备份日志文件将保留在同一目录中。
	  // 如果该值为空，则使用 os.TempDir() 下的 <程序名>_logrotatex.log。
	  Filename string `json:"filename" yaml:"filename"`
  
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
```

**描述**: LogRotateX 是一个 `io.WriteCloser`，它会将日志写入指定的文件名。

#### 工作原理

首次调用 `Write` 方法时，LogRotateX 会打开或创建日志文件。如果文件已存在且大小小于 `MaxSize` 兆字节，logrotatex 会打开该文件并追加写入。如果文件已存在且大小大于或等于 `MaxSize` 兆字节，该文件会被重命名，重命名时会在文件名的扩展名之前（如果没有扩展名，则在文件名末尾）插入当前时间戳。然后会使用原始文件名创建一个新的日志文件。

每当写入操作会导致当前日志文件大小超过 `MaxSize` 兆字节时，当前文件会被关闭、重命名，并使用原始文件名创建一个新的日志文件。因此，你提供给 LogRotateX 的文件名始终是"当前"的日志文件。

#### 备份文件命名规则

备份文件使用提供给 LogRotateX 的日志文件名，格式为 `name-timestamp.ext`，其中：

- `name` 是不带扩展名的文件名
- `timestamp` 是日志轮转时的时间，格式为 `20060102150405`
- `ext` 是原始扩展名

**示例**: 如果你的 `LogRotateX.Filename` 是 `/var/log/foo/server.log`，在 2016 年 11 月 11 日下午 6:30 创建的备份文件名将是 `/var/log/foo/server-2016-11-04T18-30-00.000.log`。

#### 清理旧日志文件

每当创建新的日志文件时，可能会删除旧的日志文件。根据编码的时间戳，最近的文件会被保留，最多保留数量等于 `MaxFiles`（如果 `MaxFiles` 为 0，则保留所有文件）。任何编码时间戳早于 `MaxAge` 天的文件都会被删除，无论 `MaxFiles` 的设置如何。

> **注意**: 时间戳中编码的时间是轮转时间，可能与该文件最后一次写入的时间不同。

如果 `MaxFiles` 和 `MaxAge` 都为 0，则不会删除任何旧日志文件。

---

## 构造函数

### New (推荐)

```go
var New = NewLogRotateX
```

**描述**: `NewLogRotateX` 的简写形式，用于创建新的 LogRotateX 实例。这是一个函数变量别名，提供了更简洁的构造函数调用方式。

#### 使用方式

```go
// 使用简写形式（推荐）
logger := logrotatex.New("logs/app.log")

// 等价于完整形式
logger := logrotatex.NewLogRotateX("logs/app.log")
```

#### 参数

- `filename string`: 日志文件的路径，会进行安全验证和清理

#### 返回值

- `*LogRotateX`: 配置好的 LogRotateX 实例，使用默认配置

#### 优势

- **简洁性**: 符合 Go 语言标准库的命名惯例
- **易用性**: 更短的函数名，提升代码可读性
- **兼容性**: 与 `NewLogRotateX` 完全等价，零性能开销
- **标准化**: 遵循 Go 社区的最佳实践

### NewLogRotateX

```go
func NewLogRotateX(filename string) *LogRotateX
```

**描述**: 创建一个新的 LogRotateX 实例，使用指定的文件路径和合理的默认配置。

该构造函数会验证和清理文件路径，确保路径安全性，并设置推荐的默认值。如果路径不安全或创建失败，此函数会立即 panic，确保问题能够快速被发现。

> **注意**: 推荐使用 `New` 简写形式，它更符合 Go 语言的命名惯例。

#### 参数

- `filename string`: 日志文件的路径，会进行安全验证和清理

#### 返回值

- `*LogRotateX`: 配置好的 LogRotateX 实例

#### 默认配置

| 配置项     | 默认值 | 说明                                      |
| ---------- | ------ | ----------------------------------------- |
| MaxSize    | 10MB   | 单个日志文件最大大小                      |
| MaxAge     | 0天    | 日志文件最大保留时间，0表示不清理历史文件 |
| MaxFiles   | 0个    | 最大历史文件数量，0表示不限制文件数量     |
| LocalTime  | true   | 使用本地时间                              |
| Compress   | false  | 禁用压缩                                  |

> **注意**: 如果文件路径不安全或创建失败，此函数会 panic

---

## 方法

### Write

```go
func (l *LogRotateX) Write(p []byte) (n int, err error)
```

**描述**: 实现了 `io.Writer` 接口，用于向日志文件写入数据。该方法会处理日志轮转逻辑，确保单个日志文件不会超过设定的最大大小。

#### 参数

- `p []byte`: 要写入的日志数据

#### 返回值

- `n int`: 实际写入的字节数
- `err error`: 如果写入过程中发生错误，则返回该错误；否则返回 nil

---

### Close

```go
func (l *LogRotateX) Close() error
```

**描述**: 关闭日志记录器。

该方法会关闭当前打开的日志文件，释放相关资源，并停止后台goroutine。此操作是线程安全的，使用 `sync.Once` 防止重复调用，并通过上下文控制超时。在异常情况下确保文件句柄正确关闭，防止资源泄漏。

#### 返回值

- `error`: 如果在关闭文件时发生错误，则返回该错误；否则返回 nil

---

### Rotate

```go
func (l *LogRotateX) Rotate() error
```

**描述**: 执行日志文件的轮转操作。

该方法会关闭当前日志文件，将其重命名为带有时间戳的备份文件，然后创建一个新的日志文件用于后续写入。此操作是线程安全的，使用互斥锁保护。

#### 返回值

- `error`: 如果在执行轮转操作时发生错误，则返回该错误；否则返回 nil

---

### Sync

```go
func (l *LogRotateX) Sync() error
```

**描述**: 强制将缓冲区数据同步到磁盘。

该方法会调用底层文件的 `Sync()` 方法，确保所有写入的数据都被刷新到磁盘上，而不是仅仅保存在操作系统的缓冲区中。这对于确保数据持久性很重要，特别是在系统可能意外关闭的情况下。

#### 注意事项

- 此操作是线程安全的，使用互斥锁保护
- 如果当前没有打开的文件，则直接返回 nil
- 频繁调用此方法可能会影响写入性能

#### 返回值

- `error`: 如果同步操作失败，则返回相应的错误；否则返回 nil

---

## 使用示例

```go
package main

import (
    "log"
    "gitee.com/MM-Q/logrotatex"
)

func main() {
    // 创建日志轮转器
    rotator := logrotatex.NewLogRotateX("/var/log/myapp.log")
  
    // 配置参数（可选）
    rotator.MaxSize = 100    // 100MB
    rotator.MaxFiles = 3     // 保留3个历史文件
    rotator.MaxAge = 28      // 保留28天
    rotator.Compress = true  // 启用压缩
  
    // 设置为日志输出
    log.SetOutput(rotator)
  
    // 使用日志
    log.Println("这是一条测试日志")
  
    // 程序结束时关闭
    defer rotator.Close()
}
```
