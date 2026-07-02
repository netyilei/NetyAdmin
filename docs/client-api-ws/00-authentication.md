# 客户端 API 认证与签名指南

> 本文档面向第三方开发者，说明如何对接 NetyAdmin 开放平台的认证机制，包括签名计算、Token 传递和请求规范。

---

## 文档索引

| 文档 | 说明 |
|------|------|
| [00-authentication.md](./00-authentication.md) | 认证与签名指南（本文档） |
| [01-auth.md](./01-auth.md) | 验证码 / 场景配置 / 发送验证码 |
| [02-user.md](./02-user.md) | 注册 / 登录 / 找回密码 / 用户资料 / 修改密码 / 注销 / 上传凭证 |
| [03-content.md](./03-content.md) | 文章 / Banner |
| [04-storage.md](./04-storage.md) | 文件上传（凭证 + 直传 + 回调） |
| [05-message.md](./05-message.md) | 站内信（列表 / 详情 / 已读 / 未读数） |
| [06-error-codes.md](./06-error-codes.md) | 错误码参考表 |

---

## 一、概述

NetyAdmin 客户端 API 采用**双层认证**机制：

| 层级 | 机制 | 用途 | 适用接口 |
|------|------|------|----------|
| 第一层 | 开放平台签名 (HMAC-SHA256) | 验证应用身份 | 所有 `/client/v1/` 接口 |
| 第二层 | 用户 JWT Token (Bearer) | 验证用户身份 | 需要登录态的接口 |

**所有客户端接口都必须携带开放平台签名**，部分接口还需要额外的用户 JWT Token。

---

## 二、开放平台签名

### 2.1 获取凭证

在管理后台的「开放平台 → 应用管理」中创建应用，获取：

- **AppKey**：应用标识，明文传输
- **AppSecret**：应用密钥，**仅用于本地签名计算，绝不在网络中传输**

### 2.2 签名流程

每次请求需在 HTTP Header 中携带以下 4 个字段：

| Header | 说明 | 示例 |
|--------|------|------|
| `X-App-Key` | 应用 AppKey | `01HXYZ1234567890ABCDEFG` |
| `X-Timestamp` | Unix 时间戳（秒） | `1713888000` |
| `X-Nonce` | 随机字符串，2分钟内不可重复 | `a1b2c3d4e5f6` |
| `X-Signature` | HMAC-SHA256 签名值（Base64 编码） | `3f8k2j...=` |

### 2.3 签名计算步骤

#### Step 1：构造待签名字符串 (StringToSign)

```
Method\nPath\nTimestamp\nNonce\nPayload
```

各字段说明：

| 字段 | 说明 |
|------|------|
| Method | HTTP 方法，大写，如 `GET`、`POST` |
| Path | 请求路径，不含域名和 Query，如 `/client/v1/user/login` |
| Timestamp | 与 `X-Timestamp` 一致 |
| Nonce | 与 `X-Nonce` 一致 |
| Payload | GET 请求：Query 参数按 key 字典序排列拼接 `key1=value1&key2=value2`；POST/PUT/DELETE 请求：Body 的 SHA256 哈希（十六进制小写）；无 Body 时为空字符串 |

#### Step 2：计算签名

```
Signature = Base64(HMAC-SHA256(AppSecret, StringToSign))
```

- 密钥 (key)：AppSecret
- 数据 (data)：StringToSign（以换行符 `\n` 连接的 5 段文本）
- 输出：Base64 编码字符串

### 2.4 签名示例

#### POST 请求签名示例

```json
POST /client/v1/user/login
Content-Type: application/json

{"userName":"test","password":"123456"}
```

1. Body SHA256 = `a1b2c3d4...`（64 位十六进制小写）
2. StringToSign:

   ```
   POST
   /client/v1/user/login
   1713888000
   a1b2c3d4e5f6
   a1b2c3d4...
   ```

3. Signature = `Base64(HMAC-SHA256(AppSecret, StringToSign))`

#### GET 请求签名示例

```
GET /client/v1/content/articles?categoryId=1&page=1&pageSize=10
```

1. Query 按字典序排列：`categoryId=1&page=1&pageSize=10`
2. StringToSign:

   ```
   GET
   /client/v1/content/articles
   1713888000
   a1b2c3d4e5f6
   categoryId=1&page=1&pageSize=10
   ```

3. Signature = `Base64(HMAC-SHA256(AppSecret, StringToSign))`

### 2.5 代码示例

#### JavaScript / TypeScript

```typescript
import CryptoJS from 'crypto-js';

function signRequest(
  method: string,
  path: string,
  appSecret: string,
  body?: string,
  queryParams?: Record<string, string>
): { timestamp: string; nonce: string; signature: string } {
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const nonce = CryptoJS.lib.WordArray.random(16).toString();

  let payload: string;
  if (method === 'GET' && queryParams) {
    const keys = Object.keys(queryParams).sort();
    payload = keys.map(k => `${k}=${queryParams[k]}`).join('&');
  } else {
    payload = body ? CryptoJS.SHA256(body).toString() : '';
  }

  const stringToSign = [method, path, timestamp, nonce, payload].join('\n');
  const signature = CryptoJS.HmacSHA256(stringToSign, appSecret).toString(CryptoJS.enc.Base64);

  return { timestamp, nonce, signature };
}
```

#### Go

```go
import (
    "crypto/hmac"
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "fmt"
    "sort"
    "strings"
    "time"
)

type SignResult struct {
    Timestamp string
    Nonce     string
    Signature string
}

func SignRequest(method, path, appSecret string, body []byte, query map[string]string) *SignResult {
    timestamp := fmt.Sprintf("%d", time.Now().Unix())

    nonceBytes := make([]byte, 16)
    rand.Read(nonceBytes)
    nonce := hex.EncodeToString(nonceBytes)

    var payload string
    if method == "GET" && len(query) > 0 {
        keys := make([]string, 0, len(query))
        for k := range query {
            keys = append(keys, k)
        }
        sort.Strings(keys)
        var sb strings.Builder
        for i, k := range keys {
            if i > 0 {
                sb.WriteString("&")
            }
            sb.WriteString(k)
            sb.WriteString("=")
            sb.WriteString(query[k])
        }
        payload = sb.String()
    } else if len(body) > 0 {
        h := sha256.New()
        h.Write(body)
        payload = hex.EncodeToString(h.Sum(nil))
    }

    stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", method, path, timestamp, nonce, payload)

    mac := hmac.New(sha256.New, []byte(appSecret))
    mac.Write([]byte(stringToSign))
    signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

    return &SignResult{Timestamp: timestamp, Nonce: nonce, Signature: signature}
}
```

---

## 三、用户 JWT Token

### 3.1 获取 Token

用户通过 `POST /client/v1/user/login` 接口登录后，返回：

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "accessToken": "eyJhbGciOi...",
    "refreshToken": "eyJhbGciOi...",
    "expiresIn": 7200
  }
}
```

### 3.2 携带 Token

需要用户身份的接口，在请求 Header 中携带：

```
Authorization: Bearer <accessToken>
```

**注意**：同时仍需携带开放平台签名 Header（`X-App-Key` 等），双层认证缺一不可。

### 3.3 Token 刷新

`accessToken` 过期后，使用 `refreshToken` 调用 `POST /client/v1/user/refresh-token` 获取新的 Token 对。

### 3.4 Token 有效期

| Token 类型 | 默认有效期 | 用途 |
|-----------|-----------|------|
| accessToken | 2 小时 | 接口鉴权 |
| refreshToken | 7 天 | 刷新 accessToken |

---

## 四、统一响应格式

所有接口返回统一的 JSON 结构：

```json
{
  "code": "100000",
  "msg": "",
  "data": {},
  "request_id": "req_01HXYZ..."
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| code | string | 状态码，`"100000"` 表示成功，其他为错误码 |
| msg | string | 消息描述，成功时为空字符串，失败时为错误描述 |
| data | any | 业务数据，失败时可能不存在 |
| request_id | string | 请求追踪 ID |

### 分页响应格式

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
  "request_id": "req_01HXYZ..."
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| records | array | 当前页数据列表 |
| current | int | 当前页码 |
| size | int | 每页条数 |
| total | int64 | 总记录数 |

---

## 五、认证错误码

### 5.1 开放平台签名错误码

| code | 说明 |
|------|------|
| `101301` | AppKey 无效 |
| `101302` | 签名验证失败（含 Nonce 重复） |
| `101303` | 请求已过期（时间戳超出 ±60s） |
| `101304` | 权限不足（Scope 不匹配） |
| `101305` | 已触发流量限制 |
| `101306` | 应用已被禁用 |

### 5.2 IP 访问控制错误码

| code | 说明 |
|------|------|
| `101401` | IP 访问受限（被封禁或校验服务异常） |
| `101402` | 非法 IP/CIDR 格式 |
| `101403` | 白名单模式，IP 未授权 |

> 完整错误码列表见 [06-error-codes.md](./06-error-codes.md)。

---

## 六、接口权限层级

| 层级 | 需要 Header | 适用接口 |
|------|------------|----------|
| 开放平台签名 | `X-App-Key` + `X-Timestamp` + `X-Nonce` + `X-Signature` | 所有 `/client/v1/` 接口 |
| 用户 JWT | 上述 + `Authorization: Bearer <token>` | 用户个人信息、消息、修改密码等 |

---

## 七、频率限制

- 每个应用有独立的请求频率限制（在管理后台「应用管理」中配置）
- 验证码发送：同一目标 60 秒内仅允许发送一次
- Nonce 防重放：同一 Nonce 在 2 分钟内不可重复使用（覆盖 ±60s 时间戳容差窗口）
- 时间戳容差：±60 秒
