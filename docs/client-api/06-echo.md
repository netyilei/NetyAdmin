# 示例接口 (Echo)

> 本文档包含 Echo 示例接口，用于验证开放平台签名是否正确配置。

---

## 一、接口总览

| 方法 | 路径 | 权限 | Scope | 说明 |
|------|------|------|-------|------|
| POST | /client/v1/echo | 签名 | `echo_test` | 回显测试 |

---

## 二、开放平台权限配置

Echo 接口需要在开放平台**应用管理**中授权 `echo_test` 权限组才能调用。

| Scope Code | 名称 | 包含接口 |
|------------|------|----------|
| `echo_test` | 示例接口 (签名验证) | `POST /client/v1/echo` |

**配置步骤**：

1. 登录管理后台 → 开放平台 → 应用管理
2. 选择目标应用 → 编辑权限范围
3. 勾选 `echo_test` 权限组并保存
4. 使用该应用的 AppKey/AppSecret 调用 Echo 接口

> **提示**：`echo_test` 是最基础的权限组，建议在接入开放平台时优先授权此权限，用于验证签名计算流程是否正确。

---

## 三、回显测试

原样返回客户端发送的消息，并附带应用 ID 和时间戳。用于验证签名计算是否正确。

```
POST /client/v1/echo
```

**权限**：开放平台签名 + `echo_test` Scope

**请求头**：

| Header | 必填 | 说明 |
|--------|------|------|
| X-App-Key | 是 | 应用 AppKey |
| X-Timestamp | 是 | 请求时间戳（秒） |
| X-Nonce | 是 | 随机字符串（防重放） |
| X-Signature | 是 | HMAC-SHA256 签名 |

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| message | string | 是 | 任意消息内容 |

**请求示例**：

```json
{
  "message": "Hello, NetyAdmin!"
}
```

**响应示例**：

```json
{
  "code": "100000",
  "data": {
    "message": "Hello, NetyAdmin!",
    "appId": "01HXYZ1234567890ABCDEFG",
    "timestamp": 1713888000
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| message | string | 原样返回的消息 |
| appId | string | 当前应用的 AppID |
| timestamp | int64 | 服务器时间戳（秒） |

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败（message 必填） |
| `100002` | 缺少签名参数或 AppKey 无效 |
| `100003` | 签名验证失败 |
| `100004` | 权限不足（Scope Mismatch，未授权 `echo_test`） |
