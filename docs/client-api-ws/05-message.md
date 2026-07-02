# 站内消息 API

> 本文档包含站内消息的列表、详情、标记已读、全部已读、未读数等接口。所有接口均需开放平台签名 + 用户 JWT Token。

---

## 一、接口总览

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | /client/v1/message/internal | 签名 + JWT | 站内消息列表 |
| GET | /client/v1/message/internal/:id | 签名 + JWT | 站内消息详情 |
| PUT | /client/v1/message/internal/read | 签名 + JWT | 标记消息已读 |
| PUT | /client/v1/message/internal/read-all | 签名 + JWT | 全部标记已读 |
| GET | /client/v1/message/internal/unread-count | 签名 + JWT | 未读消息数量 |

---

## 二、站内消息列表

分页获取当前用户的站内消息列表，支持按已读状态过滤。

```
GET /client/v1/message/internal
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**（Query）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认 1，最小 1 |
| pageSize | int | 否 | 每页条数，默认 10，最小 1，最大 100 |
| readFilter | int | 否 | 已读过滤：`0` 未读 / `1` 已读，不传则返回全部 |

**请求示例**：

```
GET /client/v1/message/internal?page=1&pageSize=20&readFilter=0
```

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "records": [
      {
        "msgInternalId": 1,
        "msgRecordId": 100,
        "type": 1,
        "title": "系统通知",
        "content": "欢迎使用 NetyAdmin！",
        "isRead": false,
        "createdAt": "2025-01-15T10:00:00Z"
      },
      {
        "msgInternalId": 2,
        "msgRecordId": 101,
        "type": 2,
        "title": "密码修改提醒",
        "content": "您的密码已成功修改，如非本人操作请及时联系管理员。",
        "isRead": true,
        "readAt": "2025-01-15T11:00:00Z",
        "createdAt": "2025-01-14T08:00:00Z"
      }
    ],
    "current": 1,
    "size": 20,
    "total": 2
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| msgInternalId | uint64 | 站内消息 ID（用户维度的消息记录 ID） |
| msgRecordId | uint64 | 消息记录 ID（全局消息发送记录 ID） |
| type | int | 消息类型 |
| title | string | 消息标题 |
| content | string | 消息内容 |
| isRead | boolean | 是否已读 |
| readAt | string | 已读时间（ISO 8601），未读时不返回 |
| createdAt | string | 创建时间（ISO 8601） |

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败 |
| `100002` | 未授权 |

---

## 三、站内消息详情

根据消息 ID 获取站内消息详情。

```
GET /client/v1/message/internal/:id
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**（Path）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | uint64 | 是 | 站内消息 ID（msgInternalId） |

**请求示例**：

```
GET /client/v1/message/internal/1
```

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "msgInternalId": 1,
    "msgRecordId": 100,
    "type": 1,
    "title": "系统通知",
    "content": "欢迎使用 NetyAdmin！",
    "isRead": false,
    "createdAt": "2025-01-15T10:00:00Z"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| msgInternalId | uint64 | 站内消息 ID |
| msgRecordId | uint64 | 消息记录 ID |
| type | int | 消息类型 |
| title | string | 消息标题 |
| content | string | 消息内容 |
| isRead | boolean | 是否已读 |
| readAt | string | 已读时间（ISO 8601），未读时不返回 |
| createdAt | string | 创建时间（ISO 8601） |

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 无效的 ID |
| `100002` | 未授权 |
| `101204` | 消息记录不存在 |

---

## 四、标记消息已读

将指定站内消息标记为已读。

```
PUT /client/v1/message/internal/read
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**（JSON Body）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| msgInternalId | uint64 | 是 | 站内消息 ID |

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
  "msg": "",
  "data": null
}
```

**可能错误码**：

| code | 说明 |
|------|------|
| `100001` | 参数校验失败（msgInternalId 必填） |
| `100002` | 未授权 |
| `101204` | 消息记录不存在 |

---

## 五、全部标记已读

将当前用户所有未读的站内消息标记为已读。

```
PUT /client/v1/message/internal/read-all
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**：无

**请求 Body**：无（或空 JSON `{}`）

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": null
}
```

**可能错误码**：

| code | 说明 |
|------|------|
| `100002` | 未授权 |

---

## 六、未读消息数量

获取当前用户的站内消息未读数量。

```
GET /client/v1/message/internal/unread-count
```

**权限**：开放平台签名 + 用户 JWT

**请求参数**：无

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "unreadCount": 5
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| unreadCount | int64 | 未读消息数量 |

**可能错误码**：

| code | 说明 |
|------|------|
| `100002` | 未授权 |
