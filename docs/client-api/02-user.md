# 用户中心 API

> 本文档包含用户个人信息、修改密码、注销账号等接口。所有接口均需开放平台签名，部分接口需额外携带用户 JWT Token。

---

## 一、接口总览

| 方法 | 路径 | 权限 | Scope | 说明 |
|------|------|------|-------|------|
| GET | /client/v1/user/profile | 签名 + JWT | `user_profile` | 获取个人资料 |
| PUT | /client/v1/user/profile | 签名 + JWT | `user_profile` | 更新个人资料 |
| PUT | /client/v1/user/password | 签名 + JWT | `user_profile` | 修改密码 |
| DELETE | /client/v1/user/account | 签名 + JWT | `user_profile` | 注销账号 |
| GET | /client/v1/user/upload-token | 签名 + JWT | `user_profile` | 获取上传凭证 |
| POST | /client/v1/user/upload-record | 签名 + JWT | `user_profile` | 记录上传结果 |
| POST | /client/v1/user/logout | 签名 + JWT | `user_profile` | 退出登录 |

> 登录、注册、找回密码、刷新令牌接口见 [01-auth.md](01-auth.md)。

---

## 二、获取个人资料

```
GET /client/v1/user/profile
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**：无

**响应示例**：

```json
{
  "code": "100000",
  "data": {
    "id": "01HXYZ1234567890ABCDEFG",
    "userName": "testuser",
    "nickName": "测试用户",
    "avatar": "https://cdn.example.com/avatar/xxx.jpg",
    "phone": "138****1234",
    "email": "user@example.com",
    "gender": "1",
    "status": "1",
    "lastLoginAt": "2025-01-01T12:00:00Z"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 用户 ID (ULID) |
| userName | string | 用户名 |
| nickName | string | 昵称 |
| avatar | string | 头像 URL |
| phone | string | 手机号 |
| email | string | 邮箱 |
| gender | string | 性别：`0` 未知 / `1` 男 / `2` 女 |
| status | string | 状态：`1` 正常 / `0` 禁用 |
| lastLoginAt | string | 最后登录时间（ISO 8601） |

**可能错误码**：`100002`（未授权）

---

## 三、更新个人资料

```
PUT /client/v1/user/profile
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| nickname | string | 否 | 昵称 |
| avatar | string | 否 | 头像 URL |
| gender | string | 否 | 性别：`0` / `1` / `2` |
| email | string | 否 | 邮箱 |
| phone | string | 否 | 手机号 |

**请求示例**：

```json
{
  "nickname": "新昵称",
  "gender": "1"
}
```

**响应示例**：

```json
{
  "code": "100000",
  "data": null
}
```

**可能错误码**：`100001`（参数校验失败）、`100002`（未授权）

---

## 四、修改密码

用户已登录状态下修改密码，需提供原密码。

```
PUT /client/v1/user/password
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| oldPassword | string | 是 | 原密码 |
| newPassword | string | 是 | 新密码，6-20 位 |

**请求示例**：

```json
{
  "oldPassword": "OldPassword123",
  "newPassword": "NewPassword456"
}
```

**响应示例**：

```json
{
  "code": "100000",
  "data": null
}
```

> **注意**：修改密码成功后，该用户的所有在线会话将被强制下线（Token 全部失效），需重新登录。

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败 |
| `100002` | 未授权 |
| `101008` | 原密码错误 |

---

## 五、注销账号

永久注销当前用户账号，操作不可逆。

```
DELETE /client/v1/user/account
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**：无

**响应示例**：

```json
{
  "code": "100000",
  "data": null
}
```

> **注意**：注销后该用户的所有数据将被软删除，所有在线会话立即失效。

**可能错误码**：`100002`（未授权）

---

## 六、获取上传凭证

获取文件上传的预签名 URL 和相关凭证。用于用户端直传文件到对象存储。

```
GET /client/v1/user/upload-token
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**：无

**响应示例**：

```json
{
  "code": "100000",
  "data": {
    "uploadUrl": "https://oss.example.com/upload?signature=xxx",
    "storageConfigId": 1
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| uploadUrl | string | 预签名上传 URL |
| storageConfigId | int | 存储配置 ID |

**可能错误码**：`100002`（未授权）、`100005`（获取上传凭证失败）

---

## 七、记录上传结果

用户直传文件到对象存储成功后，回调此接口记录上传信息。与 `/user/upload-token` 配套使用。

```
POST /client/v1/user/upload-record
```

**权限**：开放平台签名 + 用户 JWT + `user_profile` Scope

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| fileName | string | 是 | 文件名 |
| objectKey | string | 是 | 对象存储 Key（从获取凭证接口返回的路径） |
| fileSize | int64 | 否 | 文件大小（字节） |
| mimeType | string | 否 | MIME 类型 |
| md5 | string | 否 | 文件 MD5 哈希 |
| storageConfigId | int | 否 | 存储配置 ID（从获取凭证接口返回） |
| businessType | string | 否 | 业务类型标识，如 `avatar` |
| businessId | string | 否 | 业务关联 ID |

**请求示例**：

```json
{
  "fileName": "avatar.png",
  "objectKey": "user/01HXYZ1234567890ABCDEFG/avatar.png",
  "fileSize": 102400,
  "mimeType": "image/png",
  "md5": "d41d8cd98f00b204e9800998ecf8427e",
  "storageConfigId": 1,
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

> **提示**：上传记录回调是可选步骤，但强烈建议执行，以便后端追踪文件归属和管理存储空间。

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败（fileName、objectKey 必填） |
| `100002` | 未授权 |
| `100005` | 记录上传结果失败 |

---

## 八、退出登录

使当前 Token 失效，退出登录状态。

```
POST /client/v1/user/logout
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**：无

**响应示例**：

```json
{
  "code": "100000",
  "data": null
}
```

**可能错误码**：`100002`（未授权）
