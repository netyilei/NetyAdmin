# 字典管理 API

本模块提供字典类型和字典数据的管理功能，以及供前端根据编码获取字典数据的公开接口。字典数据通常用于下拉框选项、枚举展示等场景。

## 接口概览

### 字典类型管理（需权限）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/system/dict/types` | 获取字典类型列表 |
| POST | `/admin/v1/system/dict/types` | 创建字典类型 |
| PUT | `/admin/v1/system/dict/types` | 更新字典类型 |
| DELETE | `/admin/v1/system/dict/types/:id` | 删除字典类型 |

### 字典数据管理（需权限）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/system/dict/data` | 获取字典数据列表 |
| POST | `/admin/v1/system/dict/data` | 创建字典数据 |
| PUT | `/admin/v1/system/dict/data` | 更新字典数据 |
| DELETE | `/admin/v1/system/dict/data/:id` | 删除字典数据 |

### 公开接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/system/dict/data/:code` | 根据编码获取字典数据（公开，带缓存） |

---

## 一、字典类型管理

### 1.1 获取字典类型列表

分页获取字典类型列表，支持按名称、编码、状态筛选。

```
GET /admin/v1/system/dict/types
```

#### 认证级别

权限接口（JWT + RBAC）

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 页码，默认 1 |
| `size` | int | 否 | 每页数量，默认 20 |
| `name` | string | 否 | 字典类型名称 |
| `code` | string | 否 | 字典类型编码 |
| `status` | string | 否 | 状态（`0`:禁用 `1`:启用） |

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "records": [
      {
        "id": 1,
        "name": "性别",
        "code": "gender",
        "status": "1",
        "description": "性别字典"
      }
    ],
    "current": 1,
    "size": 20,
    "total": 1
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 1.2 创建字典类型

```
POST /admin/v1/system/dict/types
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 字典类型名称 |
| `code` | string | 是 | 字典类型编码 |
| `status` | string | 是 | 状态（`0`:禁用 `1`:启用） |
| `description` | string | 否 | 描述 |

#### 请求示例

```json
{
  "name": "性别",
  "code": "gender",
  "status": "1",
  "description": "性别字典"
}
```

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 1.3 更新字典类型

```
PUT /admin/v1/system/dict/types
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | uint | 是 | 字典类型 ID |
| `name` | string | 是 | 字典类型名称 |
| `code` | string | 是 | 字典类型编码 |
| `status` | string | 是 | 状态（`0`:禁用 `1`:启用） |
| `description` | string | 否 | 描述 |

### 1.4 删除字典类型

```
DELETE /admin/v1/system/dict/types/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 字典类型 ID |

---

## 二、字典数据管理

### 2.1 获取字典数据列表

分页获取字典数据全量管理列表，支持按字典编码、标签、状态筛选。

```
GET /admin/v1/system/dict/data
```

#### 认证级别

权限接口（JWT + RBAC）

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 页码，默认 1 |
| `size` | int | 否 | 每页数量，默认 20 |
| `dictCode` | string | 否 | 字典编码 |
| `label` | string | 否 | 字典标签 |
| `status` | string | 否 | 状态（`0`:禁用 `1`:启用） |

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "records": [
      {
        "id": 1,
        "dictCode": "gender",
        "label": "男",
        "value": "1",
        "tagType": "primary",
        "orderBy": 1,
        "status": "1",
        "remark": ""
      },
      {
        "id": 2,
        "dictCode": "gender",
        "label": "女",
        "value": "2",
        "tagType": "danger",
        "orderBy": 2,
        "status": "1",
        "remark": ""
      }
    ],
    "current": 1,
    "size": 20,
    "total": 2
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 2.2 创建字典数据

```
POST /admin/v1/system/dict/data
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `dictCode` | string | 是 | 字典编码 |
| `label` | string | 是 | 字典标签 |
| `value` | string | 是 | 字典值 |
| `tagType` | string | 否 | 标签类型 |
| `orderBy` | int | 否 | 排序 |
| `status` | string | 是 | 状态（`0`:禁用 `1`:启用） |
| `remark` | string | 否 | 备注 |

#### 请求示例

```json
{
  "dictCode": "gender",
  "label": "未知",
  "value": "0",
  "tagType": "info",
  "orderBy": 0,
  "status": "1",
  "remark": "未知性别"
}
```

### 2.3 更新字典数据

```
PUT /admin/v1/system/dict/data
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | uint | 是 | 字典数据 ID |
| `dictCode` | string | 是 | 字典编码 |
| `label` | string | 是 | 字典标签 |
| `value` | string | 是 | 字典值 |
| `tagType` | string | 否 | 标签类型 |
| `orderBy` | int | 否 | 排序 |
| `status` | string | 是 | 状态（`0`:禁用 `1`:启用） |
| `remark` | string | 否 | 备注 |

### 2.4 删除字典数据

```
DELETE /admin/v1/system/dict/data/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 字典数据 ID |

---

## 三、公开接口

### 3.1 根据编码获取字典数据

根据字典编码获取字典数据，用于前端下拉框、枚举展示等场景。该接口为公开接口，带缓存。

```
GET /admin/v1/system/dict/data/:code
```

#### 认证级别

公开接口（无需 Token）

#### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `code` | string | 是 | 字典编码 |

#### 请求示例

```
GET /admin/v1/system/dict/data/gender
```

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": [
    {
      "id": 1,
      "dictCode": "gender",
      "label": "男",
      "value": "1",
      "tagType": "primary",
      "orderBy": 1,
      "status": "1",
      "remark": ""
    },
    {
      "id": 2,
      "dictCode": "gender",
      "label": "女",
      "value": "2",
      "tagType": "danger",
      "orderBy": 2,
      "status": "1",
      "remark": ""
    }
  ],
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```
