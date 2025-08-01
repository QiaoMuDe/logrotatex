# LogRotateX - Go 日志轮转工具

[![Go Version](https://img.shields.io/badge/Go-1.24.4+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

LogRotateX 是一个高性能、线程安全的 Go 日志轮转库，提供了完整的日志文件管理功能。它可以与任何支持 `io.Writer` 接口的日志库配合使用，自动管理日志文件的大小、数量和保留时间。

## 🚀 功能特性

- 📁 **自动日志轮转** - 基于文件大小自动轮转日志文件
- 🔄 **智能文件管理** - 支持按数量和时间清理旧日志文件
- 🗜️ **ZIP 压缩支持** - 自动压缩轮转后的日志文件，节省存储空间
- ⏱️ **时间格式选择** - 支持本地时间或 UTC 时间命名备份文件
- 🛡️ **线程安全设计** - 支持并发写入，适用于高并发场景
- 🔒 **路径安全验证** - 内置路径安全检查，防止路径遍历攻击
- 🎯 **构造函数支持** - 提供 `NewLogRotateX()` 构造函数，简化初始化
- 📊 **性能优化** - 优化的文件扫描算法，支持大量日志文件场景

## 📦 安装

```bash
go get gitee.com/MM-Q/logrotatex
```

**系统要求**: Go 1.24.4 或更高版本

## 🔧 快速开始

### 基本用法

```go
package main

import (
    "log"
    "gitee.com/MM-Q/logrotatex"
)

func main() {
    // 使用构造函数创建日志轮转器（推荐方式）
    logger := logrotatex.NewLogRotateX("logs/app.log")
  
    // 可选：自定义配置
    logger.MaxSize = 100    // 100MB
    logger.MaxBackups = 5   // 保留5个备份文件
    logger.MaxAge = 30      // 保留30天
    logger.Compress = true  // 启用压缩
  
    // 设置为标准日志输出
    log.SetOutput(logger)
  
    // 写入日志
    log.Println("这是一条测试日志")
  
    // 程序退出前关闭
    defer logger.Close()
}
```

### 手动配置方式

```go
package main

import "gitee.com/MM-Q/logrotatex"

func main() {
    // 手动配置所有参数
    logger := &logrotatex.LogRotateX{
        Filename:   "logs/myapp.log",
        MaxSize:    50,     // 50MB
        MaxBackups: 3,      // 保留3个备份
        MaxAge:     7,      // 保留7天
        LocalTime:  true,   // 使用本地时间
        Compress:   true,   // 启用压缩
        FilePerm:   0600,   // 文件权限
    }
    defer logger.Close()
  
    // 直接写入
    logger.Write([]byte("直接写入的日志消息\n"))
}
```

### 与流行日志库集成

```go
// 与 logrus 集成
import (
    "github.com/sirupsen/logrus"
    "gitee.com/MM-Q/logrotatex"
)

func setupLogrus() {
    logger := logrotatex.NewLogRotateX("logs/app.log")
    logger.MaxSize = 100
    logger.Compress = true
  
    logrus.SetOutput(logger)
    logrus.SetFormatter(&logrus.JSONFormatter{})
}

// 与 zap 集成
import (
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
    "gitee.com/MM-Q/logrotatex"
)

func setupZap() *zap.Logger {
    logger := logrotatex.NewLogRotateX("logs/app.log")
    logger.MaxSize = 100
    logger.Compress = true
  
    writeSyncer := zapcore.AddSync(logger)
    encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
    core := zapcore.NewCore(encoder, writeSyncer, zapcore.InfoLevel)
  
    return zap.New(core)
}
```

## ⚙️ 配置参数

| 参数名         | 类型            | 说明                       | 默认值             |
| -------------- | --------------- | -------------------------- | ------------------ |
| `Filename`   | `string`      | 日志文件路径               | 空（使用临时目录） |
| `MaxSize`    | `int`         | 单个日志文件最大大小（MB） | 10                 |
| `MaxBackups` | `int`         | 最大保留备份文件数量       | 0（不限制）        |
| `MaxAge`     | `int`         | 最大保留天数               | 0（不限制）        |
| `LocalTime`  | `bool`        | 使用本地时间命名备份文件   | true               |
| `Compress`   | `bool`        | 是否压缩轮转后的日志       | false              |
| `FilePerm`   | `os.FileMode` | 日志文件权限               | 0600               |

### 默认配置说明

使用 `NewLogRotateX()` 构造函数创建的实例具有以下默认配置：

- **MaxSize**: 10MB - 适合大多数应用场景
- **MaxAge**: 0天 - 不自动删除历史文件，由 MaxBackups 控制
- **MaxBackups**: 0个 - 不限制备份文件数量
- **LocalTime**: true - 使用本地时间，便于查看
- **Compress**: false - 默认不压缩，可根据需要启用
- **FilePerm**: 0600 - 仅所有者可读写，保证安全性

## 📁 文件命名规则

### 当前日志文件

- 始终使用您指定的 `Filename`
- 例如：`app.log`

### 备份文件命名

- 格式：`{前缀}_{时间戳}.{扩展名}`
- 时间戳格式：`2006-01-02T15-04-05.000`
- 例如：`app_2025-01-11T10-30-00.000.log`

### 压缩文件命名

- 格式：`{备份文件名}.zip`
- 例如：`app_2025-01-11T10-30-00.000.log.zip`

## 🔄 轮转触发条件

日志轮转在以下情况下自动触发：

1. **文件大小达到限制**：当前日志文件大小 ≥ MaxSize
2. **手动轮转**：调用 `Rotate()` 方法
3. **程序重启**：如果现有文件已达到大小限制

## 🧹 清理策略

LogRotateX 使用双重清理策略：

1. **数量限制**：保留最新的 `MaxBackups` 个备份文件
2. **时间限制**：删除超过 `MaxAge` 天的文件

**注意**：两个条件同时生效，任一条件满足都会触发文件删除。

## 🛡️ 安全特性

LogRotateX 内置了多层安全防护：

- **路径遍历攻击防护**：检测并阻止 `../` 等危险路径
- **系统目录保护**：禁止访问 `/etc`、`/proc` 等敏感目录
- **文件名验证**：检查危险字符和系统保留名称
- **权限控制**：默认使用安全的文件权限（0600）
- **符号链接检查**：验证符号链接指向的真实路径

## 📊 性能特性

- **优化的文件扫描**：O(n) 时间复杂度的文件扫描算法
- **智能缓冲区**：根据文件大小自适应调整缓冲区大小
- **并发安全**：使用互斥锁保护关键操作
- **资源管理**：自动管理文件句柄，防止资源泄漏
- **异步压缩**：压缩操作在后台 goroutine 中执行

## 🔧 高级用法

### 手动轮转

```go
logger := logrotatex.NewLogRotateX("logs/app.log")
defer logger.Close()

// 手动触发轮转
err := logger.Rotate()
if err != nil {
    log.Printf("轮转失败: %v", err)
}
```

### 获取状态信息

```go
logger := logrotatex.NewLogRotateX("logs/app.log")
defer logger.Close()

// 获取当前文件大小
currentSize := logger.GetCurrentSize()
fmt.Printf("当前文件大小: %d 字节\n", currentSize)

// 获取最大文件大小
maxSize := logger.GetMaxSize()
fmt.Printf("最大文件大小: %d 字节\n", maxSize)

// 获取当前文件路径
currentFile := logger.CurrentFile()
fmt.Printf("当前文件路径: %s\n", currentFile)
```

### 强制同步到磁盘

```go
logger := logrotatex.NewLogRotateX("logs/app.log")
defer logger.Close()

// 写入数据
logger.Write([]byte("重要日志数据\n"))

// 强制同步到磁盘
err := logger.Sync()
if err != nil {
    log.Printf("同步失败: %v", err)
}
```

## 🏆 最佳实践

### 1. 生产环境配置

```go
logger := logrotatex.NewLogRotateX("logs/app.log")
logger.MaxSize = 100      // 100MB，避免单文件过大
logger.MaxBackups = 10    // 保留10个备份，控制磁盘使用
logger.MaxAge = 30        // 保留30天，满足审计要求
logger.Compress = true    // 启用压缩，节省存储空间
```

### 2. 高并发场景

```go
// 创建全局日志实例
var globalLogger *logrotatex.LogRotateX

func init() {
    globalLogger = logrotatex.NewLogRotateX("logs/app.log")
    globalLogger.MaxSize = 50
    globalLogger.MaxBackups = 5
    globalLogger.Compress = true
  
    // 设置为标准日志输出
    log.SetOutput(globalLogger)
}

// 在程序退出时清理
func cleanup() {
    if globalLogger != nil {
        globalLogger.Close()
    }
}
```

### 3. 错误处理

```go
logger := logrotatex.NewLogRotateX("logs/app.log")
defer func() {
    if err := logger.Close(); err != nil {
        log.Printf("关闭日志文件失败: %v", err)
    }
}()

// 写入时检查错误
if _, err := logger.Write([]byte("日志消息\n")); err != nil {
    log.Printf("写入日志失败: %v", err)
}
```

### 4. 监控和告警

```go
// 定期检查日志文件状态
func monitorLogFile(logger *logrotatex.LogRotateX) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
  
    for range ticker.C {
        currentSize := logger.GetCurrentSize()
        maxSize := logger.GetMaxSize()
    
        // 当文件大小接近限制时发出告警
        if float64(currentSize)/float64(maxSize) > 0.8 {
            log.Printf("警告: 日志文件大小接近限制 (%d/%d)", currentSize, maxSize)
        }
    }
}
```

## 🐛 故障排除

### 常见问题

**Q: 日志文件没有按预期轮转？**
A: 检查以下几点：

- 文件权限是否正确
- 磁盘空间是否充足
- MaxSize 设置是否合理
- 是否有其他进程占用文件

**Q: 压缩功能不工作？**
A: 确认：

- `Compress` 字段设置为 `true`
- 有足够的磁盘空间进行压缩操作
- 检查系统日志中的错误信息

**Q: 备份文件没有被清理？**
A: 检查：

- `MaxBackups` 和 `MaxAge` 的设置
- 文件名格式是否符合预期
- 目录权限是否允许删除操作

**Q: 在 Windows 上出现文件锁定问题？**
A: 这是 Windows 文件系统的特性，建议：

- 确保及时调用 `Close()` 方法
- 避免多个进程同时写入同一文件
- 考虑使用不同的文件名

### 调试技巧

1. **启用详细日志**：在测试环境中使用 `testing.Verbose()` 查看详细输出
2. **检查文件权限**：确保程序有足够权限创建和删除文件
3. **监控文件句柄**：使用系统工具监控文件句柄使用情况
4. **测试轮转逻辑**：使用小的 `MaxSize` 值快速测试轮转功能

## 🤝 贡献指南

我们欢迎社区贡献！在提交代码前，请确保：

### 开发环境设置

```bash
# 克隆仓库
git clone https://gitee.com/MM-Q/logrotatex.git
cd logrotatex

# 运行测试
go test -v ./...

# 运行基准测试
go test -bench=. -benchmem ./...
```

### 提交要求

- ✅ 通过所有单元测试
- ✅ 符合 Go 代码规范（使用 `gofmt` 和 `golint`）
- ✅ 添加必要的测试用例
- ✅ 更新相关文档
- ✅ 提供清晰的提交信息

### 测试覆盖率

当前测试覆盖了以下场景：

- 基本读写操作
- 日志轮转逻辑
- 文件压缩功能
- 并发安全性
- 错误处理
- 边界条件
- 性能测试

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

## 🙏 致谢

本项目基于 [natefinch/lumberjack](https://github.com/natefinch/lumberjack) 库的 v2 分支进行开发和扩展。我们对原作者 **Nate Finch** 及其团队的杰出工作表示诚挚的感谢！

lumberjack 是一个优秀的 Go 日志轮转库，为我们提供了坚实的基础。在此基础上，LogRotateX 进行了以下主要改进和扩展：

- 🔧 **构造函数支持** - 添加了 `NewLogRotateX()` 构造函数，简化初始化过程
- 🛡️ **增强安全特性** - 内置路径安全验证，防止路径遍历攻击
- 📊 **性能优化** - 优化文件扫描算法，提升大量文件场景下的性能
- 🗜️ **ZIP 压缩** - 改进压缩格式为 ZIP，提供更好的兼容性
- 🔒 **权限控制** - 增加文件权限配置选项
- 🌐 **本地化支持** - 提供中文文档和更好的本地化体验

我们深深感谢开源社区的贡献精神，也希望 LogRotateX 能够继续为 Go 开发者社区提供价值。

### 原始项目信息

- **原项目地址**: https://github.com/natefinch/lumberjack
- **原作者**: Nate Finch
- **基于分支**: v2
- **原项目许可**: MIT License

## 🔗 相关链接

- [API 文档](APIDOC.md) - 详细的 API 参考文档
- [Gitee 仓库](https://gitee.com/MM-Q/logrotatex) - 源代码仓库
- [问题反馈](https://gitee.com/MM-Q/logrotatex/issues) - 报告 Bug 或提出建议
- [原始项目 lumberjack](https://github.com/natefinch/lumberjack) - 致敬原作者

---

**LogRotateX** - 让日志管理变得简单高效！ 🚀
