# 测试规范（V2 核心版）

> 关联：`ai-coding-discipline.md` §5、全局 G1-12（契约逐条断言/防假绿）。

---

## 1. 分层与门禁

| 层 | 工具 | 门禁 |
|----|------|------|
| 单元 | testify + 内存适配器（不启动 DB/Redis） | kernel ≥ 80%；platform ≥ 70%（初始值） |
| 集成 | 本机 PG/Redis（不用 docker） | 每张表 ≥ 1 条 CRUD 冒烟；迁移全量跑通 |
| 集成接入 | `TEST_DATABASE_DSN` 指向**测试专用库**（开发库永不作为测试目标——Cleanup 清表会误伤共享数据）；未设置时集成测试显式 Skip，不做假绿 | 集成用例必须真库验证事务/迁移链路 |
| 多包集成串行 | 全量跑集成必须 `go test -p 1 ./...`（包串行）——goose 全局版本水位共享，并行包互删水位致乱序拒绝；各包 Cleanup 精确删本模块迁移号段，禁宽范围 | 全量绿必须以 `-p 1` 跑出；CI 同步 |
| 契约 | Port local/remote 同一套用例 | 双适配器结果一致 |
| 架构 | import 图/字面量/Manifest 脚本 | 零违规 |
| E2E | Quickstart 冒烟（登录→CRUD→审计落库） | 每次发布 |
| 并发 | `go test -race` | **必开** |
| 泄漏 | goleak | 每包 TestMain 接入 |

---

## 2. 测试纪律（防 AI 假绿）

- **断言行为/数据，不断言 mock 调用次数**；mock 只用于隔离边界。
- 每 Port 方法必须覆盖：正常路径 + **每种错误码** + 边界（nil/空/超长）+ 并发（race）。
- 禁 `t.Skip` 无 issue 关联；禁测试内 `time.Sleep` > 50ms（用 channel/sync 等待）。
- 禁测试内硬编码密码/密钥/生产地址；fixture 统一 `testkit/`。
- 错误路径优先于正常路径（G1-03③）：先写错误分支断言。
- 表驱动 + `t.Run` 子测试；错误断言必须校验 `code` 与 `message_key`。

---

## 3. 命名与结构

- `xxx_test.go` 与被测同包（或 `_test` 外部包）；表驱动；子测试命名 `场景-输入-期望`。
- 集成测试统一 `testkit/`：连本机 PG/Redis、跑迁移、注入 Clock、清理。
- 契约测试：local/remote 双适配器跑同一 `testkit` 用例集。

---

## 4. CI

- `go test -race ./...`（PR 必跑）。
- 覆盖率门禁脚本（kernel/platform 分阈值）。
- profile 矩阵（lean/standard/full）构建 + 冒烟。
- 迁移 dry-run + down 验证。
- E2E 冒烟（本机 PG/Redis + app 进程；不用 docker）。

---

## 5. 反模式

- ❌ 只测 happy path。
- ❌ mock 一切导致测试通过但集成坏。
- ❌ 断言 `assert.NoError` 就算测过错误路径。
- ❌ 测试里重复实现业务逻辑。
- ❌ 共享可变全局 fixture 互相污染。
