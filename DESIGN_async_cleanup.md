# logrotatex 异步后台清理设计（极简版，单协程）

目标
- 将“压缩+删除”的重任务改为异步后台执行，避免阻塞写入与轮转。
- 同一时间仅允许运行一个异步清理协程，避免并发与任务堆积。
- 代码改动小、易维护；不引入队列/池，保持清理规则与压缩逻辑不变。
- 关闭时等待清理协程完成，避免资源泄露或半压缩文件。

总体方案（无队列、单协程、合并触发）
- 新增布尔开关 `Async` 控制是否异步清理。
- 通过两个原子标记保证单协程运行：
  - `cleanupRunning int32`：0=未运行，1=正在运行。
  - `rerunNeeded int32`：0=不需要重跑，1=当前轮结束后再跑一次。
- 触发清理时：
  - 若 `cleanupRunning` 为 0，置为 1 并启动后台协程；`wg.Add(1)`。
  - 若已在运行，仅将 `rerunNeeded` 置为 1，当前协程结束后再跑一轮（合并多次触发），不再新建协程。
- 后台协程每一轮都“现查现算”最新删除/压缩列表，以避免旧快照失效。

字段与类型变更（logrotatex.go）
```go
type LogRotateX struct {
    // ...已有字段...
    Async         bool           `json:"async" yaml:"async"`
    cleanupRunning int32         // 原子标记：是否正在运行清理协程
    rerunNeeded    int32         // 原子标记：是否需要在本轮后再运行一次
    wg             sync.WaitGroup
}
```

触发逻辑（file_manager.go / internal.go 调用侧）
```go
// cleanupAsync 触发异步清理（不阻塞），保证同一时间仅一个协程。
func (l *LogRotateX) cleanupAsync() {
    // 快速路径：无需清理直接返回
    if l.MaxFiles <= 0 && l.MaxAge <= 0 && !l.Compress {
        return
    }
    // 尝试启动一个协程（CAS 将 0 -> 1）
    if atomic.CompareAndSwapInt32(&l.cleanupRunning, 0, 1) {
        l.wg.Add(1)
        go l.runCleanupLoop()
        return
    }
    // 已在运行，合并触发：设置重跑标记
    atomic.StoreInt32(&l.rerunNeeded, 1)
}
```

后台协程执行（logrotatex.go 或 file_manager.go）
```go
// 单协程清理循环：每轮都现查现算，错误仅日志打印。
func (l *LogRotateX) runCleanupLoop() {
    defer func() {
        atomic.StoreInt32(&l.cleanupRunning, 0)
        l.wg.Done()
    }()

    for {
        // 1) 现查最新文件状态
        files, err := l.oldLogFiles()
        if err != nil {
            fmt.Printf("logrotatex: failed to get old log files: %v\n", err)
            // 若发生错误也继续检查是否需要重跑
        }

        // 2) 计算删除列表
        var remove []logInfo
        if files != nil {
            remove = l.getFilesToRemove(files)
        }

        // 3) 计算压缩列表
        var compress []logInfo
        if l.Compress && files != nil {
            for _, f := range files {
                if !strings.HasSuffix(f.Name(), compressSuffix) {
                    compress = append(compress, f)
                }
            }
        }

        // 4) 执行清理
        if err := l.executeCleanup(remove, compress); err != nil {
            fmt.Printf("logrotatex: async cleanup error: %v\n", err)
        }

        // 5) 检查是否需要再跑一轮
        if atomic.SwapInt32(&l.rerunNeeded, 0) == 1 {
            // 再跑一轮，继续 for
            continue
        }
        // 无需重跑，退出
        break
    }
}
```

在 rotate 中按开关选择同步/异步（internal.go）
原逻辑（同步）：
```go
if err := l.cleanupSync(); err != nil {
    fmt.Printf("logrotatex: cleanup failed during rotation: %v\n", err)
}
```
替换为：
```go
if l.Async {
    l.cleanupAsync() // 异步：不阻塞轮转/写入；单协程保证
} else {
    if err := l.cleanupSync(); err != nil {
        fmt.Printf("logrotatex: cleanup failed during rotation: %v\n", err)
    }
}
```

在 Close 中等待后台任务完成（logrotatex.go）
```go
func (l *LogRotateX) Close() error {
    var closeErr error
    l.closeOnce.Do(func() {
        closeErr = l.close()
        if l.Async {
            // 等待正在运行的清理协程收敛
            l.wg.Wait()
        }
    })
    return closeErr
}
```

关键取舍与并发安全
- 不使用 channel/队列，避免并发与复杂度；单协程由原子标记严格控制。
- 合并触发：高频 rotate 下不会堆积任务，只在当前协程内串行完成后续清理。
- runCleanupLoop 不持有 l.mu，不与写入锁竞争；仅进行目录扫描/压缩/删除 IO。
- executeCleanup 保持原实现不变，减少维护成本。
- 错误仅打印，不影响写流程；可后续按需加入重试策略。

使用说明
- 启用：创建后设置 `l.Async = true` 即可。
- 未启用：保持现有同步行为。
- 适配范围：Windows/Linux/macOS 均适用，未依赖平台特性。

后续可选优化（不在本次实现内）
- 支持关闭期间禁止新的触发（增加关闭标志），确保 Close 后不再受理 cleanupAsync。
- 压缩失败退避/重试策略。
- 在极端高频触发场景下，增加“最短间隔”节流以减少磁盘抖动。