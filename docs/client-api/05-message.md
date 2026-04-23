# 消息模块 API

> 本文档包含站内信相关的所有接口。所有接口均需开放平台签名 + 用户 JWT Token。

---

## 一、接口总览

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | /client/v1/message/internal | 签名 + JWT | 站内信列表 |
| GET | /client/v1/message/internal/:id | 签名 + JWT | 站内信详情 |
| PUT | /client/v1/message/internal/read | 签名 + JWT | 标记已读 |
| PUT | /client/v1/message/internal/read-all | 签名 + JWT | 全部标记已读 |
| GET | /client/v1/message/internal/unread-count | 签名 + JWT | 未读消息数 |

---

## 二、站内信列表

获取当前用户的站内信列表，支持分页和已读状态过滤。

```
GET /client/v1/message/internal
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**（Query）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认 1 |
| pageSize | int | 否 | 每页条数，默认 10，最大 100 |
| readFilter | int | 否 | 已读过滤：`0` 未读 / `1` 已读，不传则返回全部 |

**请求示例**：

```
GET /client/v1/message/internal?page=1&pageSize=10&readFilter=0
```

**响应示例**：

```json
{
  "code": "100000",
  "data": {
    "records": [
      {
        "msgInternalId": 1,
        "title": "系统通知",
        "content": "欢迎使用 NetyAdmin",
        "isRead": false,
        "createdAt": "2025-01-15T10:00:00Z"
      }
    ],
    "current": 1,
    "size": 10,
    "total": 5
  }
}
```

**可能错误码**：`100001`（参数校验失败）、`100002`（未授权）

---

## 三、站内信详情

获取单条站内信的详细内容。

```
GET /client/v1/message/internal/:id
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**（Path）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | uint | 是 | 站内信 ID |

**响应示例**：

```json
{
  "code": "100000",
  "data": {
    "msgInternalId": 1,
    "title": "系统通知",
    "content": "欢迎使用 NetyAdmin 平台...",
    "isRead": true,
    "createdAt": "2025-01-15T10:00:00Z"
  }
}
```

**可能错误码**：`100001`（无效的 ID）、`100002`（未授权）、`100004`（消息不存在）

---

## 四、标记已读

将指定站内信标记为已读。

```
PUT /client/v1/message/internal/read
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| msgInternalId | uint64 | 是 | 站内信 ID |

**请求示例**：

```json
{
  "msgInternalId": 1
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

## 五、全部标记已读

将当前用户的所有站内信标记为已读。

```
PUT /client/v1/message/internal/read-all
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

---

## 六、未读消息数

获取当前用户的未读站内信数量。

```
GET /client/v1/message/internal/unread-count
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**：无

**响应示例**：

```json
{
  "code": "100000",
  "data": {
    "unreadCount": 3
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| unreadCount | int | 未读消息数量 |

**可能错误码**：`100002`（未授权）
