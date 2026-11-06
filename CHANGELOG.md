# Changelog

> 本文件记录对外可见的行为变更与接口变更（面向下游项目与运维）。
> 格式：倒序日期条目；每条注明影响面与迁移动作。内部重构（无行为变化）不单列。

---

## 2026-11-07（origin/master = 8a9a401；基线 762b894..8a9a401，17 提交）

### ⚠️ Breaking / 行为变更

| 变更 | 影响面 | 迁移动作 |
|------|--------|---------|
| **Redis 支持矩阵 6.0 → 8.1+**（本地开发实测 8.10.0） | 部署环境 | 升级 Redis 至 8.1+；`go.mod` 无需改动（go-redis v9.18 客户端完整覆盖 8.x 命令面） |
| **UserTokenRepository 三方法签名变更（乐观 CAS）** | 直接调用该接口的下游代码（编译期可见） | `UpdateHashes` / `UpdateAccessHash` / `ClearHashes` 增加期望态参数（详见 `internal/repository/user/user_token_repo.go` 接口注释）：`UpdateHashes(ctx, userID, platform, expectedTokenVersion, expectedRefreshHash, accessHash, refreshHash, accessExp, refreshExp)`；`ClearHashes(ctx, userID, platform, expectedAccessHash)` 且 0 行=幂等成功。语义：并发会话写入不再互踩，输家干净失败（`gorm.ErrRecordNotFound`），调用方 fail-closed |
| **api_ratelimit 非法规则加载时丢弃** | 配置了 `max<=0` 或 `window<=0` 规则的部署 | 启动日志告警「已忽略该规则」；此类规则不再生效（旧行为：`max:0` = 命中路径整组 429）。规则间计数已按「规则指纹+窗口+阈值」隔离，重复定义相同规则=阈值减半 |
| **GetAndDelete 改用原生 GETDEL 单命令** | 无接口变化；要求 Redis 6.2+（8.1 基线内） | 无需动作。`redis.Nil`（key 不存在）语义不变 |

### 🐛 Fixed

- **Incr 无 TTL 残留根除**：INCR 与 EXPIRE 分两步时 EXPIRE 失败 → 计数 key 永久残留（新验证码窗口直接触发错误上限）。现单条 Lua 原子完成 INCR + 缺失 TTL 补挂（已有 TTL 不续期；历史无 TTL key 下次自增自动修复）。
- **UpdateHashes 0 行静默成功**：纯 UPDATE 无行不报错 → token hash"以为落库了"实际丢失。现 0 行显式返回 `gorm.ErrRecordNotFound`（CAS 语义见上表）。
- **旧基座 token 刷新兼容**：`user_tokens` 行不存在的旧 token 刷新仍正常签发（跳过端级校验且不回写，并发 Login 建行不受影响）。
- **会话 read-revive 窗口根除**：登录事务 commit 前的缓存失效会被并发回源旧行回填，压制新会话 ≤30s。新增 `database.OnCommit` 提交钩子基础设施，失效统一推迟到 commit 成功后执行（§5.2 红线「缓存失效在 Commit 之后」现已基础设施化）。
- **登出误清新会话**：用已被刷新替换的旧 access token 登出，旧行为清空当前会话 hash（踢掉新会话）。现按 access_hash CAS 守卫，旧登出为幂等 no-op。
- **SlidingWindow 同维度多规则计数污染**：不同窗口/阈值的规则读写同一 ZSET，短窗口规则的 PEXPIRE 还会缩短长窗口计数。现计数 key 含规则指纹+窗口+阈值，规则间天然隔离。

### ➕ Added

- `App.ConfigWatcher()`：下游模块热读 sys_configs 的统一入口（对齐基座登录参数的即时生效体验），替代直读 DB。
- `database.OnCommit(ctx, fn)`：事务提交钩子（§5.2 红线基础设施）；无事务 ctx 立即执行，mock TxManager 天然兼容。
- `scripts/debug/e2e_session.sh`：本地栈全链路 E2E（签名→登录→刷新→顶号→登出→Incr TTL→限流隔离→非法规则丢弃→read-revive 并发哨兵），一条命令输出 PASS/FAIL 与持久层证据；脚本与服务实例强制同 Redis 库（`[redis].db` 自动解析）。

### 📚 Docs

- README / DEPLOYMENT / quick-deployment：Redis 版本要求更新为 8.1+。
- `docs/server-module-cache.md` / `docs/base-capabilities.md` / CONTEXT.md：GetAndDelete（原生 GETDEL）、Incr（Lua 保留理由）、限流规则隔离语义同步。
- `docs/server-architecture.md` §5.2：「缓存失效在 Commit 之后」红线行更新为 `database.OnCommit` 用法指引。

---

## 历史版本

2026-11-07 之前的变更见 git log（本项目在此日期前未维护本文件）。
