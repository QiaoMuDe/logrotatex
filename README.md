<div align="center">

# 🔄 LogRotateX - Go 日志轮转工具

[![Go Version](https://img.shields.io/badge/Go-1.24.4+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Release](https://img.shields.io/badge/Release-v1.0.0-brightgreen.svg)](https://gitee.com/MM-Q/logrotatex/releases)
[![Go Report Card](https://img.shields.io/badge/Go%20Report-A+-brightgreen.svg)](https://goreportcard.com/report/gitee.com/MM-Q/logrotatex)
[![Documentation](https://img.shields.io/badge/Documentation-Available-blue.svg)](APIDOC.md)

**高性能、线程安全的 Go 日志轮转库，提供完整的日志文件管理功能**

[🚀 快速开始](#-快速开始) • [📖 文档](APIDOC.md) • [💡 示例](#-使用示例) • [🤝 贡献](#-贡献指南) • [📄 许可证](#-许可证)

---

</div>

## 📋 项目简介

LogRotateX 是一个专为 Go 语言设计的高性能日志轮转库，基于 [natefinch/lumberjack](https://github.com/natefinch/lumberjack) 进行深度优化和功能扩展。它可以与任何支持 `io.Writer` 接口的日志库无缝集成，自动管理日志文件的大小、数量和保留时间，为您的应用提供可靠的日志管理解决方案。

## ✨ 核心特性

<table>
<tr>
<td width="50%">

### 🔄 智能轮转管理
- 📁 **自动日志轮转** - 基于文件大小智能轮转
- 🗂️ **多重清理策略** - 按数量和时间双重管理
- 🗜️ **ZIP 压缩支持** - 自动压缩节省存储空间
- ⏱️ **灵活时间格式** - 支持本地时间/UTC时间

</td>
<td width="50%">

### 🛡️ 安全与性能
- 🔒 **路径安全验证** - 防止路径遍历攻击
- 🚀 **高并发支持** - 线程安全的并发写入
- 📊 **性能优化** - O(n)文件扫描算法
- 🎯 **简化初始化** - 便捷的构造函数

</td>
</tr>
<tr>
<td colspan="2">

### 🚀 缓冲写入器 (BufferedWriter)
- 📦 **批量写入** - 三重触发条件智能刷新
- ⚡ **性能提升** - 减少系统调用开销
- 🔧 **通用设计** - 支持任意 io.WriteCloser
- ⏱️ **实时控制** - 缓冲区大小、写入次数、刷新间隔三重保障

</td>
</tr>
</table>

### 🌟 主要优势

| 特性 | 描述 | 优势 |
|------|------|------|
| 🔌 **无缝集成** | 实现 `io.Writer` 接口 | 兼容所有主流日志库 |
| ⚡ **高性能** | 优化的文件操作算法 | 支持高频日志写入场景 |
| 🚀 **缓冲写入** | 带缓冲批量写入器 | 显著提升写入性能，减少系统调用 |
| 🛡️ **企业级安全** | 多层安全防护机制 | 防止安全漏洞和攻击 |
| 🔧 **灵活配置** | 丰富的配置选项 | 适应各种使用场景 |
| 📈 **生产就绪** | 经过充分测试验证 | 可直接用于生产环境 |

## 📦 安装指南

### 🚀 从仓库安装

```bash
# 安装最新版本
go get gitee.com/MM-Q/logrotatex

# 安装指定版本
go get gitee.com/MM-Q/logrotatex@v1.0.0

# 更新到最新版本
go get -u gitee.com/MM-Q/logrotatex
```

### 📋 系统要求

| 项目 | 要求 |
|------|------|
| **Go 版本** | 1.24.4+ |
| **操作系统** | Linux, macOS, Windows |
| **架构** | amd64, arm64 |
| **依赖** | 无外部依赖 |

### ✅ 安装验证

```bash
# 创建测试文件
cat > test_install.go << 'EOF'
package main

import (
    "fmt"
    "gitee.com/MM-Q/logrotatex"
)

func main() {
    logger := logrotatex.NewLogRotateX("test.log")
    defer logger.Close()
    
    logger.Write([]byte("安装成功！\n"))
    fmt.Println("LogRotateX 安装验证成功！")
}
EOF

# 运行测试
go run test_install.go
```

## 🚀 快速开始

### 🎯 30秒快速体验

```go
package main

import (
    "log"
    "gitee.com/MM-Q/logrotatex"
)

func main() {
    // 一行代码创建日志轮转器
    logger := logrotatex.NewLogRotateX("logs/app.log")
    defer logger.Close()
    
    // 开始使用
    logger.Write([]byte("Hello LogRotateX! 🎉\n"))
}
```

## 💡 使用示例

### 📝 基础用法

<details>
<summary><b>🔧 推荐配置（点击展开）</b></summary>

```go
package main

import (
    "log"
    "gitee.com/MM-Q/logrotatex"
)

func main() {
    // 使用构造函数创建（推荐方式）
    logger := logrotatex.NewLogRotateX("logs/app.log")
    defer logger.Close()
    
    // 生产环境推荐配置
    logger.MaxSize = 100    // 100MB - 避免单文件过大
    logger.MaxFiles = 10  // 保留10个历史文件 - 控制磁盘使用
    logger.MaxAge = 30      // 保留30天 - 满足审计要求
    logger.Compress = true  // 启用压缩 - 节省存储空间
    
    // 设置为标准日志输出
    log.SetOutput(logger)
    
    // 直接使用Write接口写入日志
    logger.Write([]byte("应用启动成功
"))
    
    // 或者通过标准log包写入（内部调用Write方法）
    log.SetOutput(logger)
    log.Println("这条日志会通过Write方法写入")
}
```

</details>

<details>
<summary><b>⚙️ 手动配置方式（点击展开）</b></summary>

```go
package main

import "gitee.com/MM-Q/logrotatex"

func main() {
    // 完全自定义配置
    logger := &logrotatex.LogRotateX{
        LogFilePath:   "logs/custom.log",
        MaxSize:    50,     // 50MB
        MaxFiles: 5,      // 保留5个历史文件
        MaxAge:     14,     // 保留14天
        LocalTime:  true,   // 使用本地时间
        Compress:   true,   // 启用压缩
        FilePerm:   0644,   // 自定义文件权限
    }
    defer logger.Close()
    
    // 直接写入
    logger.Write([]byte("自定义配置的日志消息\n"))
}
```

</details>

### 🚀 高性能缓冲写入

<details>
<summary><b>⚡ 缓冲写入器基础用法（点击展开）</b></summary>

```go
package main

import (
    "log"
    "gitee.com/MM-Q/logrotatex"
)

func main() {
    // 创建日志轮转器
    logger := logrotatex.NewLogRotateX("logs/app.log")
    
    // 创建缓冲写入器，显著提升性能
    buffered := logrotatex.NewBufferedWriter(logger, DefBufCfg()) // 使用默认配置
    defer buffered.Close()
    
    // 高性能批量写入
    for i := 0; i < 1000; i++ {
        buffered.Write([]byte("高性能日志消息
"))
    }
    // 自动批量刷新，减少系统调用
}
```

</details>

<details>
<summary><b>🔧 自定义缓冲配置（点击展开）</b></summary>

```go
package main

import (
    "gitee.com/MM-Q/logrotatex"
)

func main() {
    // 创建日志轮转器
    logger := logrotatex.NewLogRotateX("logs/app.log")
    
    // 自定义缓冲配置
    config := &logrotatex.BufCfg{
        MaxBufferSize: 128 * 1024,                      // 128KB 缓冲区
        MaxWriteCount:   1000,                          // 1000条写入次数
        FlushInterval: 500 * time.Millisecond,          // 500ms 刷新间隔
    }
    
    // 创建缓冲写入器
    buffered := logrotatex.NewBufferedWriter(logger, config)
    defer buffered.Close()
    
    // 高频写入场景
    for i := 0; i < 10000; i++ {
        buffered.Write([]byte("大量日志数据写入测试
"))
    }
    
    // 手动刷新缓冲区
    buffered.Flush()
}
```

</details>

<details>
<summary><b>📊 性能监控示例（点击展开）</b></summary>

```go
package main

import (
    "fmt"
    "time"
    "gitee.com/MM-Q/logrotatex"
)

func main() {
    logger := logrotatex.NewLogRotateX("logs/app.log")
    buffered := logrotatex.NewBufferedWriter(logger, nil)
    defer buffered.Close()
    
    // 性能测试
    start := time.Now()
    
    for i := 0; i < 50000; i++ {
        buffered.Write([]byte("性能测试日志消息
"))
        
        // 每10000条检查状态
        if (i+1)%10000 == 0 {
            fmt.Printf("已写入 %d 条，缓冲区大小: %d 字节，日志计数: %d
", 
                i+1, buffered.BufferSize(), buffered.WriteCount())
        }
    }
    
    elapsed := time.Since(start)
    fmt.Printf("写入50000条日志耗时: %v
", elapsed)
}
```

</details>

### 🔌 与主流日志库集成

<details>
<summary><b>📊 Logrus 集成示例（点击展开）</b></summary>

```go
package main

import (
    "fmt"
    "github.com/sirupsen/logrus"
    "gitee.com/MM-Q/logrotatex"
)

func main() {
    // 创建轮转器
    rotator := logrotatex.NewLogRotateX("logs/app.log")
    rotator.MaxFiles = 100
    rotator.MaxFiles = 5
    rotator.Compress = true
    defer rotator.Close()
    
    // 配置 logrus
    logrus.SetOutput(rotator)
    logrus.SetFormatter(&logrus.JSONFormatter{
        TimestampFormat: "2006-01-02 15:04:05",
    })
    logrus.SetLevel(logrus.InfoLevel)
    
    // 使用结构化日志
    logrus.WithFields(logrus.Fields{
        "service": "user-api",
        "version": "v1.2.3",
    }).Info("服务启动成功")
    
    logrus.WithError(fmt.Errorf("示例错误")).Error("错误日志示例")
}
```

</details>

<details>
<summary><b>⚡ Zap 集成示例（点击展开）</b></summary>

```go
package main

import (
    "fmt"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
    "gitee.com/MM-Q/logrotatex"
)

func setupZapLogger() *zap.Logger {
    // 创建轮转器
    rotator := logrotatex.NewLogRotateX("logs/app.log")
    rotator.MaxSize = 100
    rotator.MaxSize = 10
    rotator.MaxAge = 30
    rotator.Compress = true
    
    // 配置编码器
    encoderConfig := zap.NewProductionEncoderConfig()
    encoderConfig.TimeKey = "timestamp"
    encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
    
    // 创建核心
    core := zapcore.NewCore(
        zapcore.NewJSONEncoder(encoderConfig),
        zapcore.AddSync(rotator),
        zapcore.InfoLevel,
    )
    
    return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

func main() {
    logger := setupZapLogger()
    defer logger.Sync()
    
    // 使用结构化日志
    logger.Info("应用启动",
        zap.String("service", "user-api"),
        zap.String("version", "v1.2.3"),
        zap.Int("port", 8080),
    )
    
    logger.Error("数据库连接失败",
        zap.String("database", "mysql"),
        zap.String("host", "localhost:3306"),
        zap.Error(fmt.Errorf("connection timeout")),
    )
}
```

</details>

### 🔧 高级用法示例

<details>
<summary><b>🎛️ 运行时控制（点击展开）</b></summary>

```go
package main

import (
    "fmt"
    "log"
    "time"
    "gitee.com/MM-Q/logrotatex"
)

func main() {
    logger := logrotatex.NewLogRotateX("logs/app.log")
    logger.MaxSize = 1 // 1MB，便于测试
    defer logger.Close()
    
    log.SetOutput(logger)
    
    // 获取状态信息
    fmt.Printf("当前文件: %s\n", logger.CurrentFile())
    fmt.Printf("当前大小: %d 字节\n", logger.GetCurrentSize())
    fmt.Printf("最大大小: %d 字节\n", logger.GetMaxSize())
    
    // 写入大量日志触发轮转
    for i := 0; i < 1000; i++ {
        log.Printf("这是第 %d 条日志消息，时间: %s", i+1, time.Now().Format("2006-01-02 15:04:05"))
        
        // 每100条检查一次状态
        if (i+1)%100 == 0 {
            fmt.Printf("已写入 %d 条，当前文件大小: %d 字节\n", i+1, logger.GetCurrentSize())
        }
    }
    
    // 强制同步到磁盘
    if err := logger.Sync(); err != nil {
        log.Printf("同步失败: %v", err)
    }
}
```

</details>

## 📖 文档

- 详细 API、功能/格式与配置项请参见 [APIDOC.md](APIDOC.md)

### 🎯 推荐配置场景

<details>
<summary><b>🏢 企业生产环境</b></summary>

```go
logger := logrotatex.NewLogRotateX("logs/production.log")
logger.MaxSize = 100      // 100MB - 平衡性能和管理
logger.MaxFiles = 30    // 30个历史文件 - 满足审计要求
logger.MaxAge = 90        // 90天 - 符合合规要求
logger.Compress = true    // 启用压缩 - 节省存储
```

</details>

<details>
<summary><b>🔬 开发测试环境</b></summary>

```go
logger := logrotatex.NewLogRotateX("logs/dev.log")
logger.MaxSize = 10       // 10MB - 快速轮转便于测试
logger.MaxFiles = 3     // 3个历史文件 - 节省空间
logger.MaxAge = 7         // 7天 - 短期保留
logger.Compress = false   // 不压缩 - 便于查看
```

</details>

<details>
<summary><b>☁️ 云原生环境</b></summary>

```go
logger := logrotatex.NewLogRotateX("logs/cloud.log")
logger.MaxSize = 50       // 50MB - 适合容器环境
logger.MaxFiles = 5     // 5个历史文件 - 控制存储使用
logger.MaxAge = 14        // 14天 - 配合日志收集系统
logger.Compress = true    // 启用压缩 - 减少网络传输
```

</details>

## 🧪 测试说明

### 🚀 运行测试

```bash
# 运行所有测试
go test -v ./...

# 运行单元测试
go test -v ./tests -run TestUnit

# 运行集成测试
go test -v ./tests -run TestIntegration

# 运行性能测试
go test -bench=. -benchmem ./tests

# 生成测试覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### 📊 测试覆盖范围

| 测试类型 | 覆盖范围 | 测试文件 |
|----------|----------|----------|
| **单元测试** | 核心功能逻辑 | `*_test.go` |
| **集成测试** | 端到端场景 | `integration_test.go` |
| **性能测试** | 性能基准 | `benchmark_test.go` |
| **并发测试** | 线程安全性 | `concurrent_test.go` |

### ✅ 测试场景

<details>
<summary><b>🔧 功能测试场景</b></summary>

- ✅ 基本写入操作
- ✅ 文件轮转触发
- ✅ 备份文件管理
- ✅ 压缩功能验证
- ✅ 权限设置检查
- ✅ 错误处理验证
- ✅ 边界条件测试

</details>

<details>
<summary><b>🚀 性能测试场景</b></summary>

- ✅ 高频写入性能
- ✅ 大文件处理能力
- ✅ 内存使用效率
- ✅ 并发写入性能
- ✅ 轮转操作耗时
- ✅ 压缩操作性能

</details>

<details>
<summary><b>🛡️ 安全测试场景</b></summary>

- ✅ 路径遍历攻击防护
- ✅ 文件权限验证
- ✅ 符号链接检查
- ✅ 恶意文件名过滤
- ✅ 资源泄漏检测

</details>

## 🏆 最佳实践

### 1. 🏢 生产环境配置

```go
logger := logrotatex.NewLogRotateX("logs/app.log")
logger.MaxSize = 100      // 100MB，避免单文件过大
logger.MaxFiles = 10    // 保留10个历史文件，控制磁盘使用
logger.MaxAge = 30        // 保留30天，满足审计要求
logger.Compress = true    // 启用压缩，节省存储空间
```

### 2. 🚀 高并发场景

```go
// 创建全局日志实例
var globalLogger *logrotatex.LogRotateX

func init() {
    globalLogger = logrotatex.NewLogRotateX("logs/app.log")
    globalLogger.MaxSize = 50
    globalLogger.MaxSize = 5
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

### 3. 🛡️ 错误处理

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

### 4. 📊 监控和告警

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

### ❓ 常见问题

<details>
<summary><b>Q: 日志文件没有按预期轮转？</b></summary>

**A: 检查以下几点：**

- ✅ 文件权限是否正确
- ✅ 磁盘空间是否充足
- ✅ MaxSize 设置是否合理
- ✅ 是否有其他进程占用文件

```bash
# 检查文件权限
ls -la logs/

# 检查磁盘空间
df -h

# 检查进程占用
lsof logs/app.log
```

</details>

<details>
<summary><b>Q: 压缩功能不工作？</b></summary>

**A: 确认以下设置：**

- ✅ `Compress` 字段设置为 `true`
- ✅ 有足够的磁盘空间进行压缩操作
- ✅ 检查系统日志中的错误信息

```go
// 启用详细错误日志
logger.Compress = true
if err := logger.Rotate(); err != nil {
    log.Printf("轮转失败: %v", err)
}
```

</details>

<details>
<summary><b>Q: 备份文件没有被清理？</b></summary>

**A: 检查配置：**

- ✅ `MaxSize` 和 `MaxAge` 的设置
- ✅ 文件名格式是否符合预期
- ✅ 目录权限是否允许删除操作

```go
// 调试清理逻辑
logger.MaxFiles = 5  // 明确设置历史文件数量
logger.MaxAge = 7      // 明确设置保留天数
```

</details>

### 🔧 调试技巧

1. **启用详细日志**：在测试环境中使用详细输出
2. **检查文件权限**：确保程序有足够权限
3. **监控文件句柄**：使用系统工具监控资源使用
4. **测试轮转逻辑**：使用小的 `MaxSize` 值快速测试

## 🤝 贡献指南

我们欢迎社区贡献！参与项目开发请遵循以下流程：

### 🚀 开发环境设置

```bash
# 1. Fork 并克隆仓库
git clone https://gitee.com/MM-Q/logrotatex.git
cd logrotatex

# 2. 创建开发分支
git checkout -b feature/your-feature-name

# 3. 安装开发依赖
go mod tidy

# 4. 运行测试确保环境正常
go test -v ./...
```

### 📋 提交要求

| 要求 | 说明 |
|------|------|
| ✅ **代码质量** | 通过 `gofmt`、`golint` 检查 |
| ✅ **测试覆盖** | 新功能必须包含测试用例 |
| ✅ **文档更新** | 更新相关文档和示例 |
| ✅ **提交信息** | 使用清晰的提交信息格式 |

### 🔄 提交流程

```bash
# 1. 提交代码
git add .
git commit -m "feat: 添加新功能描述"

# 2. 推送到远程分支
git push origin feature/your-feature-name

# 3. 创建 Pull Request
# 在 Gitee 上创建 PR，详细描述变更内容
```

### 📊 代码规范

- 遵循 Go 官方代码规范
- 使用有意义的变量和函数名
- 添加必要的注释和文档
- 保持代码简洁和可读性

## 📄 许可证

本项目采用 **MIT 许可证** - 详见 [LICENSE](LICENSE) 文件

## 🙏 致谢

本项目基于 [natefinch/lumberjack](https://github.com/natefinch/lumberjack) 库的 v2 分支进行开发和扩展。我们对原作者 **Nate Finch** 及其团队的杰出工作表示诚挚的感谢！

### 🌟 主要改进

- 🔧 **构造函数支持** - 添加了 `NewLogRotateX()` 构造函数
- 🛡️ **增强安全特性** - 内置路径安全验证机制
- 📊 **性能优化** - 优化文件扫描算法和内存使用
- 🗜️ **ZIP 压缩** - 改进压缩格式，提供更好兼容性
- 🔒 **权限控制** - 增加文件权限配置选项
- 🚀 **缓冲写入器** - 新增高性能批量写入功能，三重触发条件智能刷新
- 🌐 **本地化支持** - 提供中文文档和本地化体验

### 📚 原始项目信息

| 项目信息 | 详情 |
|----------|------|
| **原项目地址** | https://github.com/natefinch/lumberjack |
| **原作者** | Nate Finch |
| **基于分支** | v2 |
| **原项目许可** | MIT License |

## 🔗 相关链接

<div align="center">

### 📚 文档与资源

[![API文档](https://img.shields.io/badge/📖_API文档-blue?style=for-the-badge)](APIDOC.md)
[![设计文档](https://img.shields.io/badge/🏗️_设计文档-green?style=for-the-badge)](docs/design.md)
[![性能分析](https://img.shields.io/badge/📊_性能分析-orange?style=for-the-badge)](docs/performance.md)

### 🌐 项目链接

[![Gitee仓库](https://img.shields.io/badge/🏠_Gitee仓库-red?style=for-the-badge)](https://gitee.com/MM-Q/logrotatex)
[![问题反馈](https://img.shields.io/badge/🐛_问题反馈-yellow?style=for-the-badge)](https://gitee.com/MM-Q/logrotatex/issues)
[![功能建议](https://img.shields.io/badge/💡_功能建议-purple?style=for-the-badge)](https://gitee.com/MM-Q/logrotatex/issues/new)

### 🤝 社区支持

[![讨论区](https://img.shields.io/badge/💬_讨论区-lightblue?style=for-the-badge)](https://gitee.com/MM-Q/logrotatex/discussions)
[![贡献指南](https://img.shields.io/badge/🤝_贡献指南-brightgreen?style=for-the-badge)](#-贡献指南)
[![行为准则](https://img.shields.io/badge/📜_行为准则-lightgrey?style=for-the-badge)](CODE_OF_CONDUCT.md)

</div>

---

<div align="center">

**🔄 LogRotateX** - 让日志管理变得简单高效！ 🚀

*如果这个项目对您有帮助，请给我们一个 ⭐ Star！*

[![Star History Chart](https://api.star-history.com/svg?repos=MM-Q/logrotatex&type=Date)](https://gitee.com/MM-Q/logrotatex)

</div>