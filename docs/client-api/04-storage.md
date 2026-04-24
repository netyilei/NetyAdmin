# 存储与上传 API

> 本文档包含客户端文件上传相关的接口。采用**客户端直传**模式：先获取上传凭证，客户端直接上传至对象存储，最后回调记录上传结果。所有接口均需开放平台签名。

---

## 一、上传流程

```
┌──────────────────────────────────────────────────────────────┐
│ 1. POST /client/v1/storage/credentials                       │
│    → 获取上传凭证（预签名 URL、Headers、ObjectKey 等）         │
├──────────────────────────────────────────────────────────────┤
│ 2. 客户端使用凭证直接上传文件到对象存储                         │
│    → PUT/POST 至 credentials.url，携带 credentials.headers    │
├──────────────────────────────────────────────────────────────┤
│ 3. POST /client/v1/storage/records                            │
│    → 上传成功后回调，记录上传结果                               │
└──────────────────────────────────────────────────────────────┘
```

**设计说明**：

- 客户端不经过后端中转文件，直接上传至对象存储（S3/OSS/COS 等），减轻服务器压力
- 上传凭证有时效性（通常 15-30 分钟），过期需重新获取
- 上传记录回调是可选步骤，但强烈建议执行，以便后端追踪文件归属

---

## 二、接口总览

| 方法 | 路径 | 权限 | Scope | 说明 |
|------|------|------|-------|------|
| POST | /client/v1/storage/credentials | 签名 | `storage_upload` | 获取上传凭证 |
| POST | /client/v1/storage/records | 签名 | `storage_upload` | 记录上传结果 |

---

## 三、开放平台权限配置

存储上传接口需要在开放平台**应用管理**中授权 `storage_upload` 权限组才能调用。

| Scope Code | 名称 | 包含接口 |
|------------|------|----------|
| `storage_upload` | 存储上传 (凭证/记录) | `POST /client/v1/storage/credentials`、`POST /client/v1/storage/records` |

**配置步骤**：

1. 登录管理后台 → 开放平台 → 应用管理
2. 选择目标应用 → 编辑权限范围
3. 勾选 `storage_upload` 权限组并保存
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
    "maxFileSize": 10485760
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

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败（fileName 必填） |
| `100002` | 未授权（应用信息缺失） |
| `100005` | 获取上传凭证失败（存储配置不存在或不可用） |

---

## 五、记录上传结果

客户端直传文件到对象存储成功后，回调此接口记录上传信息。

```
POST /client/v1/storage/records
```

**权限**：开放平台签名

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| fileName | string | 是 | 文件名 |
| objectKey | string | 是 | 对象存储 Key（从获取凭证接口返回） |
| fileSize | int64 | 否 | 文件大小（字节） |
| mimeType | string | 否 | MIME 类型 |
| md5 | string | 否 | 文件 MD5 哈希 |
| businessType | string | 否 | 业务类型标识 |
| businessId | string | 否 | 业务关联 ID |
| sourceInfo | string | 否 | 额外来源信息（JSON 字符串） |

**请求示例**：

```json
{
  "fileName": "avatar.png",
  "objectKey": "uploads/2025/01/01HXYZ1234567890ABCDEFG.png",
  "fileSize": 102400,
  "mimeType": "image/png",
  "md5": "d41d8cd98f00b204e9800998ecf8427e",
  "businessType": "avatar"
}
```

**响应示例**：

```json
{
  "code": "100000",
  "data": null
}
```

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败（fileName、objectKey 必填） |
| `100002` | 未授权（应用信息缺失） |
| `100005` | 记录上传结果失败 |

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

  // 3. 记录上传结果
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
      fileName: file.name,
      objectKey: credData.objectKey,
      fileSize: file.size,
      mimeType: file.type,
    }),
  });

  return finalUrl;
}
```
