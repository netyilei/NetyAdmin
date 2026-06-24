# 基座升级指南：上传接口安全闭环改造（必读）

> **适用对象**：基于本基座程序做二次开发的团队（通常由 AI 写代码）
> **基线提交**：`9bc85ee`（fix(security): 上传接口凭证签发-通知状态机闭环）
> **升级性质**：**破坏性变更（Breaking Change）**，涉及数据库、后端、前端三端联动
> **无需重新 clone**：本文档给出所有改动点和适配方向，交给你们的 AI 自行合并修改即可

---

## 一、为什么要改（务必让 AI 读完这段）

**原设计存在严重安全漏洞**：`upload_record` 表没有状态字段，"获取上传凭证"和"上传成功通知"是两个**完全解耦**的接口：

- 获取凭证接口：只返回 presigned URL，**不落任何数据库记录**，响应里**没有记录 ID 也没有加密字符串**。
- 上传记录接口：**直接 INSERT 一条"已上传"记录**，不验签、不校验文件是否真的上传、不校验 objectKey 与凭证是否绑定。

**后果**：任意已登录用户可以**完全不执行真实上传**，直接调上传记录接口，伪造一个 `objectKey` + `fileName` + 任意 `md5`，就能在 `upload_record` 表凭空制造一条"已上传"记录。

**改造方案**：把两个解耦接口改成 **pending → uploaded 状态机闭环**：

```
获取凭证接口
   └─ 落一条 pending 记录（status=pending, secret=HMAC签名, expires_at=30分钟后）
   └─ 返回 { recordId, secret, ...上传凭证 }

客户端真实 PUT 上传到对象存储

上传成功通知接口
   └─ 必须带 recordId + secret
   └─ 验签 + 行锁事务内 status 翻转 pending → uploaded
```

签名公式：`HMAC-SHA256(jwt.secret, recordID|objectKey|source|sourceID|expiresAtUnix)`

- 防伪造 / 防 ID 枚举：光传 recordId 无法通过验签
- 防篡改：objectKey 写进签名，改 key 会破坏签名
- 防超期重放：expiresAt 写进签名 + 状态机超期校验
- 防并发重复提交：事务内 `SELECT ... FOR UPDATE` + `WHERE status='pending'`
- 防时序攻击：验签用 `hmac.Equal` 恒定时间比较

> **重要提醒给 AI**：本改造是**破坏性变更**。新流程下，上传成功通知接口**不再接受**旧的裸 objectKey + md5 调用方式，**必须**带 `recordId` + `secret`。所以用户项目里所有调用上传相关接口的代码都要配套改。

---

## 二、数据库改动（必须执行）

新建迁移文件 `server/migrations/table/073_upload_record_status.sql`：

```sql
BEGIN;

-- status:    pending(待上传) / uploaded(已上传) / expired(已过期)
-- secret:    凭证签发时生成的 HMAC 签名，与 recordID 组合防止只传 ID 的伪造攻击
-- expires_at: 凭证过期时间，超期未通知的 pending 记录由定时任务标记为 expired
--
-- 注意：status 默认值设为 'uploaded'，使历史数据（旧逻辑直接 INSERT 的已完成记录）
-- 自动为已上传状态，无需回填。

ALTER TABLE upload_record ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'uploaded';
ALTER TABLE upload_record ADD COLUMN IF NOT EXISTS secret VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE upload_record ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_upload_record_status ON upload_record(status);

COMMIT;
```

**说明**：
- `status` 默认值 `'uploaded'` 是**刻意设计**——历史数据自动为已上传状态，无需数据回填。
- `uploaded_at` 列原有的 `autoCreateTime` 语义被废弃（见下方 entity 改动），但 DB 列定义不变。
- 迁移框架每次启动都会执行该文件，`IF NOT EXISTS` 保证幂等。

---

## 三、后端改动清单

### 3.1 实体 `server/internal/domain/entity/storage/record.go`

**改动**：
1. 新增 `RecordStatus` 类型 + 三个常量（`pending`/`uploaded`/`expired`）。
2. `Record` 结构体新增 3 个字段：`Status`、`Secret`、`ExpiresAt`。
3. `UploadedAt` 字段类型从 `time.Time` **改为 `*time.Time`**（指针），并**去掉 `autoCreateTime` tag**。

**为什么**：pending 记录一插入时 `uploaded_at` 必须是 NULL（表示尚未上传完成），只有通知成功后才写时间。原来是 `time.Time` + autoCreateTime，一插入就被赋值为 now，无法表达"未上传"。

```go
type RecordStatus string

const (
    RecordStatusPending  RecordStatus = "pending"
    RecordStatusUploaded RecordStatus = "uploaded"
    RecordStatusExpired  RecordStatus = "expired"
)

// 在 Record 结构体内：
Status     RecordStatus `gorm:"column:status;type:varchar(20);not null;index;comment:上传状态" json:"status"`
Secret     string       `gorm:"column:secret;type:varchar(128);not null;default:'';comment:凭证HMAC签名" json:"-"`
ExpiresAt  *time.Time   `gorm:"column:expires_at;comment:凭证过期时间" json:"expiresAt"`
UploadedAt *time.Time   `gorm:"column:uploaded_at;comment:上传完成时间" json:"uploadedAt"`
```

> **连带影响**：项目里任何读取/写入 `record.UploadedAt` 的地方，都要适配指针类型（用 `*record.UploadedAt` 或判空）。原 `UploadedAt time.Time` 的零值判断 `record.UploadedAt.IsZero()` 要改成 `record.UploadedAt == nil`。

### 3.2 VO `server/internal/domain/vo/storage/record.go`

`RecordVO.UploadedAt` 同样改为 `*time.Time`，并新增 `Status`、`ExpiresAt` 字段。

### 3.3 新增签名工具 `server/internal/pkg/utils/sign.go`（新建文件）

提供 `SignUploadRecord` / `VerifyUploadRecord`，复用 `[jwt] secret` 作为 HMAC 密钥。验签用 `hmac.Equal` 恒定时间比较。

```go
// 签名内容：recordID|objectKey|source|sourceID|expiresAtUnix
func SignUploadRecord(key string, recordID uint, objectKey, source, sourceID string, expiresAtUnix int64) string
func VerifyUploadRecord(key string, recordID uint, objectKey, source, sourceID string, expiresAtUnix int64, secret string) bool
```

### 3.4 仓储层 `server/internal/repository/storage/record.go`

**新增 3 个方法**：
- `UpdateSecret(ctx, id, secret)` —— 仅更新 secret 字段（**不能用 `Save`**，否则全列覆盖丢数据）。
- `MarkUploaded(ctx, id, fileSize, md5, mimeType, fileURL) (updated bool, err error)` —— **在单个事务内** `SELECT ... FOR UPDATE` 行锁读取 + 仅当 `status='pending'` 时翻转，返回是否真正翻转（防并发）。
- `CleanupExpiredPending(ctx, before) (int64, error)` —— 定时任务用，把超期 pending 标记为 expired。

> **AI 注意**：`MarkUploaded` 的 Raw 查询必须 `SELECT id, status`（含 id），不能只 `SELECT status`，否则 `record.ID == 0` 会误判记录不存在。

### 3.5 服务层 `server/internal/service/storage/record.go`（核心改动）

1. `recordService` 注入 `hmacKey` 字段（值来自 `cfg.JWT.Secret`），`NewRecordService` 签名新增 `hmacKey string` 参数。
2. `GetUploadCredentials` 签名**新增两个参数**：`source UploadSource`、`sourceID string`。内部改为：
   - 生成 presigned URL 后，**落一条 pending 记录**（`status=pending`、`secret=HMAC`、`expires_at=now+30min`、`file_path=objectKey`、`uploaded_at=NULL`）。
   - **关键**：secret 写入要先 Create 拿到 recordID，再算 HMAC，再 UpdateSecret（recordID 在 Create 后才生成）。UpdateSecret 失败要回滚（删除刚建的 pending 记录）。
   - 响应 `Credentials` 结构体**新增 `RecordID` + `Secret` 字段**。
3. **新增** `CompleteUpload(ctx, recordID, secret, objectKey, fileURL, fileSize, mimeType, md5) (*RecordVO, error)`：状态机入口。流程：查记录 → 验 HMAC → 验 objectKey 一致 → 验 status==pending → 验未超期 → `MarkUploaded`（事务内翻转）→ 返回。
4. **删除**旧的 `RecordUpload` 和 `CreateUploadRecord` 方法（裸 INSERT 已废弃，强制新流程）。

### 3.6 DTO（三套）

| 文件 | 改动 |
|---|---|
| `interface/admin/dto/storage/record.go` | `Credentials` 加 `RecordID`/`Secret`；`CreateRecordReq` 改为 `CompleteUploadReq`（字段：`recordId`+`secret`+`objectKey`+`fileUrl`+`fileSize`+`mimeType`+`md5`） |
| `interface/client/dto/v1/storage.go` | `ClientCredentials` 加 `RecordID`/`Secret`；`CreateClientRecordReq` 改为 `CompleteClientUploadReq` |
| `interface/client/dto/v1/user.go` | `CreateUserUploadRecordReq` 字段改为 `recordId`+`secret`+`objectKey`+`fileUrl`+`fileSize`+`mimeType`+`md5` |

### 3.7 Handler（三套）

| Handler | 改动 |
|---|---|
| admin `storage_handler.go` | `GetUploadCredentials` 传 `source=UploadSourceAdmin` + `sourceID=adminID`；`CreateUploadRecord` 改为调 `CompleteUpload` |
| client `storage_handler.go` | `GetUploadCredentials` 传 `source=UploadSourceClient` + `sourceID=app.ID`，响应映射 `RecordID`/`Secret`；`CreateUploadRecord` 改为调 `CompleteUpload` |
| client `user_handler.go` | `GetUploadToken` 改为走 `RecordService.GetUploadCredentials`（不再调 user service 的旧方法），返回 `UploadTokenVO`（含 recordId/secret）；`RecordUpload` 改为调 `CompleteUpload` |

> **AI 注意**：user 端 `GetUploadToken` 是 GET 接口，`fileName` 可能为空。空时要用时间戳兜底（如 `fmt.Sprintf("upload-%d.bin", time.Now().UnixNano())`），否则 `GenerateObjectKeyWithBusiness` 会生成非法 objectKey。

### 3.8 user service `server/internal/service/user/user.go`

**删除** `GetUploadToken` 方法及其接口声明（重复实现，已统一走 `RecordService`）。`userService.storageMgr` 字段若不再使用可保留（不影响编译）。

### 3.9 user VO `server/internal/domain/vo/user/user.go`

`UploadTokenVO` 新增 `ObjectKey`/`FinalURL`/`RecordID`/`Secret` 字段。

### 3.10 定时任务 `server/internal/job/upload_record_cleanup.go`（新建文件）

每 30 分钟扫描超期 pending 记录标记为 expired。实现 `task.Task` 接口，cron 表达式 `0 */30 * * * *`（6 字段带秒，本项目 cron 用 `WithSeconds()`）。

### 3.11 job 注册 `server/internal/job/init.go`

`AllJobs` 签名新增 `uploadRecordRepo storageRepo.RecordRepository` 参数，return 列表加 `NewUploadRecordCleanupJob(uploadRecordRepo)`。

### 3.12 错误码 `server/internal/pkg/errorx/errorx.go`

新增 1015xx 段（存储上传）：
```go
CodeUploadRecordNotFound   Code = 101501  // 上传记录不存在
CodeUploadSignatureInvalid Code = 101502  // 上传凭证校验失败
CodeUploadRecordCompleted  Code = 101503  // 该上传记录已完成，不可重复提交
CodeUploadRecordExpired    Code = 101504  // 上传凭证已过期
CodeUploadRecordMismatch   Code = 101505  // 上传记录与请求不匹配
```
记得在 `codeMessages` map 里加对应中文消息。

### 3.13 依赖注入 `server/internal/app/wire.go`

两处改动：
1. `NewRecordService(repos.uploadRecord, s.storageConfig, storageMgr, s.app, cfg.JWT.Secret)` —— 新增最后一个参数 `cfg.JWT.Secret`。
2. `job.AllJobs(...)` 调用新增 `repos.uploadRecord` 参数。

---

## 四、前端改动清单（admin-web）

### 4.1 类型 `src/typings/api/v1/storage.d.ts`

1. `UploadCredentials` 加 `recordId: number` + `secret: string`。
2. 新增 `UploadRecordStatus` 类型 + `CompleteUploadParams` 类型。
3. `UploadRecord` 加 `status: UploadRecordStatus`，`uploadedAt` 改为可选 `uploadedAt?: string`（pending 记录该字段为 null）。

### 4.2 API 层 `src/service/api/v1/storage.ts`

`fetchCreateUploadRecord` 改名为 `fetchCompleteUpload`，参数类型改 `CompleteUploadParams`，URL/方法不变（`POST /admin/v1/storage/upload-record`）。

### 4.3 上传工具 `src/utils/upload.ts`

`UploadCredentials` 接口加 `recordId` + `secret`（用于类型传递）。

### 4.4 所有调用上传流程的组件（重点！让 AI 全局搜索）

**全局搜索** `fetchCreateUploadRecord`，把每处上传流程改为新流程。典型模式：

```ts
// ❌ 旧流程
const { data: credentials } = await fetchGetUploadCredentials({ fileName, ... });
await uploadWithPresignedUrl(credentials, file);
await fetchCreateUploadRecord({
  configId: credentials.configId,
  fileName, objectKey: credentials.objectKey, fileSize, mimeType, ...
});

// ✅ 新流程
const { data: credentials } = await fetchGetUploadCredentials({ fileName, ... });
await uploadWithPresignedUrl(credentials, file);
await fetchCompleteUpload({
  recordId: credentials.recordId,   // ← 新增：必须回传
  secret: credentials.secret,        // ← 新增：必须回传
  objectKey: credentials.objectKey,
  fileUrl: credentials.finalUrl,
  fileSize, mimeType
});
```

基座自带的 5 个调用点（用户的业务代码可能有更多，要全改）：
- `components/custom/toast-ui-editor.vue`
- `views/content/article/components/article-operate-modal.vue`
- `views/content/banner/components/banner-operate-modal.vue`
- `views/manage/user/components/user-operate-modal.vue`
- `views/settings/storage-config/components/storage-test-upload-modal.vue`

### 4.5 上传记录列表 `src/views/ops/upload-record/index.vue`

新增"状态"列（pending/uploaded/expired 用不同颜色 Tag）。

### 4.6 i18n（`zh-cn`/`en-us` 的 `page/manage.ts`）

`upload` 下新增：`status`/`statusPending`/`statusUploaded`/`statusExpired` 文案。

---

## 五、给用户 AI 的适配指令（可直接复制给 AI）

```
我们的基座程序刚修复了一个上传接口安全漏洞（凭证签发-通知状态机闭环）。
请基于 docs/upgrade-upload-security-closure.md 帮我做以下适配：

1. 检查我们项目里所有调用「获取上传凭证」+「上传成功通知」流程的代码
   （前端搜 fetchCreateUploadRecord / fetchGetUploadCredentials，
   后端搜 RecordUpload / CreateUploadRecord / GetUploadToken / GetUploadCredentials）。

2. 对每处上传流程，按文档"新流程"模式改造：
   - 获取凭证后，凭证响应里会多出 recordId + secret，要保存下来。
   - 真实 PUT 上传到对象存储后，调上传成功通知接口时，
     必须把 recordId + secret 回传，不能再只传 objectKey + md5。
   - 业务类型(businessType)等字段已在凭证签发时登记，通知接口无需再传。

3. 后端如果我们有自定义的上传相关 handler/service/repository，检查是否：
   - 直接调用了已删除的 RecordUpload / CreateUploadRecord 方法 → 改用 CompleteUpload。
   - 读取了 record.UploadedAt（time.Time）→ 适配为指针 *time.Time。
   - 自己 new 了 Record 实体 → 补上 Status 字段（新建走 pending）。

4. 数据库：确保 migrations/table/073_upload_record_status.sql 存在并执行。

5. 不要重新 clone 基座，只做增量适配。改完后跑 go build / go vet / 前端 tsc 验证。

参考文档：docs/upgrade-upload-security-closure.md
```

---

## 六、验收检查清单

升级后请逐项确认：

- [ ] 数据库 `upload_record` 表已有 `status` / `secret` / `expires_at` 三列
- [ ] 历史记录 `status` 全部为 `uploaded`（默认值生效，无 NULL）
- [ ] 调获取凭证接口，响应里含 `recordId` 和 `secret`
- [ ] 不带 `recordId`+`secret` 调上传成功通知接口，返回 101502「上传凭证校验失败」
- [ ] 重复调上传成功通知接口，返回 101503「该上传记录已完成，不可重复提交」
- [ ] 等凭证过期（或手动改 expires_at）后调通知，返回 101504「上传凭证已过期」
- [ ] 定时任务能清理超期 pending 记录（看日志 `[UploadRecordCleanup]`）
- [ ] `go build ./...` / `go vet ./...` 通过
- [ ] 前端 `vue-tsc --noEmit` 通过
- [ ] 上传记录列表能显示状态列

---

## 七、常见问题

**Q: 升级后旧的"已上传"历史记录会不会出问题？**
A: 不会。`status` 列默认值 `'uploaded'`，历史数据自动标记为已上传，无需回填。

**Q: 我们自己写了上传相关的业务代码，怎么知道要不要改？**
A: 全局搜上述关键词。只要你的代码调用了 `fetchCreateUploadRecord`（前端）或 `RecordUpload`/`CreateUploadRecord`（后端），就**必须**改。如果你的代码只是**读取**上传记录列表，不受影响。

**Q: 我们扩展了 Record 实体或 upload_record 表，会冲突吗？**
A: 不会。本次只新增列（status/secret/expires_at）和改 UploadedAt 类型。你新增的列/字段保留即可。但要注意 UploadedAt 从 `time.Time` 变 `*time.Time`，引用它的代码要适配指针。

**Q: 为什么签名复用 jwt.secret？**
A: jwt.secret 已有强度校验（≥16字节+2类字符）、已加载、无需新增配置。它是基座内置密钥，复用最省事。注意：**生产环境必须替换默认密钥**，否则签名形同虚设（但这是部署配置问题，基座开源故不强制）。

**Q: 凭证有效期为什么是 30 分钟？**
A: 常量 `credentialTTL = 30 * time.Minute`（在 `service/storage/record.go`）。可根据业务调整，但应 ≥ presigned URL 的有效期（默认 15 分钟）。

---

*本文档对应基座提交 `9bc85ee`。如有疑问，对照该提交的 diff 即可。*
