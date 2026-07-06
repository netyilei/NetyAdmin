# GM Admin API 认证指南

## 概述

NetyAdmin 管理后台 API 采用基于 **JWT (JSON Web Token)** 的认证机制。所有 API 请求的基础路径为：

```
/admin/v1
```

系统通过三个中间件组对接口进行分级保护，实现精细化的访问控制。

---

## 一、登录流程

完整的登录认证流程如下：

```
1. 获取验证码（可选）
   GET /admin/v1/common/captcha
   -> 返回 captchaId + captchaImg (Base64 图片)

2. 管理员登录
   POST /admin/v1/auth/login
   -> 返回 accessToken (token) + refreshToken

3. 携带 Token 访问受保护接口
   Authorization: Bearer <accessToken>

4. Token 过期后刷新
   POST /admin/v1/auth/refresh-token
   -> 返回新的 accessToken + refreshToken

5. 退出登录
   POST /admin/v1/auth/logout
```

### 流程图

```
┌─────────────┐     ┌──────────────┐     ┌──────────────────┐
│  获取验证码  │────▶│  提交登录     │────▶│ 获取 accessToken  │
│  (可选)     │     │  POST /login  │     │ + refreshToken    │
└─────────────┘     └──────────────┘     └────────┬─────────┘
                                                   │
                                                   ▼
                                          ┌──────────────────┐
                                          │ 携带 Bearer Token │
                                          │ 访问受保护接口     │
                                          └────────┬─────────┘
                                                   │
                                          Token 过期│
                                                   ▼
                                          ┌───────────────────┐
                                          │ 刷新令牌           │
                                          │ POST /refresh-token│
                                          └───────────────────┘
```

---

## 二、JWT Token 格式

### 请求头格式

所有需要认证的接口必须在 HTTP 请求头中携带 JWT Token：

```
Authorization: Bearer <accessToken>
```

### Token 结构

JWT Token 默认使用 **RS256**（RSA 非对称签名）算法签名（可通过 `[jwt].algorithm` 配置切换为 HS256），由三部分组成（以 `.` 分隔）：

```
<Header>.<Payload>.<Signature>
```

**Header:**
```json
{
  "alg": "RS256",
  "typ": "JWT"
}
```

**Payload (AdminClaims):**
```json
{
  "userId": 1,
  "userName": "admin",
  "roles": ["super_admin"],
  "exp": 1783000000,
  "iat": 1782913600,
  "sub": "access"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `userId` | uint | 管理员 ID |
| `userName` | string | 管理员用户名 |
| `roles` | []string | 角色编码列表 |
| `exp` | int64 | 过期时间戳（Unix） |
| `iat` | int64 | 签发时间戳（Unix） |
| `sub` | string | Token 类型：`access` 或 `refresh` |

---

## 三、Token 过期时间

| Token 类型 | 过期时间 | 说明 |
|------------|----------|------|
| AccessToken | **30 分钟**（按 `[jwt].access_token_ttl` 配置） | 用于访问受保护接口，`sub` 为 `access` |
| RefreshToken | **168 小时**（7 天，按 `[jwt].refresh_token_ttl` 配置） | 用于刷新 AccessToken，`sub` 为 `refresh` |

> 注意：Token 实际过期时间会附加最多 600 秒的随机抖动（jitter），以防止 Token 雪崩。

---

## 四、Token 刷新机制

当 AccessToken 过期时，客户端可使用 RefreshToken 换取新的令牌对：

```
POST /admin/v1/auth/refresh-token
Content-Type: application/json

{
  "refreshToken": "<refreshToken>"
}
```

刷新成功后返回全新的 accessToken 和 refreshToken，旧 Token 失效。

---

## 五、中间件分组

系统将所有接口分为三个中间件组，按安全级别递增：

### 1. Public（公开接口）

- **认证要求**：无
- **中间件**：仅经过全局中间件（RequestID、IPAC）
- **适用场景**：登录、获取验证码、获取公开配置、获取字典数据

```
POST   /admin/v1/auth/login
POST   /admin/v1/auth/refresh-token
GET    /admin/v1/common/captcha
GET    /admin/v1/system/configs
GET    /admin/v1/system/dict/data/:code
```

### 2. Auth（认证接口）

- **认证要求**：JWT Token 必填
- **中间件**：`JWTAuth()`
- **适用场景**：获取个人信息、修改密码、退出登录、获取上传凭证、获取用户路由

```
GET    /admin/v1/auth/user-info
GET    /admin/v1/auth/profile
PUT    /admin/v1/auth/profile
POST   /admin/v1/auth/change-password
POST   /admin/v1/auth/logout
POST   /admin/v1/storage/upload-credentials
POST   /admin/v1/storage/upload-record
GET    /admin/v1/route/getUserRoutes
GET    /admin/v1/route/isRouteExist
```

### 3. Permission（权限接口）

- **认证要求**：JWT Token + RBAC 权限校验
- **中间件**：`JWTAuth()` + `PermissionAuth(authVerifier)`
- **适用场景**：所有管理类操作（管理员管理、角色管理、菜单管理、内容管理等）

```
GET    /admin/v1/admins
POST   /admin/v1/admins
GET    /admin/v1/systemManage/getRoleList
POST   /admin/v1/systemManage/addRole
...
```

---

## 六、JWT 认证中间件工作流程

`JWTAuth()` 中间件执行以下校验：

1. **提取 Token**：从 `Authorization` 请求头中提取 `Bearer <token>` 格式的 Token
2. **解析 Token**：使用 JWT 库验证签名并解析 Claims
3. **校验 Token 类型**：确保 `sub` 字段为 `access`（不接受 refresh token 访问接口）
4. **校验 Token 有效性**：查询 TokenStore，确认 Token 未被吊销（支持改密码/禁用/登出后立即失效）
5. **校验账户状态**：查询管理员账户，确认账户未被禁用或删除
6. **注入上下文**：将 `adminID`、`username`、`roles` 注入 Gin Context 供后续使用

如果任何一步校验失败，中间件将终止请求并返回对应错误码。

---

## 七、认证错误码

| 错误码 | 常量名 | 说明 |
|--------|--------|------|
| `100002` | CodeUnauthorized | 未授权（缺少 Authorization 头或 Token 已被吊销） |
| `100003` | CodeForbidden | 无权限（RBAC 权限校验不通过） |
| `101005` | CodeTokenExpired | 令牌已过期 |
| `101006` | CodeTokenInvalid | 令牌无效（格式错误、签名错误或用途不正确） |

### 错误响应示例

**缺少 Token：**
```json
{
  "code": "100002",
  "msg": "未授权",
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

**Token 过期：**
```json
{
  "code": "101006",
  "msg": "令牌无效",
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

**权限不足：**
```json
{
  "code": "100003",
  "msg": "无权限",
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

---

## 八、统一响应格式

所有接口均返回统一的 JSON 响应结构：

```json
{
  "code": "100000",
  "msg": "",
  "data": {},
  "request_id": "uuid"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | string | 业务状态码，`100000` 表示成功，其他为错误码 |
| `msg` | string | 提示信息，始终为空字符串；前端应基于 `code` 映射用户可见文本 |
| `data` | any | 响应数据，成功时返回具体数据，失败时省略 |
| `request_id` | string | 请求唯一标识（UUID），用于链路追踪 |

### 分页响应格式

分页接口的 `data` 字段结构：

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "records": [],
    "current": 1,
    "size": 10,
    "total": 100
  },
  "request_id": "uuid"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `records` | array | 当前页数据列表 |
| `current` | int | 当前页码 |
| `size` | int | 每页数量 |
| `total` | int64 | 总记录数 |

---

## 九、全局中间件

除了上述认证中间件外，所有请求都会经过以下全局中间件：

| 中间件 | 说明 |
|--------|------|
| `RequestID()` | 为每个请求注入唯一 `request_id`（UUID），并写入响应头和响应体 |
| `IPACAuth(ipacSvc)` | IP 访问控制，校验请求来源 IP 是否在黑名单/白名单中 |

> RequestID 中间件在应用初始化时全局注册，IPAC 中间件在路由注册时挂载。
