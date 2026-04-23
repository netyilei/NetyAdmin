# 示例接口 (Echo)

> 本文档包含 Echo 示例接口，用于验证开放平台签名是否正确配置。

---

## 一、接口总览

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | /client/v1/echo | 签名 | 回显测试 |

---

## 二、回显测试

原样返回客户端发送的消息，并附带应用 ID 和时间戳。用于验证签名计算是否正确。

```
POST /client/v1/echo
```

**权限**：开放平台签名

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

**可能错误码**：`100001`（参数校验失败，message 必填）
