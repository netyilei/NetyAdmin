# 消息管理 API

本模块涵盖消息模板管理、消息发送记录管理及直接发送消息功能。支持多种消息渠道（如邮件、短信、站内信等）。

## 接口概览

### 模板管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/message/templates` | 获取消息模板列表 |
| POST | `/admin/v1/message/templates` | 创建消息模板 |
| PUT | `/admin/v1/message/templates` | 更新消息模板 |
| DELETE | `/admin/v1/message/templates/:id` | 删除消息模板 |

### 记录与发送

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/message/records` | 获取消息记录列表 |
| POST | `/admin/v1/message/records/:id/retry` | 重发消息 |
| POST | `/admin/v1/message/send` | 直接发送消息 |

---

## 一、模板管理

### 1.1 获取消息模板列表

分页获取消息模板列表，支持按渠道、编码、名称、状态筛选。

```
GET /admin/v1/message/templates
```

#### 认证级别

权限接口（JWT + RBAC）

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 页码 |
| `size` | int | 否 | 每页数量 |
| `channel` | string | 否 | 消息渠道（如 `email`/`sms`/`inner`） |
| `code` | string | 否 | 模板编码 |
| `name` | string | 否 | 模板名称 |
| `status` | int | 否 | 状态（`0`:禁用 `1`:启用） |

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "records": [
      {
        "id": 1,
        "channel": "email",
        "code": "welcome_email",
        "name": "欢迎邮件",
        "title": "欢迎注册 NetyAdmin",
        "content": "亲爱的用户，欢迎注册！",
        "status": 1,
        "createdAt": "2025-01-01T12:00:00Z",
        "updatedAt": "2025-01-01T12:00:00Z"
      }
    ],
    "current": 1,
    "size": 10,
    "total": 1
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 1.2 创建消息模板

```
POST /admin/v1/message/templates
```

#### 请求参数

请求体为消息模板实体对象，主要字段：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `channel` | string | 是 | 消息渠道 |
| `code` | string | 是 | 模板编码（唯一） |
| `name` | string | 是 | 模板名称 |
| `title` | string | 否 | 消息标题 |
| `content` | string | 否 | 模板内容（支持变量占位符） |
| `status` | int | 否 | 状态（`0`:禁用 `1`:启用） |

#### 请求示例

```json
{
  "channel": "email",
  "code": "welcome_email",
  "name": "欢迎邮件",
  "title": "欢迎注册 NetyAdmin",
  "content": "亲爱的 {{nickname}}，欢迎注册！",
  "status": 1
}
```

### 1.3 更新消息模板

```
PUT /admin/v1/message/templates
```

请求体同创建模板，需包含模板 `id` 字段。

### 1.4 删除消息模板

```
DELETE /admin/v1/message/templates/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 模板 ID |

#### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `101201` | 消息模板不存在 |
| `101202` | 模板编码已存在 |

---

## 二、记录与发送

### 2.1 获取消息记录列表

分页获取消息发送记录，支持按渠道、接收人、状态筛选。

```
GET /admin/v1/message/records
```

#### 认证级别

权限接口（JWT + RBAC）

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 页码 |
| `size` | int | 否 | 每页数量 |
| `channel` | string | 否 | 消息渠道 |
| `receiver` | string | 否 | 接收人 |
| `status` | int | 否 | 发送状态 |

### 2.2 重发消息

根据记录 ID 重发失败的消息。

```
POST /admin/v1/message/records/:id/retry
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 记录 ID |

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

#### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `101204` | 消息记录不存在 |

### 2.3 直接发送消息

管理员直接发送消息到指定接收人。

```
POST /admin/v1/message/send
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `channel` | string | 是 | 消息渠道 |
| `receiver` | string | 是 | 接收人（邮箱/手机号/用户ID） |
| `title` | string | 否 | 消息标题 |
| `content` | string | 是 | 消息内容 |

#### 请求示例

```json
{
  "channel": "email",
  "receiver": "user@example.com",
  "title": "系统通知",
  "content": "您的账户审核已通过。"
}
```

#### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `101203` | 消息发送失败 |
| `101205` | 消息驱动未配置或不存在 |
