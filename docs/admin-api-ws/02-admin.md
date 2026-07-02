# 管理员管理 API

本模块提供管理员账号的列表查询、创建、详情获取、更新、删除及批量删除功能。

## 接口概览

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/admins` | 权限 | 获取管理员列表 |
| POST | `/admin/v1/admins` | 权限 | 创建管理员 |
| GET | `/admin/v1/admins/:id` | 权限 | 获取管理员详情 |
| PUT | `/admin/v1/admins/:id` | 权限 | 更新管理员 |
| DELETE | `/admin/v1/admins/:id` | 权限 | 删除管理员 |
| DELETE | `/admin/v1/admins/batch` | 权限 | 批量删除管理员 |

---

## 1. 获取管理员列表

分页查询管理员列表，支持按用户名、昵称、手机号、邮箱、状态、性别筛选。

```
GET /admin/v1/admins
```

### 认证级别

权限接口（JWT + RBAC）

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 当前页码，默认 1 |
| `size` | int | 否 | 每页数量，默认为系统默认值 |
| `username` | string | 否 | 用户名（模糊查询） |
| `nickname` | string | 否 | 昵称（模糊查询） |
| `phone` | string | 否 | 手机号 |
| `email` | string | 否 | 邮箱 |
| `status` | string | 否 | 状态（`0`:禁用 `1`:启用） |
| `gender` | string | 否 | 性别（`0`:未知 `1`:男 `2`:女） |

### 请求示例

```
GET /admin/v1/admins?current=1&size=10&username=admin&status=1
Authorization: Bearer <token>
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "records": [
      {
        "id": 1,
        "username": "admin",
        "nickname": "超级管理员",
        "phone": "13800138000",
        "email": "admin@example.com",
        "gender": "1",
        "status": "1",
        "roles": ["super_admin"]
      }
    ],
    "current": 1,
    "size": 10,
    "total": 1
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

---

## 2. 创建管理员

创建新的管理员账号，设置用户名、密码、昵称、联系方式、状态及角色。

```
POST /admin/v1/admins
```

### 认证级别

权限接口（JWT + RBAC）

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | string | 是 | 用户名（3-50 字符） |
| `password` | string | 是 | 密码（8-32 字符） |
| `nickname` | string | 否 | 昵称 |
| `phone` | string | 否 | 手机号 |
| `email` | string | 否 | 邮箱（需符合邮箱格式，最大 100 字符） |
| `gender` | string | 否 | 性别（`0`:未知 `1`:男 `2`:女） |
| `status` | string | 是 | 状态（`0`:禁用 `1`:启用） |
| `roles` | []string | 否 | 角色编码列表 |

### 请求示例

```json
{
  "username": "newadmin",
  "password": "password123",
  "nickname": "新管理员",
  "phone": "13900139000",
  "email": "new@example.com",
  "gender": "1",
  "status": "1",
  "roles": ["editor"]
}
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "管理员创建成功",
  "data": {
    "id": 10
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `100001` | 参数错误 |
| `101004` | 用户名已存在 |

---

## 3. 获取管理员详情

根据管理员 ID 获取管理员详细信息。

```
GET /admin/v1/admins/:id
```

### 认证级别

权限接口（JWT + RBAC）

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | uint | 是 | 管理员 ID |

### 请求示例

```
GET /admin/v1/admins/1
Authorization: Bearer <token>
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "id": 1,
    "username": "admin",
    "nickname": "超级管理员",
    "phone": "13800138000",
    "email": "admin@example.com",
    "gender": "1",
    "status": "1",
    "roles": ["super_admin"]
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

---

## 4. 更新管理员

根据管理员 ID 更新管理员账号信息，包括资料、密码及角色。

```
PUT /admin/v1/admins/:id
```

### 认证级别

权限接口（JWT + RBAC）

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | uint | 是 | 管理员 ID |

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | string | 是 | 用户名（3-50 字符） |
| `password` | string | 否 | 新密码（8-32 字符，留空则不修改） |
| `nickname` | string | 否 | 昵称 |
| `phone` | string | 否 | 手机号 |
| `email` | string | 否 | 邮箱 |
| `gender` | string | 否 | 性别（`0`:未知 `1`:男 `2`:女） |
| `status` | string | 是 | 状态（`0`:禁用 `1`:启用） |
| `roles` | []string | 否 | 角色编码列表 |

### 请求示例

```json
{
  "username": "admin",
  "nickname": "更新昵称",
  "phone": "13800138001",
  "email": "updated@example.com",
  "gender": "2",
  "status": "1",
  "roles": ["super_admin", "editor"]
}
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "管理员更新成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

---

## 5. 删除管理员

根据管理员 ID 删除单个管理员账号。

```
DELETE /admin/v1/admins/:id
```

### 认证级别

权限接口（JWT + RBAC）

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | uint | 是 | 管理员 ID |

### 自删除防护规则

系统禁止管理员删除自己的账号。如果请求中的 `id` 等于当前登录管理员的 `adminID`，将返回错误码 `100007`（请求错误），提示信息为"不能删除自己的账号"。

### 请求示例

```
DELETE /admin/v1/admins/10
Authorization: Bearer <token>
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "管理员删除成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `100001` | 参数错误（无效的管理员 ID） |
| `100007` | 请求错误（不能删除自己的账号） |

---

## 6. 批量删除管理员

根据管理员 ID 列表批量删除管理员账号。

```
DELETE /admin/v1/admins/batch
```

### 认证级别

权限接口（JWT + RBAC）

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ids` | []uint | 是 | 待删除的管理员 ID 列表 |

### 请求示例

```json
{
  "ids": [10, 11, 12]
}
```

### 自删除防护规则

批量删除时同样会校验每个 ID，如果列表中包含当前登录管理员的 ID，将返回错误码 `100007`（请求错误），提示信息为"不能删除自己的账号"。

### 响应示例

```json
{
  "code": "100000",
  "msg": "管理员批量删除成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `100001` | 参数错误（ids 为空或格式不正确） |
| `100007` | 请求错误（不能删除自己的账号） |
