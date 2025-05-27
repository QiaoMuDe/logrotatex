# LogRotateX - Go 日志轮转工具

## 功能特性

- 📁 自动日志轮转功能
- 🔄 基于文件大小的轮转触发
- 🗑️ 自动清理旧日志文件
- 🗜️ 支持 gzip 压缩轮转后的日志
- ⏱️ 支持本地时间或 UTC 时间命名
- 🛡️ 线程安全设计

## 安装

```bash
go get gitee.com/MM-Q/logrotatex
```

## 快速开始

### 基本用法

```go
package main

import "gitee.com/MM-Q/logrotatex"

func main() {
    // 初始化日志记录器
    logger := &logrotatex.Logger{
        Filename:   "/var/log/myapp.log",
        MaxSize:    100, // MB
        MaxBackups: 5,
        MaxAge:     30, // days
        Compress:   true,
    }

    // 写入日志
    logger.Write([]byte("This is a log message"))

    // 程序退出前关闭日志
    defer logger.Close()
}
```

### 高级用法

```go
// 使用本地时间命名日志文件
logger := &logrotatex.Logger{
    Filename:   "/var/log/myapp.log",
    LocalTime:  true,
    MaxSize:    200,
    Compress:   true,
}

// 监控日志文件变化
// 这里可以添加监控代码
```

## 配置参数

| 参数名     | 类型   | 说明                       | 默认值           |
| ---------- | ------ | -------------------------- | ---------------- |
| Filename   | string | 日志文件路径               | 空(使用临时目录) |
| MaxSize    | int    | 最大单个日志文件的大小(MB) | 10               |
| MaxBackups | int    | 最大保留日志文件的数量     | 0(保留所有)      |
| MaxAge     | int    | 最大保留日志文件的天数     | 0(不删除)        |
| LocalTime  | bool   | 使用本地时间命名备份文件   | false(UTC)       |
| Compress   | bool   | 是否压缩轮转后的日志       | false            |

## 最佳实践

1. 生产环境建议设置 MaxBackups 和 MaxAge，避免日志文件无限增长
2. 对于高并发场景，建议使用单独的 goroutine 处理日志写入
3. 压缩日志可以节省存储空间，但会增加 CPU 开销

## 贡献指南

欢迎提交 Pull Request 或 Issue 报告问题。提交代码前请确保:

- 通过所有单元测试
- 符合 Go 代码规范
- 添加必要的文档

## 常见问题

Q: 日志文件没有按预期轮转？ 
A: 检查文件权限和磁盘空间

Q: 压缩后的日志文件名是什么格式？
A: 原始文件名加上.gz 后缀

## 许可证

MIT License

Copyright (c) 2025-present MM-Q
