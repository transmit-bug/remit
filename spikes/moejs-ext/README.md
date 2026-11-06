# moejs spike — 进程内 JS 扩展基质

一次性的可行性验证，对应 #67。**不是产品代码，不合并。**

- `main.js` 模拟「内置制卡插件」：`cards(block) → [{front,back,block}]`，外加 `describe` / `on_event` / `spin`（死循环）/ `hungry`（吃内存）/ `dynamic`（eval）。
- `main.go` 测：冷启动、单次调用、死循环打断、内存上限、host panic、动态代码关闭、context 超时、runtime 成本。
- `extension.json` 是设想中的 manifest 形态（`runtime: "js"`）。

## 实测（Apple Silicon / alpha.11）

| 项 | 结果 |
|---|---|
| 编译 | 1.0 ms |
| runtime + load | 1.35 ms（每扩展一次） |
| `cards(block)` 调用 | **1.06–1.2 µs**（126 B 入参，出参 JSON 已生成） |
| 打断 `for(;;)` | **19–25 µs** 从 `Interrupt` 到 `Call` 返回，`*InterruptedError` |
| `MemoryLimit 8MiB` | 触发 `ErrMemoryLimit`（8405894 B） |
| host Go 函数 panic | 返回 `*InternalError`，进程存活 |
| `DisableDynamicCode` | `eval()` 抛错 |
| context 超时 + Interrupt | 返回 `interrupted: context deadline exceeded` |
| 三平台 `CGO_ENABLED=0` | darwin/arm64 7.3M · linux/amd64 7.6M · windows/amd64 7.9M 全部 OK |
| 二进制代价 | 1.1M（hello）→ 7.3M，**+6.2 MB** |
| 依赖代价 | `go.sum` 6 行，1 个 direct require |

未测准：单 runtime 常驻内存（本机 `HeapAlloc` 差值不可信；README 称 81 KiB，按未验证处理）。
