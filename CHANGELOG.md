# Changelog

> 本文件记录对外可见的行为变更与接口变更（面向下游项目与运维）。
> 格式：倒序日期条目；每条注明影响面与迁移动作。内部重构（无行为变化）不单列。

---

## 2026-11-10 · 2.0.1（下游 Module 装配补全：8 个 App 服务 getter）

### ➕ Added

- `App` 新增 8 个已构造服务实例 getter，供下游 Module（实现 `ClientRouterModule` / `AdminRouterModule` 等）组装自有路由处理器直接取用，不再需要重建服务或穿透基座内部结构。
- **类型重命名（源码级，无行为变更）**：`storage.RecordService` → `storage.UploadRecordService`（含构造器 `NewUploadRecordService`）——getter 名与类型名对齐为同一领域名；该接口能力是上传记录全生命周期（凭证签发/完成验签/记录管理），原名 "RecordService" 欠精确。B4 尚未被下游消费，此为最后零成本窗口；下游声明类型以 §5.6 getter↔类型映射表为准。
- getter 明细：`CaptchaStore()`（验证码存储：cache/DB 双轨，由 sys_configs 验证码场景开关动态选择；cache 模式内部为 L1/L2）、`LoginLimiter()`（登录限流器，与基座登录共享窗口计数）、`UploadRecordService()`、`UserClientService()`（C 端用户服务：资料/联系方式/密码）、`ContentCategoryClientService()` / `ContentArticleClientService()` / `ContentBannerGroupClientService()` / `ContentBannerItemClientService()`（C 端内容四服务）。
- 文档：[server-architecture §5.6](docs/server-architecture.md) 增加「App getter 依赖面」小节（安全组件 / 基础设施 / 已构造服务三类全列）；[基座能力清单](docs/base-capabilities.md) 同步登记。

> 纯增量 API，无行为变更；不升级的下游不受影响。

---

## 2026-11-07 · 2.0.0（手机号/邮箱 PII 静态加密 + Redis 8.1 基线）

> **升级前必读 [docs/upgrade-guide.md](docs/upgrade-guide.md)**（含预检脚本 `scripts/tools/upgrade_preflight.sh`）。
> `/health` 的 `component.version` 自本版本起为 `2.0.0`（此前为 `v1`）。

### ⚠️ Breaking / 行为变更

| 变更 | 影响面 | 迁移动作 |
|------|--------|---------|
| **users / admin_user 手机号静态加密** | 直接读写 `users.phone` / `admin_user.phone` 列的下游 SQL/报表 | 明文列退役（惰性迁移后恒空）；密文在 `phone_enc`（AES-256-GCM）、检索走 `phone_idx`（HMAC 盲索引）、尾号检索走 `phone_tail4`。**中缀模糊搜索（LIKE '%…%'）永久下线**，检索语义 = 完整号精确 / 尾号 4 位精确 |
| **users / admin_user 邮箱静态加密（同构）** | 直接读写 `email` 列的下游 SQL/报表 | 明文列退役；密文在 `email_enc`（保留原始大小写）、检索走 `email_idx`（小写归一化盲索引，大小写多形态同索引）。**无尾号等价物**，检索语义 = 完整地址精确；中缀模糊搜索永久下线 |
| **接口出口手机号/邮箱掩码策略** | 前端展示层 | 列表 / autocomplete 的 phone/email 字段为掩码；**本人资料（客户端 profile）与管理端详情返回完整值**（用户需确认自己绑定的号码/邮箱；admin 详情按权限查看）。编辑表单回显完整值原样提交 = 未变更（不会覆盖真实值） |
| **邮箱格式校验收紧** | 注册 / 换邮箱 / admin 建/编 | 非法邮箱返回 CodeInvalidParams；`utils.IsEmail` 正则迁入 `pii.IsValidEmail`（单一事实源，调用方已同步） |
| **新增详情接口（PII 完整查看）** | 前端/下游 | `GET /admin/v1/systemManage/users/:id`（新增，0061 权限种子）；`GET /admins/:id` 改为返回完整手机号/邮箱。列表/autocomplete 保持掩码；管理端用户/管理员编辑弹窗已改为详情接口回填 |
| **空手机号/邮箱提交 = 保持原值**（原为清空） | admin 编辑用户/管理员、资料更新 | 写路径 `seal` 对空明文采取"未变更"语义：一方面防解密失败（密钥错配）时静默清空密文，另一方面避免编辑表单留空误清号。显式清空需走专用方法（当前无 UI 入口） |
| **启动期同步加密迁移 / 不支持混合版本** | 部署时序 | 存量明文在服务开始监听前完成加密（阻塞启动）。**本版本不支持新旧版本混合运行**：必须停止全部节点后整批切换 |
| **不提供降级迁移（前向演进）** | 回滚策略 | PII 加密是不可逆数据变换，0059/0060 无 down 迁移；版本回退 = 恢复升级前备份 + 旧版本程序（基座不做旧版本兼容） |
| **写入口手机号格式校验**（^1[3-9]\d{9}，归一化后） | 注册 / 换手机号 / admin 建/编用户与管理员 | 不合法号码返回 CodeInvalidParams |

### ➕ Added

- `internal/pkg/pii`：手机号/邮箱 PII 单一事实源（HKDF 从 `[security].aes_key` 派生四子密钥、AES-GCM 加密、HMAC 盲索引、归一化、掩码；邮箱格式校验正则自 utils 迁入）。
- `userBase.GetUserContact`：通用 PII 读取出口——按条件（ID/用户名/手机号/邮箱，优先级取第一个非空）查不同用户并返回解密后的完整联系方式，各端均可调用（权限由各端 API 决定）。
- `UserRepository.UpdatePhone` / `UpdateEmail`、`AdminRepository.UpdatePhone` / `UpdateEmail`：换号/换邮箱专用（加密在 repo 层，service 不接触密钥与列名；显式清空可传空串）。
- 启动惰性迁移：存量明文手机号后台加密写回（幂等、多节点安全），完成后明文列置 NULL。

### 🐛 Fixed

- **手机号/邮箱明文泄露面清零**：admin 用户列表与 autocomplete 此前直接序列化实体（明文 phone/email 全量出接口），现统一 VO + 掩码；操作日志/开放平台日志的 phone/email 字段纳入脱敏（mask 包）。
- admin 编辑回显掩码值被当作改号提交的回写污染（防御性跳过）。
- 自审修复：`UpdateProfile` 双改（手机+邮箱）三条写改为同事务（拆分后原为部分成功）；autocomplete 移除对已退役明文列的 `email LIKE` 死条件；历史脏值邮箱（无 @）掩码不再原样出接口；email 明文日志脱敏。

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
