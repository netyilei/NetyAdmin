# AI 编码纪律（AI 红线与唯一写法）

> 目标：防 AI 在中后期造成契约漂移、假绿测试、隐形债务、模式不一致、进度失真。
> 执行：红线条目由 lint/CI/Hook 强制；"唯一写法"条目由 AI 每次编码对照；违反 = 阻断或退回。
> 关联：全局 G1（证据/根因/五问）、`anti-over-design.md`、`testing.md`。

---

## 1. 增量验证（AI 最易违反，排第一）

- 每完成一个 package：立即 `go build ./... && go vet ./...`。
- **禁止连续生成 > 5 个文件不构建**；不攒到最后一起编译。
- 提交前：`go test -race ./...` 全绿 + 相关包覆盖率达标。
- 前端：每页面/每组件完成即 `pnpm typecheck`；不攒批。
- 脚本：`node --check` / `bash -n` 通过才提交。

---

## 2. 幻觉与猜测（AI 第二易违反）

- 不确定的第三方 API：先查 pkg.go.dev / 写 3 行验证程序确认存在与签名；**禁止凭记忆写不存在的 API**。
- 不确定的业务规则：问用户；**禁止编造默认值/兜底逻辑**。
- 新依赖：必须 ADR；未 ADR 不 import。
- 生成器产物必须带 `// Code generated ... DO NOT EDIT.` 头；CI 禁手改。

---

## 3. 红线清单（CI/门禁强制）

| # | 模式 | 检测 | 处理 |
|---|------|------|------|
| 1 | `panic("not implemented")` / 空 stub 提交 | grep | **fail** |
| 2 | 无编号 `TODO/FIXME/XXX` | grep（必须 `// TODO(<issue>):`） | **fail** |
| 3 | `context.TODO()`（非测试） | lint | **fail** |
| 4 | `time.Sleep`（非测试） | grep/lint | **fail** |
| 5 | `time.Now` 取业务时间（kernel 内延迟/span 计时测量豁免——度量采样非业务时间；业务时间一律 Clock 注入） | lint | **fail** |
| 6 | `os.Getenv`（config 之外） | lint | **fail** |
| 7 | 裸 `go func`（不经 GoSafe/worker） | grep/review | **fail** |
| 8 | `_ = err` / 空 err 分支吞错 | errcheck | **fail** |
| 9 | `interface{}`/`any`（kernel 边界白名单外） | lint | **fail** |
| 10 | `reflect`（kernel 外） | lint | **fail** |
| 11 | 中文/文案进 Go 错误字符串（用 message_key） | AST 扫描 | **fail** |
| 12 | 跨模块 JOIN / 字符串拼 SQL / 非参数化 | 静态+review | **fail** |
| 13 | 手写 5 位状态码 / `na:` key / `user:` 频道 / 裸 topic | 已有扫描 | **fail** |
| 14 | 单文件 > 400 行 / 函数 > 80 行 / 圈复杂度 > 15 | lint 警告 | 连续两次必须拆 |
| 15 | 测试 `t.Skip` 无 issue 关联 | grep | **fail** |
| 16 | 测试 `time.Sleep` > 50ms 等异步 | review | 退回 |
| 17 | 导出 struct 字段缺 json tag | musttag | **fail** |
| 18 | 修改 `// Code generated` 产物 | CI | **fail** |
| 19 | 错误吞掉：`if err != nil { return nil }` | review/lint | **fail** |
| 20 | ctx 未透传进 goroutine/Job | review | 退回 |

---

## 4. 唯一写法清单（同一件事只许一种写法）

| 事项 | 唯一写法 |
|------|---------|
| 错误包装 | 跨层 `fmt.Errorf("<module>: <op>: %w", err)`；同层透传可裸 return |
| 错误文案 | code + message_key；**禁止中文/任意文案进代码字符串** |
| 时间 | `clock.Now()`（kernel 注入）；存储一律 UTC；展示层才转时区 |
| ID | ULID（kernel idgen）；禁 uuid/snowflake 混用 |
| 日志 | slog + 固定字段集（ts/level/module/type/request_id/trace_id/node_id） |
| ctx | 一律第一参数；goroutine/Job 必须透传 |
| goroutine | 一律 `kernel.GoSafe(name, fn)`（panic 隔离 + 命名） |
| 分页 | kernel query helper；默认页大小 ≤ 100；大表 cursor |
| 排序 | 白名单字段；禁止前端直传列名 |
| DTO 映射 | application 层（domain ↔ api DTO）；adapters/domain 不做 |
| 事务 | UnitOfWork；禁 repo 自管；默认超时 5s |
| 缓存 | state/cache Entity + class；禁直连 Redis |
| 事件 | 类型化 Emit；禁字符串 topic |
| 表结构 | 只经 migration；禁运行时改 |
| 配置读取 | config 注入；禁 `os.Getenv` 散落 |
| SQL | 参数化 + 常量表名/列名；禁拼接 |

---

## 5. 结构红线

- `api/` 只 import kernel + 自身；**禁止依赖其他模块 api**（防循环）。
- `domain` 禁框架/`errorx`/`keys`/`reflect`；`application` 禁 `adapters`。
- 导出符号无调用方 → 删除（deadcode）。
- kernel 导出符号变更 → ADR + 版本矩阵更新。
- Port 只在"第二个消费者"或"需要 remote 绑定"时定义；模块内部逻辑不外露。

---

## 6. AI 工作循环（每功能行固定流程）

1. 读任务清单选行（≤5 行/会话；≤1 里程碑）。
2. **先契约**：Manifest/Port/DTO/事件/迁移/权限种子；**再写失败测试**（契约断言）。
3. 实现 → 增量构建 → 测试绿。
4. 自查 §3/§4 清单 + code-audit 要点。
5. 更新任务清单状态 + 进度日志 → 目的式 commit。

- 并行子智能体：按模块/目录划分，**禁止两个智能体写同一文件**。
- 单会话结束必须：任务清单/进度日志已更新 + 全绿 + 无未编号 TODO。

---

## 7. Windows/仓库纪律

- 文件名全小写（`user_auth.go`）；禁止大小写混用（Git 大小写陷阱）。
- 所有文本文件 LF（`.gitattributes` 强制）。
- 路径用 `path/filepath`；禁手拼 `\`。
- 禁 PowerShell 专有语法进跨平台脚本。
