# 存储与上传 API

> 本文档包含客户端文件上传相关的接口。采用**客户端直传**模式：先获取上传凭证，客户端直接上传至对象存储，最后回调记录上传结果。所有接口均需开放平台签名。

---

## 一、上传流程

```
┌──────────────────────────────────────────────────────────────┐
│ 1. POST /client/v1/storage/credentials                         │
│    → 获取上传凭证（预签名 URL、Headers、ObjectKey、recordId、  │
│      secret 等）                                               │
├──────────────────────────────────────────────────────────────┤
│ 2. 客户端使用凭证直接上传文件到对象存储                         │
│    → PUT/POST 至 credentials.url，携带 credentials.headers    │
├──────────────────────────────────────────────────────────────┤
│ 3. POST /client/v1/storage/records                             │
│    → 上传成功后回调，携带 recordId + secret 校验后完成记录      │
└──────────────────────────────────────────────────────────────┘
```

**设计说明**：

- 客户端不经过后端中转文件，直接上传至对象存储（S3/OSS/COS 等），减轻服务器压力
- 上传凭证有时效性（通常 15-30 分钟），过期需重新获取
- 上传记录回调是**必填步骤**，需携带 `recordId` + `secret` 校验上传合法性

---

## 二、接口总览

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | /client/v1/storage/credentials | 签名 | 获取上传凭证 |
| POST | /client/v1/storage/records | 签名 | 记录上传结果 |

---

## 三、开放平台权限配置

存储上传接口需要在开放平台**应用管理**中授权对应的 API 权限才能调用。服务端会校验请求路径是否在应用授权的 API 列表中。

**配置步骤**：

1. 登录管理后台 → 开放平台 → 应用管理
2. 选择目标应用 → 编辑权限范围
3. 勾选存储上传相关 API 并保存
4. 使用该应用的 AppKey/AppSecret 调用存储上传接口

> **提示**：应用绑定了专属存储配置时，`/storage/credentials` 会自动使用应用级存储源；未绑定则使用系统默认存储源。

---

## 四、获取上传凭证

根据文件信息获取上传所需的预签名 URL 和相关凭证。系统会根据应用绑定的存储配置自动选择存储后端。

```
POST /client/v1/storage/credentials
```

**权限**：开放平台签名

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| fileName | string | 是 | 文件名，如 `photo.jpg` |
| contentType | string | 否 | MIME 类型，如 `image/jpeg` |
| fileSize | int64 | 否 | 文件大小（字节） |
| businessType | string | 否 | 业务类型标识，如 `avatar`、`article` |
| businessId | string | 否 | 业务关联 ID |
| sourceInfo | object | 否 | 额外来源信息（键值对） |

**请求示例**：

```json
{
  "fileName": "avatar.png",
  "contentType": "image/png",
  "fileSize": 102400,
  "businessType": "avatar"
}
```

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "url": "https://bucket.oss-cn-hangzhou.aliyuncs.com/uploads/2025/01/xxx.png?OSSAccessKeyId=...&Signature=...",
    "method": "PUT",
    "headers": {
      "Content-Type": "image/png"
    },
    "expiresAt": "2025-01-15T11:00:00Z",
    "objectKey": "uploads/2025/01/01HXYZ1234567890ABCDEFG.png",
    "domain": "https://cdn.example.com",
    "finalUrl": "https://cdn.example.com/uploads/2025/01/01HXYZ1234567890ABCDEFG.png",
    "configId": 1,
    "region": "cn-hangzhou",
    "bucket": "my-bucket",
    "endpoint": "oss-cn-hangzhou.aliyuncs.com",
    "pathPrefix": "uploads",
    "maxFileSize": 10485760,
    "recordId": 42,
    "secret": "a1b2c3d4e5f6..."
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| url | string | 预签名上传 URL，客户端直接向此地址上传文件 |
| method | string | 上传 HTTP 方法：`PUT` / `POST` |
| headers | object | 上传时需携带的额外 HTTP Header |
| expiresAt | string | 凭证过期时间（ISO 8601） |
| objectKey | string | 对象存储 Key，用于后续记录上传结果 |
| domain | string | CDN 域名 |
| finalUrl | string | 文件最终访问 URL（domain + objectKey） |
| configId | uint | 存储配置 ID |
| region | string | 存储区域 |
| bucket | string | 存储桶名称 |
| endpoint | string | 存储端点 |
| pathPrefix | string | 路径前缀 |
| maxFileSize | int64 | 最大文件大小限制（字节） |
| recordId | uint | 上传记录 ID，**回调时必须携带** |
| secret | string | 上传校验密钥，**回调时必须携带** |

> **重要**：`recordId` 和 `secret` 是上传成功后回调 `/storage/records` 接口的必填参数，用于服务端校验上传合法性。请妥善保存。

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败（fileName 必填） |
| `100002` | 未授权（应用信息缺失） |
| `100005` | 获取上传凭证失败（存储配置不存在或不可用） |

---

## 五、记录上传结果

客户端直传文件到对象存储成功后，回调此接口通知服务端上传完成。需携带凭证签发时返回的 `recordId` + `secret` 进行校验。

```
POST /client/v1/storage/records
```

**权限**：开放平台签名

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| recordId | uint | 是 | 上传记录 ID（从获取凭证接口返回） |
| secret | string | 是 | 上传校验密钥（从获取凭证接口返回） |
| objectKey | string | 否 | 对象存储 Key |
| fileUrl | string | 否 | 文件访问 URL |
| fileSize | int64 | 否 | 文件大小（字节） |
| mimeType | string | 否 | MIME 类型 |
| md5 | string | 否 | 文件 MD5 哈希 |

**请求示例**：

```json
{
  "recordId": 42,
  "secret": "a1b2c3d4e5f6...",
  "objectKey": "uploads/2025/01/01HXYZ1234567890ABCDEFG.png",
  "fileUrl": "https://cdn.example.com/uploads/2025/01/01HXYZ1234567890ABCDEFG.png",
  "fileSize": 102400,
  "mimeType": "image/png",
  "md5": "d41d8cd98f00b204e9800998ecf8427e"
}
```

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": null
}
```

> **说明**：服务端会根据 `recordId` + `secret` 校验上传凭证的合法性，校验通过后将上传记录状态从 `pending` 置为 `uploaded`。

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败（recordId、secret 必填） |
| `100002` | 未授权（应用信息缺失） |
| `101501` | 上传记录不存在 |
| `101502` | 上传凭证校验失败（recordId + secret 不匹配） |
| `101503` | 该上传记录已完成，不可重复提交 |
| `101504` | 上传凭证已过期 |
| `101505` | 上传记录与请求不匹配 |

---

## 六、客户端直传示例

### JavaScript / TypeScript（以阿里云 OSS 为例）

```typescript
async function uploadFile(file: File, credentials: ClientCredentials): Promise<string> {
  const { url, method, headers } = credentials;

  const uploadHeaders: Record<string, string> = { ...headers };

  const response = await fetch(url, {
    method,
    headers: uploadHeaders,
    body: file,
  });

  if (!response.ok) {
    throw new Error(`Upload failed: ${response.status}`);
  }

  return credentials.finalUrl;
}
```

### 完整上传流程示例

```typescript
async function fullUploadFlow(file: File): Promise<string> {
  // 1. 获取上传凭证
  const credRes = await fetch('/client/v1/storage/credentials', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-App-Key': appKey,
      'X-Timestamp': timestamp,
      'X-Nonce': nonce,
      'X-Signature': signature,
    },
    body: JSON.stringify({
      fileName: file.name,
      contentType: file.type,
      fileSize: file.size,
    }),
  });
  const credData = (await credRes.json()).data;

  // 2. 直传文件到对象存储
  const finalUrl = await uploadFile(file, credData);

  // 3. 记录上传结果（携带 recordId + secret）
  await fetch('/client/v1/storage/records', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-App-Key': appKey,
      'X-Timestamp': timestamp,
      'X-Nonce': nonce,
      'X-Signature': signature,
    },
    body: JSON.stringify({
      recordId: credData.recordId,
      secret: credData.secret,
      objectKey: credData.objectKey,
      fileUrl: credData.finalUrl,
      fileSize: file.size,
      mimeType: file.type,
    }),
  });

  return finalUrl;
}
```
