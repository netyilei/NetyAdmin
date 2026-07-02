# 开放平台管理 API

本模块涵盖开放平台应用管理、权限范围（Scope）管理、开放 API 管理及权限范围与 API 的关联管理。

## 接口概览

### 应用管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/open/apps` | 获取应用列表 |
| POST | `/admin/v1/open/apps` | 创建应用 |
| PUT | `/admin/v1/open/apps` | 更新应用 |
| DELETE | `/admin/v1/open/apps/:id` | 删除应用 |
| PUT | `/admin/v1/open/apps/reset-secret` | 重置密钥 |
| PUT | `/admin/v1/open/apps/ip-rules` | 关联 IP 规则 |
| GET | `/admin/v1/open/apps/scopes` | 获取应用权限范围 |
| GET | `/admin/v1/open/apps/available-scopes` | 获取可用权限范围 |

### 权限范围管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/open/scopes` | 获取权限分组列表 |
| POST | `/admin/v1/open/scopes` | 创建权限分组 |
| PUT | `/admin/v1/open/scopes` | 更新权限分组 |
| DELETE | `/admin/v1/open/scopes/:id` | 删除权限分组 |

### 开放 API 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/open/apis` | 获取开放 API 列表 |
| POST | `/admin/v1/open/apis` | 创建开放 API |
| PUT | `/admin/v1/open/apis` | 更新开放 API |
| DELETE | `/admin/v1/open/apis/:id` | 删除开放 API |
| GET | `/admin/v1/open/apis/grouped` | 获取分组 API 列表 |
| GET | `/admin/v1/open/apis/scope-apis` | 获取权限范围关联的 API |
| PUT | `/admin/v1/open/apis/scope-apis` | 更新权限范围关联的 API |

---

## 一、应用管理

### 1.1 获取应用列表

分页获取开放平台应用列表，支持按名称、AppKey、状态筛选。

```
GET /admin/v1/open/apps
```

#### 认证级别

权限接口（JWT + RBAC）

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 页码 |
| `size` | int | 否 | 每页数量 |
| `name` | string | 否 | 应用名称 |
| `appKey` | string | 否 | 应用 AppKey |
| `status` | int | 否 | 状态（`0`:禁用 `1`:启用） |

### 1.2 创建应用

创建开放平台应用，并关联权限范围。

```
POST /admin/v1/open/apps
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 应用名称 |
| `status` | int | 是 | 状态（`0`:禁用 `1`:启用） |
| `ipFilterEnabled` | bool | 否 | 是否启用 IP 过滤 |
| `rateLimitEnabled` | bool | 否 | 是否启用限流 |
| `remark` | string | 否 | 备注 |
| `quotaConfig` | string | 否 | 配额配置（JSON 字符串，默认 `{}`） |
| `cacheTTL` | int | 否 | 缓存时间（秒） |
| `storageId` | uint | 否 | 关联存储配置 ID |
| `scopes` | []string | 否 | 权限范围编码列表 |

#### 请求示例

```json
{
  "name": "小程序客户端",
  "status": 1,
  "ipFilterEnabled": true,
  "rateLimitEnabled": true,
  "remark": "微信小程序",
  "quotaConfig": "{\"daily\": 10000}",
  "cacheTTL": 300,
  "scopes": ["user:read", "content:read"]
}
```

### 1.3 更新应用

更新开放平台应用信息及其权限范围。

```
PUT /admin/v1/open/apps
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | 应用 ID |
| `name` | string | 是 | 应用名称 |
| `status` | int | 是 | 状态（`0`:禁用 `1`:启用） |
| `ipFilterEnabled` | bool | 否 | 是否启用 IP 过滤 |
| `rateLimitEnabled` | bool | 否 | 是否启用限流 |
| `remark` | string | 否 | 备注 |
| `quotaConfig` | string | 否 | 配额配置 |
| `cacheTTL` | int | 否 | 缓存时间 |
| `storageId` | uint | 否 | 关联存储配置 ID |
| `scopes` | []string | 否 | 权限范围编码列表 |

### 1.4 删除应用

```
DELETE /admin/v1/open/apps/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | string | 应用 ID |

### 1.5 重置密钥

重置开放平台应用的 AppSecret，返回新的密钥。

```
PUT /admin/v1/open/apps/reset-secret
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | 应用 ID |

#### 请求示例

```json
{
  "id": "app001"
}
```

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "appSecret": "sk-new-secret-1234567890"
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 1.6 关联 IP 规则

将 IP 规则关联到指定应用。

```
PUT /admin/v1/open/apps/ip-rules
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | 应用 ID |
| `ruleIds` | []uint | 否 | IP 规则 ID 列表 |

#### 请求示例

```json
{
  "id": "app001",
  "ruleIds": [1, 2, 3]
}
```

### 1.7 获取应用权限范围

根据应用 ID 获取应用已授权的权限范围列表。

```
GET /admin/v1/open/apps/scopes?id=<应用ID>
```

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | 应用 ID |

### 1.8 获取可用权限范围

获取所有可用的权限范围列表。

```
GET /admin/v1/open/apps/available-scopes
```

#### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `100004` | 资源不存在（应用不存在） |
| `101301` | AppKey 无效 |
| `101306` | 应用已被禁用 |

---

## 二、权限范围管理

### 2.1 获取权限分组列表

获取所有权限分组列表。

```
GET /admin/v1/open/scopes
```

### 2.2 创建权限分组

```
POST /admin/v1/open/scopes
```

#### 请求参数

请求体为权限分组实体对象，包含名称、编码、描述等字段。

#### 请求示例

```json
{
  "name": "用户读取权限",
  "code": "user:read",
  "description": "允许读取用户基本信息"
}
```

### 2.3 更新权限分组

```
PUT /admin/v1/open/scopes
```

### 2.4 删除权限分组

```
DELETE /admin/v1/open/scopes/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 权限分组 ID |

---

## 三、开放 API 管理

### 3.1 获取开放 API 列表

分页获取开放平台 API 列表，支持按方法、路径、名称、分组、状态筛选。

```
GET /admin/v1/open/apis
```

#### 认证级别

权限接口（JWT + RBAC）

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 页码 |
| `size` | int | 否 | 每页数量 |
| `method` | string | 否 | 请求方法 |
| `path` | string | 否 | API 路径 |
| `name` | string | 否 | API 名称 |
| `group` | string | 否 | API 分组 |
| `status` | int | 否 | 状态（`0`:禁用 `1`:启用） |

### 3.2 创建开放 API

```
POST /admin/v1/open/apis
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `method` | string | 是 | 请求方法（`GET`/`POST`/`PUT`/`DELETE`/`PATCH`） |
| `path` | string | 是 | API 路径 |
| `name` | string | 是 | API 名称 |
| `group` | string | 否 | API 分组 |
| `description` | string | 否 | 描述 |
| `status` | int | 是 | 状态（`0`:禁用 `1`:启用） |

#### 请求示例

```json
{
  "method": "GET",
  "path": "/open/v1/user/info",
  "name": "获取用户信息",
  "group": "用户服务",
  "description": "获取当前登录用户的详细信息",
  "status": 1
}
```

### 3.3 更新开放 API

```
PUT /admin/v1/open/apis
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | uint64 | 是 | API ID |
| `method` | string | 是 | 请求方法 |
| `path` | string | 是 | API 路径 |
| `name` | string | 是 | API 名称 |
| `group` | string | 否 | API 分组 |
| `description` | string | 否 | 描述 |
| `status` | int | 是 | 状态 |

### 3.4 删除开放 API

```
DELETE /admin/v1/open/apis/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint64 | API ID |

### 3.5 获取分组 API 列表

按分组获取开放平台 API 列表。

```
GET /admin/v1/open/apis/grouped
```

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": [
    {
      "group": "用户服务",
      "apis": [
        {
          "id": 1,
          "name": "获取用户信息",
          "method": "GET",
          "path": "/open/v1/user/info"
        }
      ]
    },
    {
      "group": "内容服务",
      "apis": [
        {
          "id": 2,
          "name": "获取文章列表",
          "method": "GET",
          "path": "/open/v1/articles"
        }
      ]
    }
  ],
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 3.6 获取权限范围关联的 API

根据权限范围 ID 获取其关联的 API 列表。

```
GET /admin/v1/open/apis/scope-apis?scopeId=<权限范围ID>
```

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `scopeId` | uint64 | 是 | 权限范围 ID |

### 3.7 更新权限范围关联的 API

更新指定权限范围所关联的 API 列表。

```
PUT /admin/v1/open/apis/scope-apis
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `scopeId` | uint64 | 是 | 权限范围 ID |
| `apiIds` | []uint64 | 否 | API ID 列表 |

#### 请求示例

```json
{
  "scopeId": 1,
  "apiIds": [1, 2, 3, 4]
}
```

#### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `101304` | 权限不足（Scope 不匹配） |
| `101305` | 已触发流量限制 |
