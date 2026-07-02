# 存储管理 API

本模块涵盖存储配置管理、上传记录管理及文件上传凭证功能。存储配置支持多种对象存储提供商（如 S3、OSS、COS 等），上传采用"凭证 + 直传 + 通知"的三步模式。

## 接口概览

### 存储配置管理（需权限）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/storage-configs` | 获取存储配置列表 |
| GET | `/admin/v1/storage-configs/all-enabled` | 获取所有启用的存储配置 |
| GET | `/admin/v1/storage-configs/:id` | 获取存储配置详情 |
| POST | `/admin/v1/storage-configs` | 创建存储配置 |
| PUT | `/admin/v1/storage-configs` | 更新存储配置 |
| DELETE | `/admin/v1/storage-configs/:id` | 删除存储配置 |
| PUT | `/admin/v1/storage-configs/:id/default` | 设置默认存储配置 |
| POST | `/admin/v1/storage-configs/test-upload` | 测试存储上传 |

### 上传记录管理（需权限）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/upload-records` | 获取上传记录列表 |
| GET | `/admin/v1/upload-records/:id` | 获取上传记录详情 |
| DELETE | `/admin/v1/upload-records/:id` | 删除上传记录 |
| POST | `/admin/v1/upload-records/batch-delete` | 批量删除上传记录 |

### 文件上传（需认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/admin/v1/storage/upload-credentials` | 获取上传凭证 |
| POST | `/admin/v1/storage/upload-record` | 上传成功通知 |

---

## 一、存储配置管理

### 1.1 获取存储配置列表

分页获取存储配置列表。

```
GET /admin/v1/storage-configs
```

#### 认证级别

权限接口（JWT + RBAC）

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 当前页码 |
| `size` | int | 否 | 每页数量 |

### 1.2 获取所有启用的存储配置

获取所有状态为启用的存储配置列表，不分页。通常用于下拉选择。

```
GET /admin/v1/storage-configs/all-enabled
```

### 1.3 获取存储配置详情

```
GET /admin/v1/storage-configs/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 存储配置 ID |

### 1.4 创建存储配置

```
POST /admin/v1/storage-configs
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 配置名称 |
| `provider` | string | 是 | 存储提供商 |
| `endpoint` | string | 是 | 端点地址 |
| `region` | string | 否 | 区域 |
| `bucket` | string | 是 | 存储桶名 |
| `accessKey` | string | 是 | 访问密钥 |
| `secretKey` | string | 是 | 秘密密钥 |
| `domain` | string | 否 | 自定义域名 |
| `pathPrefix` | string | 否 | 路径前缀 |
| `isDefault` | bool | 否 | 是否默认 |
| `status` | string | 否 | 状态 |
| `maxFileSize` | int64 | 否 | 最大文件大小（字节） |
| `allowedTypes` | string | 否 | 允许的文件类型 |
| `stsExpireTime` | int | 否 | STS 凭证过期时间（秒，默认 3600） |
| `remark` | string | 否 | 备注 |

#### 请求示例

```json
{
  "name": "阿里云OSS",
  "provider": "oss",
  "endpoint": "https://oss-cn-hangzhou.aliyuncs.com",
  "region": "cn-hangzhou",
  "bucket": "netyadmin",
  "accessKey": "LTAI5tXXX",
  "secretKey": "XXXXXXX",
  "domain": "https://cdn.example.com",
  "pathPrefix": "uploads/",
  "isDefault": true,
  "status": "1",
  "maxFileSize": 10485760,
  "allowedTypes": "jpg,png,gif,mp4",
  "stsExpireTime": 3600
}
```

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "id": 1
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 1.5 更新存储配置

```
PUT /admin/v1/storage-configs
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | uint | 是 | 存储配置 ID |
| `name` | string | 是 | 配置名称 |
| `provider` | string | 是 | 存储提供商 |
| `endpoint` | string | 是 | 端点地址 |
| `region` | string | 否 | 区域 |
| `bucket` | string | 是 | 存储桶名 |
| `accessKey` | string | 否 | 访问密钥（留空不修改） |
| `secretKey` | string | 否 | 秘密密钥（留空不修改） |
| `domain` | string | 否 | 自定义域名 |
| `pathPrefix` | string | 否 | 路径前缀 |
| `isDefault` | bool | 否 | 是否默认 |
| `status` | string | 否 | 状态 |
| `maxFileSize` | int64 | 否 | 最大文件大小 |
| `allowedTypes` | string | 否 | 允许的文件类型 |
| `stsExpireTime` | int | 否 | STS 凭证过期时间 |
| `remark` | string | 否 | 备注 |

### 1.6 删除存储配置

```
DELETE /admin/v1/storage-configs/:id
```

### 1.7 设置默认存储配置

```
PUT /admin/v1/storage-configs/:id/default
```

### 1.8 测试存储上传

使用指定存储配置测试文件上传。

```
POST /admin/v1/storage-configs/test-upload
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `configId` | uint | 是 | 存储配置 ID |
| `fileName` | string | 是 | 测试文件名 |
| `content` | string | 是 | 测试文件内容 |

#### 请求示例

```json
{
  "configId": 1,
  "fileName": "test.txt",
  "content": "Hello, NetyAdmin Storage!"
}
```

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "url": "https://cdn.example.com/uploads/test.txt"
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

---

## 二、上传记录管理

### 2.1 获取上传记录列表

分页获取上传记录列表，支持多维度筛选。

```
GET /admin/v1/upload-records
```

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 当前页码 |
| `size` | int | 否 | 每页数量 |
| `fileName` | string | 否 | 文件名 |
| `source` | string | 否 | 来源 |
| `sourceId` | string | 否 | 来源 ID |
| `businessType` | string | 否 | 业务类型 |
| `businessId` | string | 否 | 业务 ID |
| `mimeType` | string | 否 | MIME 类型 |
| `storageConfigId` | uint | 否 | 存储配置 ID |
| `appId` | string | 否 | 应用 ID |
| `startTime` | string | 否 | 开始时间 |
| `endTime` | string | 否 | 结束时间 |

### 2.2 获取上传记录详情

```
GET /admin/v1/upload-records/:id
```

### 2.3 删除上传记录

```
DELETE /admin/v1/upload-records/:id
```

### 2.4 批量删除上传记录

```
POST /admin/v1/upload-records/batch-delete
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ids` | []uint | 是 | 上传记录 ID 列表 |

#### 请求示例

```json
{
  "ids": [1, 2, 3]
}
```

---

## 三、文件上传

NetyAdmin 采用"凭证 + 直传 + 通知"的三步上传模式：

```
1. 获取上传凭证    POST /admin/v1/storage/upload-credentials
2. 直传文件到存储   使用凭证中的 URL 和 Headers 直接上传文件到对象存储
3. 上传成功通知    POST /admin/v1/storage/upload-record
```

### 3.1 获取上传凭证

获取文件上传凭证，返回包含 recordId 和 secret 的完整上传信息。

```
POST /admin/v1/storage/upload-credentials
```

#### 认证级别

认证接口（需 Bearer Token，无需 RBAC）

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `configId` | uint | 否 | 存储配置 ID（不指定则使用默认配置） |
| `fileName` | string | 是 | 文件名 |
| `contentType` | string | 否 | 文件 MIME 类型 |
| `fileSize` | int64 | 否 | 文件大小（字节） |
| `businessType` | string | 否 | 业务类型 |
| `businessId` | string | 否 | 业务 ID |
| `sourceInfo` | object | 否 | 来源信息（键值对） |

#### 请求示例

```json
{
  "configId": 1,
  "fileName": "avatar.jpg",
  "contentType": "image/jpeg",
  "fileSize": 102400,
  "businessType": "admin_avatar"
}
```

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "url": "https://oss-cn-hangzhou.aliyuncs.com/netyadmin/uploads/avatar.jpg",
    "method": "PUT",
    "headers": {
      "Content-Type": "image/jpeg"
    },
    "expiresAt": "2025-01-01T13:00:00Z",
    "objectKey": "uploads/avatar.jpg",
    "domain": "https://cdn.example.com",
    "finalUrl": "https://cdn.example.com/uploads/avatar.jpg",
    "configId": 1,
    "region": "cn-hangzhou",
    "bucket": "netyadmin",
    "endpoint": "https://oss-cn-hangzhou.aliyuncs.com",
    "pathPrefix": "uploads/",
    "maxFileSize": 10485760,
    "recordId": 100,
    "secret": "abc123def456ghi789"
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

#### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `url` | string | 直传上传 URL |
| `method` | string | 上传 HTTP 方法（通常为 PUT） |
| `headers` | object | 上传请求头 |
| `expiresAt` | string | 凭证过期时间 |
| `objectKey` | string | 对象存储 Key |
| `domain` | string | 自定义域名 |
| `finalUrl` | string | 最终访问 URL |
| `configId` | uint | 使用的存储配置 ID |
| `region` | string | 区域 |
| `bucket` | string | 存储桶名 |
| `endpoint` | string | 端点地址 |
| `pathPrefix` | string | 路径前缀 |
| `maxFileSize` | int64 | 最大文件大小 |
| `recordId` | uint | 上传记录 ID（用于后续通知） |
| `secret` | string | 上传凭证密钥（用于后续通知验签） |

### 3.2 上传成功通知

客户端直传文件到对象存储成功后，通知服务端完成上传记录。

```
POST /admin/v1/storage/upload-record
```

#### 认证级别

认证接口（需 Bearer Token，无需 RBAC）

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `recordId` | uint | 是 | 上传记录 ID（凭证返回） |
| `secret` | string | 是 | 上传凭证密钥（凭证返回） |
| `objectKey` | string | 否 | 对象存储 Key |
| `fileUrl` | string | 否 | 文件 URL |
| `fileSize` | int64 | 否 | 文件大小 |
| `mimeType` | string | 否 | MIME 类型 |
| `md5` | string | 否 | 文件 MD5 值 |

#### 请求示例

```json
{
  "recordId": 100,
  "secret": "abc123def456ghi789",
  "objectKey": "uploads/avatar.jpg",
  "fileUrl": "https://cdn.example.com/uploads/avatar.jpg",
  "fileSize": 102400,
  "mimeType": "image/jpeg",
  "md5": "d41d8cd98f00b204e9800998ecf8427e"
}
```

#### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `101501` | 上传记录不存在 |
| `101502` | 上传凭证校验失败 |
| `101503` | 该上传记录已完成，不可重复提交 |
| `101504` | 上传凭证已过期 |
| `101505` | 上传记录与请求不匹配 |
