# 运维管理 API

本模块涵盖操作日志、错误日志、IP 访问控制及开放平台调用日志的管理功能。

## 接口概览

### 操作日志

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/operation-logs` | 获取操作日志列表 |
| DELETE | `/admin/v1/operation-logs/:id` | 删除操作日志 |
| POST | `/admin/v1/operation-logs/batch-delete` | 批量删除操作日志 |

### 错误日志

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/error-logs` | 获取错误日志列表 |
| PUT | `/admin/v1/error-logs/:id/resolve` | 标记错误日志已解决 |
| DELETE | `/admin/v1/error-logs/:id` | 删除错误日志 |
| POST | `/admin/v1/error-logs/batch-delete` | 批量删除错误日志 |

### IP 访问控制

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/open-platform/ip-access` | 获取 IP 访问规则列表 |
| POST | `/admin/v1/open-platform/ip-access` | 新增 IP 访问规则 |
| PUT | `/admin/v1/open-platform/ip-access` | 修改 IP 访问规则 |
| DELETE | `/admin/v1/open-platform/ip-access/:id` | 删除 IP 访问规则 |
| DELETE | `/admin/v1/open-platform/ip-access/batch` | 批量删除 IP 访问规则 |

### 开放平台日志

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/ops/open-platform-log` | 获取开放平台调用日志列表 |

---

## 一、操作日志

### 1.1 获取操作日志列表

分页获取操作日志，支持按管理员、操作类型、时间范围筛选。

```
GET /admin/v1/operation-logs
```

#### 认证级别

权限接口（JWT + RBAC）

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 页码，默认 1，最小 1 |
| `size` | int | 否 | 每页数量，默认 10，最大 100 |
| `adminId` | uint | 否 | 管理员 ID |
| `action` | string | 否 | 操作类型 |
| `startDate` | string | 否 | 开始日期 |
| `endDate` | string | 否 | 结束日期 |

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": [
    {
      "id": 1,
      "adminId": 1,
      "adminName": "admin",
      "action": "CREATE_ADMIN",
      "method": "POST",
      "path": "/admin/v1/admins",
      "ip": "192.168.1.1",
      "userAgent": "Mozilla/5.0",
      "statusCode": 200,
      "latency": "12ms",
      "createdAt": "2025-01-01T12:00:00Z"
    }
  ],
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 1.2 删除操作日志

```
DELETE /admin/v1/operation-logs/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 日志 ID |

### 1.3 批量删除操作日志

```
POST /admin/v1/operation-logs/batch-delete
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ids` | []uint | 是 | 日志 ID 列表 |

#### 请求示例

```json
{
  "ids": [1, 2, 3]
}
```

---

## 二、错误日志

### 2.1 获取错误日志列表

分页获取错误日志，支持按日志级别、是否已解决筛选。

```
GET /admin/v1/error-logs
```

#### 认证级别

权限接口（JWT + RBAC）

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 页码，默认 1 |
| `size` | int | 否 | 每页数量，默认 10，最大 100 |
| `level` | string | 否 | 日志级别 |
| `resolved` | string | 否 | 是否已解决（`true`/`false`） |

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "records": [
      {
        "id": 1,
        "level": "ERROR",
        "message": "database connection failed",
        "stack": "goroutine 1 [running]:...",
        "resolved": false,
        "resolvedBy": null,
        "resolvedAt": null,
        "createdAt": "2025-01-01T12:00:00Z"
      }
    ],
    "current": 1,
    "size": 10,
    "total": 1
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 2.2 标记错误日志已解决

根据 ID 将错误日志标记为已解决。

```
PUT /admin/v1/error-logs/:id/resolve
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 日志 ID |

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 2.3 删除错误日志

```
DELETE /admin/v1/error-logs/:id
```

### 2.4 批量删除错误日志

```
POST /admin/v1/error-logs/batch-delete
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ids` | []uint | 是 | 日志 ID 列表 |

---

## 三、IP 访问控制

### 3.1 获取 IP 访问规则列表

分页获取 IP 访问控制规则列表，支持按应用、IP、类型、状态筛选。

```
GET /admin/v1/open-platform/ip-access
```

#### 认证级别

权限接口（JWT + RBAC）

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 页码 |
| `size` | int | 否 | 每页数量 |
| `appId` | string | 否 | 应用 ID |
| `ipAddr` | string | 否 | IP 地址 |
| `type` | int | 否 | 类型（`1`:黑名单 `2`:白名单） |
| `status` | int | 否 | 状态（`0`:禁用 `1`:启用） |

### 3.2 新增 IP 访问规则

```
POST /admin/v1/open-platform/ip-access
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `appId` | string | 否 | 关联应用 ID（为空则为全局规则） |
| `ipAddr` | string | 是 | IP 地址或 CIDR |
| `type` | int | 是 | 类型（`1`:黑名单 `2`:白名单） |
| `reason` | string | 否 | 原因说明 |
| `expiredAt` | *string | 否 | 过期时间（格式：`2006-01-02 15:04:05`） |
| `status` | int | 否 | 状态（`0`:禁用 `1`:启用） |

#### 请求示例

```json
{
  "ipAddr": "192.168.1.100",
  "type": 1,
  "reason": "恶意请求",
  "status": 1
}
```

### 3.3 修改 IP 访问规则

```
PUT /admin/v1/open-platform/ip-access
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | uint | 是 | 规则 ID |
| `type` | int | 是 | 类型（`1`:黑名单 `2`:白名单） |
| `reason` | string | 否 | 原因说明 |
| `expiredAt` | *string | 否 | 过期时间 |
| `status` | int | 否 | 状态（`0`:禁用 `1`:启用） |

### 3.4 删除 IP 访问规则

```
DELETE /admin/v1/open-platform/ip-access/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 规则 ID |

### 3.5 批量删除 IP 访问规则

```
DELETE /admin/v1/open-platform/ip-access/batch
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ids` | []uint | 是 | 规则 ID 列表（最少 1 个） |

#### 请求示例

```json
{
  "ids": [1, 2, 3]
}
```

#### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `101402` | 非法 IP/CIDR 格式 |

---

## 四、开放平台调用日志

### 4.1 获取开放平台调用日志列表

分页获取开放平台调用日志，支持按应用、AppKey、API 路径、状态码、时间范围筛选。

```
GET /admin/v1/ops/open-platform-log
```

#### 认证级别

权限接口（JWT + RBAC）

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 页码 |
| `size` | int | 否 | 每页数量 |
| `appId` | string | 否 | 应用 ID |
| `appKey` | string | 否 | 应用 AppKey |
| `apiPath` | string | 否 | API 路径 |
| `statusCode` | int | 否 | HTTP 状态码 |
| `startTime` | string | 否 | 开始时间 |
| `endTime` | string | 否 | 结束时间 |

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "records": [
      {
        "id": 1,
        "appId": "app001",
        "appKey": "AK123456",
        "apiPath": "/open/v1/user/info",
        "method": "GET",
        "statusCode": 200,
        "latency": "25ms",
        "ip": "203.0.113.50",
        "requestBody": "",
        "responseBody": "",
        "createdAt": "2025-01-01T12:00:00Z"
      }
    ],
    "current": 1,
    "size": 10,
    "total": 1
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```
