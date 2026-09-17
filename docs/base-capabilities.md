# 基座能力清单（下游开发必读）

> **给 AI/新成员的指令**：在实现任何"通用"功能（工具函数、格式化、鉴权、缓存、上传、脱敏……）之前，
> **先查本清单**。本清单列出的能力均已在基座中单一实现并被测试覆盖——直接引用，禁止重写。
> 各模块的深入设计见 `docs/server-module-*.md`，本清单是"有什么能用"的索引。

## 一、后端通用工具（pkg 层，单一事实源）

| 能力 | 唯一入口 | 说明 |
|------|---------|------|
| LIKE 查询转义 | `pkg/like` 的 `EscapeLike` / `LikeContains` | 转义 `% _ \`，所有 repo 搜索已接入；新搜索点必须用它 |
| PG 唯一冲突判定 | `pkg/database.IsUniqueViolation(err)` | SQLSTATE 23505；并发写竞态的 DB 兜底转业务错误码 |
| 事务管理 | `pkg/database.TxManager`（`Begin/Rollback/Commit/WithTransaction`） | service 层事务统一走 TM，禁止自管事务 |
| HMAC-SHA256 | `pkg/utils.HMACSHA256Hex` / `HMACSHA256Base64` | 两种编码各归其位，勿再内联 hmac 实现 |
| 上传记录验签 | `pkg/utils.SignUploadRecord` | recordID+objectKey+expires 的 HMAC 防伪造 |
| RFC3339 时间解析 | `pkg/utils.ParseRFC3339(s)` | 各 service 时间字段的统一入口（报错文案由调用方给业务语境） |
| 树构建 | `pkg/utils.BuildTree` | menu/category 通用树；配套防环 visited |
| 敏感字段判定/脱敏 | `pkg/mask.IsSensitive`（精确归一化）/ `IsSensitiveSubstr`（子串宽松）/ `ScrubValue`（递归遍历器） | 全项目唯一脱敏实现；open_platform 日志与 Sentry PII 均已接入，关键词并集在此维护 |
| 分页规整 | `pkg/pagination.NormalizePagination` | 0/负/超大 size 统一处理（上限 100） |
| 错误码与默认文案 | `pkg/errorx`（`New(code)` 即用默认文案；`NewWithErr(code, err)` 保留错误链） | 调用点**不要**重复抄默认文案；新码在 `codeMessages` 表登记 |
| 安全恢复 goroutine | `pkg/recovery.GoSafe` | 所有自起 goroutine 必须包裹（panic recover + Sentry） |
| 请求 ID | `pkg/requestid` | 日志/Sentry 关联 |
| endpoint 解析与 URL 构造 | `pkg/storage.ParseEndpoint` / `BuildPublicURL(domain, endpoint, bucket, key, style)` | 含协议剥离、path/virtual-host 寻址风格（`AddressingStyleFor(provider)` 为唯一决策点） |
| MIME 推断 | `pkg/storage.MimeTypeByExt(fileName)` | 上传凭证的 contentType 一律服务端推断 |
| 缓存 | `pkg/cache`：FetchFast（L1+L2）/ SetNX（原子占位）/ **GetAndDelete**（GETDEL 原子消费）/ InvalidateByTags（跨节点失效） | 一次性凭证消费、防重放、tag 失效均有现成原语 |
| 验证码原子消费 | `service/user` 的 `VerifyAndClearCode` | GETDEL + 5 次错误上限 + 故障 fail-closed，直接用 |
| 登录锁定 | `pkg/auth.HandlePasswordWrong` / `ClearLoginRetry` | 计数、锁定、Redis 故障 fail-closed 语义齐备 |
| refresh 轮换抢占/登出拉黑 | `pkg/auth.ClaimRefreshRotation`（fail-closed）/ `BlacklistRefresh`（best-effort） | admin/user 两端共享，勿再内联 SetNX/Set 黑名单块 |
| 用户唯一性预检 | `userBase.checkUserUnique`（service/user） | Register 与 admin Create 共享；DB 唯一索引兜底 |
| 布尔配置解析 | `pkg/utils.IsTruthy(s)` | "true"/"1" 唯一定义（configsync 内部变体因循环依赖保留） |
| int 配置读取 | `pkg/utils.GetIntWithDefault` / 无 watcher 版 | 勿再手写 Atoi+默认值 |

**注意**：实体字段标 `json:"-"` 的（如 SecretKey/hash）**不能**直接放进 JSON 序列化的缓存层——用显式 entry 结构（参考 `repository/user/user_token_cached.go`、`service/storage/config.go` 的先例）。

## 二、后端模块级能力（避免重建整个模块）

认证（admin/client 双体系、RS256、TokenVersion+端级顶号、refresh SetNX 轮换防重放）、RBAC、开放平台（AppKey HMAC 签名中间件、scope/API 双层授权、nonce 防重放、调用统计）、IPAC（CIDR trie、全局/应用级、过期自动解载、CRUD 热更新+PubSub 同步）、存储（S3 兼容预签名直传三步流：凭证→PUT→CompleteUpload 验签）、缓存（A/B 双模式）、任务（interval/cron/队列、分布式锁、防重入守卫）、PubSub 事件总线、LogBus 操作/错误日志缓冲、消息（模板+多渠道+站内信）、字典、内容（文章/分类/Banner/定时发布）、迁移（0001-0499 基座 / 1001+ 下游号段）。
详见 AGENTS.md 文档索引对应模块文档。

## 三、前端通用能力（admin-web）

| 能力 | 唯一入口 | 说明 |
|------|---------|------|
| 日期时间/日期格式化 | `utils/format.formatDateTime / formatDate` | 空值兜底 `-`，已覆盖全站列表 |
| 延迟格式化 | `utils/format.formatNsToMs / formatMs` | 纳秒/毫秒两单位明确命名 |
| 分类树扁平化 | `utils/category.buildCategoryOptions(tree, withExtra?)` | 下拉选项（全角空格缩进） |
| 超管判定 | `utils/common.isSuperByCode(code \| codes[])` | 唯一实现，勿再内联 env 比较 |
| 字典 label i18n | `hooks/common/dict.translateDictLabel` + `useDict()`（getDictLabel/renderDictTag/renderBoolDictTag） | 含 i18n 启发式的唯一实现 |
| 上传三步流 | `utils/upload.uploadFileWithCredentials` | 凭证→直传→CompleteUpload 已封装，勿手写（富文本编辑器内嵌上传同用此封装） |
| 表格 CRUD 样板 | `hooks/common/table.useTable` + `hooks/common/operation.useOperation` | 分页/弹窗操作统一 |
| 深拷贝 | `@na/utils` 的 `jsonClone` | 勿写 `JSON.parse(JSON.stringify(...))` |
| 状态标签渲染 | `hooks/common/dict.renderTagFromMap(map, value)` | 各页只维护映射表，渲染结构统一（勿再内联 NTag 三元/查表） |
| 字节数格式化 | `utils/format.formatBytes` | B/KB/MB/GB |
| 表单校验正则 | `constants/reg` + `hooks/common/form` | 手机号/邮箱等已集中 |
| 请求层 | `utils/service`（统一响应/错误/token 注入） | 禁止裸 fetch/axios |

## 四、明确不要重复造的轮子（高频误建）

1. 任何"格式化时间/延迟/大小"的局部函数 → 先查 `utils/format`（前端）、`pkg/utils`（后端）
2. 任何"HMAC/SHA256 哈希"内联 → `pkg/utils` 双版本已备
3. 任何"敏感字段脱敏"遍历器 → `pkg/mask.ScrubValue`
4. 任何"验证码校验+删除"逻辑 → `VerifyAndClearCode`（自带原子性与次数上限）
5. 任何"endpoint 剥协议/拼存储 URL" → `pkg/storage` 已闭环（含寻址风格决策）
6. 任何"防重入/一次性消费" → `SetNX` / `GetAndDelete` 缓存原语
7. 错误返回时抄写 errorx 默认文案 → `errorx.New(code)` 即可
8. refresh 黑名单/轮换抢占逻辑 → `pkg/auth.ClaimRefreshRotation` / `BlacklistRefresh`
9. 布尔配置判断 `"true"||"1"` → `utils.IsTruthy`
10. 状态列 NTag 渲染 → `renderTagFromMap`（前端）/勿自建渲染结构
