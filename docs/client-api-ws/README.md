# NetyAdmin 客户端 API 文档

> NetyAdmin 开放平台客户端 API 完整文档。面向第三方开发者，涵盖认证签名、用户管理、内容、存储、消息等全部接口。

---

## 快速入门

1. **阅读认证指南**：先阅读 [00-authentication.md](./00-authentication.md) 了解开放平台签名机制和用户 JWT Token
2. **获取应用凭证**：在管理后台「开放平台 → 应用管理」中创建应用，获取 AppKey 和 AppSecret
3. **配置 API 权限**：为应用授权所需的 API 接口权限
4. **开始对接**：根据各模块文档完成接口对接

---

## 文档目录

| 文档 | 说明 |
|------|------|
| [00-authentication.md](./00-authentication.md) | 认证与签名指南 — 开放平台 HMAC-SHA256 签名算法、用户 JWT Token、统一响应格式、频率限制 |
| [01-auth.md](./01-auth.md) | 认证模块 API — 图形验证码、场景验证配置、发送短信/邮箱验证码 |
| [02-user.md](./02-user.md) | 用户模块 API — 注册、登录、刷新令牌、找回密码、个人资料、修改密码、注销、上传凭证 |
| [03-content.md](./03-content.md) | 内容模块 API — 文章列表/详情/点赞、Banner 组获取/点击 |
| [04-storage.md](./04-storage.md) | 存储模块 API — 获取上传凭证、记录上传结果（客户端直传模式） |
| [05-message.md](./05-message.md) | 消息模块 API — 站内消息列表/详情/已读/全部已读/未读数 |
| [06-echo.md](./06-echo.md) | 示例接口 — 回显测试，用于验证开放平台签名是否正确配置 |
| [06-error-codes.md](./06-error-codes.md) | 错误码参考表 — 全部客户端相关错误码及说明 |

---

## 认证机制

NetyAdmin 客户端 API 采用**双层认证**：

| 认证层 | 机制 | 适用接口 |
|--------|------|----------|
| 开放平台签名 | HMAC-SHA256（`X-App-Key` / `X-Timestamp` / `X-Nonce` / `X-Signature`） | 所有 `/client/v1/` 接口 |
| 用户 JWT | Bearer Token（`Authorization: Bearer <token>`） | 需要登录态的接口 |

详见 [00-authentication.md](./00-authentication.md)。

---

## 完整接口列表

### 认证模块（[01-auth.md](./01-auth.md)）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/client/v1/auth/captcha` | 签名 | 获取图形验证码 |
| GET | `/client/v1/auth/scene-config` | 签名 | 获取场景验证配置 |
| POST | `/client/v1/auth/send-code` | 签名 | 发送短信/邮箱验证码 |

### 用户模块（[02-user.md](./02-user.md)）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/client/v1/user/register` | 签名 | 用户注册 |
| POST | `/client/v1/user/login` | 签名 | 用户登录 |
| POST | `/client/v1/user/refresh-token` | 签名 | 刷新令牌 |
| POST | `/client/v1/user/reset-password` | 签名 | 找回密码 |
| GET | `/client/v1/user/profile` | 签名 + JWT | 获取个人资料 |
| PUT | `/client/v1/user/profile` | 签名 + JWT | 更新个人资料 |
| PUT | `/client/v1/user/password` | 签名 + JWT | 修改密码 |
| DELETE | `/client/v1/user/account` | 签名 + JWT | 注销账号 |
| GET | `/client/v1/user/upload-token` | 签名 + JWT | 获取上传凭证 |
| POST | `/client/v1/user/upload-record` | 签名 + JWT | 记录上传结果 |
| POST | `/client/v1/user/logout` | 签名 + JWT | 退出登录 |

### 内容模块（[03-content.md](./03-content.md)）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/client/v1/content/articles` | 签名 | 文章列表 |
| GET | `/client/v1/content/article/:id` | 签名 | 文章详情 |
| POST | `/client/v1/content/article/:id/like` | 签名 | 点赞文章 |
| GET | `/client/v1/content/banners/:code` | 签名 | 获取 Banner 组 |
| POST | `/client/v1/content/banners/:id/click` | 签名 | 记录 Banner 点击 |

### 存储模块（[04-storage.md](./04-storage.md)）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/client/v1/storage/credentials` | 签名 | 获取上传凭证 |
| POST | `/client/v1/storage/records` | 签名 | 记录上传结果 |

### 消息模块（[05-message.md](./05-message.md)）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/client/v1/message/internal` | 签名 + JWT | 站内消息列表 |
| GET | `/client/v1/message/internal/:id` | 签名 + JWT | 站内消息详情 |
| PUT | `/client/v1/message/internal/read` | 签名 + JWT | 标记消息已读 |
| PUT | `/client/v1/message/internal/read-all` | 签名 + JWT | 全部标记已读 |
| GET | `/client/v1/message/internal/unread-count` | 签名 + JWT | 未读消息数量 |

### 示例接口（[06-echo.md](./06-echo.md)）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/client/v1/echo` | 签名 | 回显测试 |

---

## 统一响应格式

```json
{
  "code": "100000",
  "msg": "",
  "data": {},
  "request_id": "req_01HXYZ..."
}
```

- `code` 为 `"100000"` 表示成功，其他为错误码
- 错误码完整列表见 [06-error-codes.md](./06-error-codes.md)

---

## 接口统计

| 模块 | 接口数量 | 需要 JWT |
|------|---------|----------|
| 认证 | 3 | 0 |
| 用户 | 11 | 7 |
| 内容 | 5 | 0 |
| 存储 | 2 | 0 |
| 消息 | 5 | 5 |
| 示例 | 1 | 0 |
| **合计** | **27** | **12** |
