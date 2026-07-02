# 系统 RBAC 管理 API

本模块涵盖角色管理、菜单管理、按钮管理、API 管理及角色权限分配，是 RBAC（基于角色的访问控制）的核心模块。所有接口均需要 JWT Token + RBAC 权限校验。

所有 RBAC 相关接口的基础路径为 `/admin/v1/systemManage`。

## 接口概览

### 角色管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/systemManage/getRoleList` | 获取角色列表 |
| GET | `/admin/v1/systemManage/getRole/:id` | 获取角色详情 |
| GET | `/admin/v1/systemManage/getAllRoles` | 获取全部角色 |
| POST | `/admin/v1/systemManage/addRole` | 创建角色 |
| PUT | `/admin/v1/systemManage/updateRole` | 更新角色 |
| DELETE | `/admin/v1/systemManage/deleteRole` | 删除角色 |
| DELETE | `/admin/v1/systemManage/deleteRoles` | 批量删除角色 |

### 菜单管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/systemManage/getMenuList` | 获取菜单列表 |
| GET | `/admin/v1/systemManage/getMenuTree` | 获取菜单树 |
| GET | `/admin/v1/systemManage/getButtonTree` | 获取菜单按钮树 |
| GET | `/admin/v1/systemManage/getApiTree` | 获取菜单 API 树 |
| GET | `/admin/v1/systemManage/getAllPages` | 获取全部页面 |
| GET | `/admin/v1/systemManage/getMenu/:id` | 获取菜单详情 |
| POST | `/admin/v1/systemManage/addMenu` | 创建菜单 |
| PUT | `/admin/v1/systemManage/updateMenu` | 更新菜单 |
| DELETE | `/admin/v1/systemManage/deleteMenu` | 删除菜单 |
| DELETE | `/admin/v1/systemManage/deleteMenus` | 批量删除菜单 |

### 按钮管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/systemManage/getButtonList` | 获取按钮列表 |
| GET | `/admin/v1/systemManage/getAllButton` | 获取全部按钮 |
| GET | `/admin/v1/systemManage/getButton/:id` | 获取按钮详情 |
| POST | `/admin/v1/systemManage/createButton` | 创建按钮 |
| PUT | `/admin/v1/systemManage/updateButton` | 更新按钮 |
| DELETE | `/admin/v1/systemManage/deleteButton` | 删除按钮 |

### API 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/systemManage/getApiList` | 获取 API 列表 |
| GET | `/admin/v1/systemManage/getAllApi` | 获取全部 API |
| GET | `/admin/v1/systemManage/getApi/:id` | 获取 API 详情 |
| POST | `/admin/v1/systemManage/createApi` | 创建 API |
| PUT | `/admin/v1/systemManage/updateApi` | 更新 API |
| DELETE | `/admin/v1/systemManage/deleteApi/:id` | 删除 API |

### 角色权限分配

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/systemManage/role/:id/menus` | 获取角色菜单权限 |
| PUT | `/admin/v1/systemManage/role/:id/menus` | 更新角色菜单权限 |
| GET | `/admin/v1/systemManage/role/:id/buttons` | 获取角色按钮权限 |
| PUT | `/admin/v1/systemManage/role/:id/buttons` | 更新角色按钮权限 |
| GET | `/admin/v1/systemManage/role/:id/apis` | 获取角色 API 权限 |
| PUT | `/admin/v1/systemManage/role/:id/apis` | 更新角色 API 权限 |

---

## 一、角色管理

### 1.1 获取角色列表

分页查询角色列表，支持按角色名称、编码、状态筛选。

```
GET /admin/v1/systemManage/getRoleList
```

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 当前页码 |
| `size` | int | 否 | 每页数量 |
| `name` | string | 否 | 角色名称 |
| `code` | string | 否 | 角色编码 |
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
        "name": "超级管理员",
        "code": "super_admin",
        "desc": "系统超级管理员",
        "status": "1"
      }
    ],
    "current": 1,
    "size": 10,
    "total": 1
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 1.2 获取角色详情

```
GET /admin/v1/systemManage/getRole/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 角色 ID |

### 1.3 获取全部角色

获取所有角色列表，不分页。通常用于下拉选择。

```
GET /admin/v1/systemManage/getAllRoles
```

### 1.4 创建角色

创建新的角色，可同时绑定菜单、按钮及 API 权限。

```
POST /admin/v1/systemManage/addRole
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 角色名称 |
| `code` | string | 是 | 角色编码 |
| `desc` | string | 否 | 角色描述 |
| `status` | string | 是 | 状态（`0`:禁用 `1`:启用） |
| `menus` | []uint | 否 | 菜单 ID 列表 |
| `buttons` | []uint | 否 | 按钮 ID 列表 |
| `apis` | []uint | 否 | API ID 列表 |

#### 请求示例

```json
{
  "name": "编辑员",
  "code": "editor",
  "desc": "内容编辑员",
  "status": "1",
  "menus": [1, 2, 3],
  "buttons": [1, 2],
  "apis": [1, 2, 3]
}
```

#### 响应示例

```json
{
  "code": "100000",
  "msg": "角色创建成功",
  "data": {
    "id": 5
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 1.5 更新角色

更新角色信息，可同时调整菜单、按钮及 API 权限。

```
PUT /admin/v1/systemManage/updateRole
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | uint | 是 | 角色 ID |
| `name` | string | 是 | 角色名称 |
| `code` | string | 是 | 角色编码 |
| `desc` | string | 否 | 角色描述 |
| `status` | string | 是 | 状态（`0`:禁用 `1`:启用） |
| `menus` | []uint | 否 | 菜单 ID 列表 |
| `buttons` | []uint | 否 | 按钮 ID 列表 |
| `apis` | []uint | 否 | API ID 列表 |

### 1.6 删除角色

根据角色 ID 删除单个角色。

```
DELETE /admin/v1/systemManage/deleteRole
```

> 注意：该接口通过路径参数 `id` 传递角色 ID（如 `?id=5` 或路径中携带）。

### 1.7 批量删除角色

根据角色 ID 列表批量删除角色。

```
DELETE /admin/v1/systemManage/deleteRoles
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `roleIds` | []uint | 是 | 角色 ID 列表 |

#### 请求示例

```json
{
  "roleIds": [5, 6, 7]
}
```

#### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `102001` | 角色不存在 |
| `102002` | 角色正在使用中 |
| `102003` | 角色已存在 |
| `102004` | 角色编码重复 |
| `102005` | 不能删除超级管理员 |
| `102006` | 不能修改超级管理员 |

---

## 二、菜单管理

### 2.1 获取菜单列表

分页查询菜单列表，支持按名称、状态、父菜单 ID 筛选。

```
GET /admin/v1/systemManage/getMenuList
```

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 否 | 菜单名称 |
| `status` | string | 否 | 状态（`0`:禁用 `1`:启用） |
| `parentId` | uint | 否 | 父菜单 ID |
| `current` | int | 否 | 当前页码 |
| `size` | int | 否 | 每页数量 |

### 2.2 获取菜单树

获取完整的菜单树形结构。

```
GET /admin/v1/systemManage/getMenuTree
```

### 2.3 获取菜单按钮树

获取菜单与按钮关联的树形结构，用于角色按钮权限分配。

```
GET /admin/v1/systemManage/getButtonTree
```

### 2.4 获取菜单 API 树

获取菜单与 API 关联的树形结构，用于角色 API 权限分配。

```
GET /admin/v1/systemManage/getApiTree
```

### 2.5 获取全部页面

获取所有可作为菜单的页面列表。

```
GET /admin/v1/systemManage/getAllPages
```

### 2.6 获取菜单详情

```
GET /admin/v1/systemManage/getMenu/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 菜单 ID |

### 2.7 创建菜单

创建新的菜单，可设置路由、组件、图标、按钮等信息。

```
POST /admin/v1/systemManage/addMenu
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `parentId` | uint | 否 | 父菜单 ID |
| `type` | string | 是 | 菜单类型 |
| `name` | string | 是 | 菜单名称 |
| `routeName` | string | 是 | 路由名称 |
| `routePath` | string | 是 | 路由路径 |
| `component` | string | 否 | 组件路径 |
| `i18nKey` | string | 否 | 国际化 Key |
| `icon` | string | 否 | 图标 |
| `iconType` | string | 否 | 图标类型 |
| `order` | int | 否 | 排序 |
| `status` | string | 是 | 状态（`0`:禁用 `1`:启用） |
| `hideInMenu` | bool | 否 | 是否在菜单中隐藏 |
| `keepAlive` | bool | 否 | 是否缓存 |
| `constant` | bool | 否 | 是否为常量路由 |
| `activeMenu` | string | 否 | 高亮菜单名称 |
| `multiTab` | bool | 否 | 是否支持多标签页 |
| `fixedIndexInTab` | *int | 否 | 固定标签页索引 |
| `query` | []object | 否 | 路由参数 |
| `href` | string | 否 | 外链地址 |
| `buttons` | []object | 否 | 按钮列表 |

#### 请求示例

```json
{
  "parentId": 0,
  "type": "1",
  "name": "系统管理",
  "routeName": "system",
  "routePath": "/system",
  "component": "layout.base",
  "icon": "carbon:cloud-service-management",
  "iconType": "1",
  "order": 1,
  "status": "1",
  "hideInMenu": false,
  "keepAlive": true
}
```

### 2.8 更新菜单

更新菜单信息，可调整路由、组件、图标、按钮等。

```
PUT /admin/v1/systemManage/updateMenu
```

#### 请求参数

在创建菜单参数基础上增加必填字段 `id`：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | uint | 是 | 菜单 ID |

### 2.9 删除菜单

根据菜单 ID 删除单个菜单。

```
DELETE /admin/v1/systemManage/deleteMenu
```

### 2.10 批量删除菜单

根据菜单 ID 列表批量删除菜单。

```
DELETE /admin/v1/systemManage/deleteMenus
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `menuIds` | []uint | 是 | 菜单 ID 列表 |

#### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `103001` | 菜单不存在 |
| `103002` | 菜单存在子菜单 |
| `103003` | 菜单已存在 |
| `103004` | 菜单路由重复 |

---

## 三、按钮管理

### 3.1 获取按钮列表

分页查询按钮列表，支持按标签、编码、菜单 ID 筛选。

```
GET /admin/v1/systemManage/getButtonList
```

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `label` | string | 否 | 按钮标签 |
| `code` | string | 否 | 按钮编码 |
| `menuId` | uint | 否 | 菜单 ID |
| `current` | int | 否 | 当前页码 |
| `size` | int | 否 | 每页数量 |

### 3.2 获取全部按钮

获取所有按钮列表，不分页。

```
GET /admin/v1/systemManage/getAllButton
```

### 3.3 获取按钮详情

```
GET /admin/v1/systemManage/getButton/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 按钮 ID |

### 3.4 创建按钮

创建新的按钮，绑定到指定菜单。

```
POST /admin/v1/systemManage/createButton
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `menuId` | uint | 是 | 所属菜单 ID |
| `name` | string | 是 | 按钮名称 |
| `code` | string | 是 | 按钮编码 |

#### 请求示例

```json
{
  "menuId": 1,
  "name": "新增",
  "code": "btn:add"
}
```

### 3.5 更新按钮

```
PUT /admin/v1/systemManage/updateButton
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | uint | 是 | 按钮 ID |
| `menuId` | uint | 是 | 所属菜单 ID |
| `name` | string | 是 | 按钮名称 |
| `code` | string | 是 | 按钮编码 |

### 3.6 删除按钮

```
DELETE /admin/v1/systemManage/deleteButton
```

#### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `104001` | 按钮不存在 |
| `104002` | 按钮已存在 |
| `104003` | 按钮编码重复 |

---

## 四、API 管理

### 4.1 获取 API 列表

分页查询 API 列表，支持按名称、路径、请求方法、分组筛选。

```
GET /admin/v1/systemManage/getApiList
```

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 否 | API 名称 |
| `path` | string | 否 | API 路径 |
| `method` | string | 否 | 请求方法 |
| `group` | string | 否 | API 分组 |
| `current` | int | 否 | 当前页码 |
| `size` | int | 否 | 每页数量 |

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "records": [
      {
        "id": 1,
        "menuId": 1,
        "menuName": "系统管理",
        "name": "获取管理员列表",
        "method": "GET",
        "path": "/admin/v1/admins",
        "desc": "分页查询管理员列表",
        "auth": "admin:list",
        "createTime": "2025-01-01 12:00:00",
        "updateTime": "2025-01-01 12:00:00"
      }
    ],
    "current": 1,
    "size": 10,
    "total": 1
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 4.2 获取全部 API

获取所有 API 接口列表，不分页。

```
GET /admin/v1/systemManage/getAllApi
```

### 4.3 获取 API 详情

```
GET /admin/v1/systemManage/getApi/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | API ID |

### 4.4 创建 API

```
POST /admin/v1/systemManage/createApi
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | API 名称 |
| `path` | string | 是 | API 路径 |
| `method` | string | 是 | 请求方法 |
| `group` | string | 是 | API 分组 |
| `desc` | string | 否 | 描述 |

#### 请求示例

```json
{
  "name": "获取管理员列表",
  "path": "/admin/v1/admins",
  "method": "GET",
  "group": "admin",
  "desc": "分页查询管理员列表"
}
```

### 4.5 更新 API

```
PUT /admin/v1/systemManage/updateApi
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | uint | 是 | API ID |
| `name` | string | 是 | API 名称 |
| `path` | string | 是 | API 路径 |
| `method` | string | 是 | 请求方法 |
| `group` | string | 否 | API 分组 |
| `desc` | string | 否 | 描述 |

### 4.6 删除 API

```
DELETE /admin/v1/systemManage/deleteApi/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | API ID |

#### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `105001` | API 不存在 |
| `105002` | API 已存在 |
| `105003` | API 路径重复 |

---

## 五、角色权限分配

### 5.1 获取角色菜单权限

根据角色 ID 获取角色已分配的菜单权限及首页路由。

```
GET /admin/v1/systemManage/role/:id/menus
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 角色 ID |

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "menuIds": [1, 2, 3],
    "homeRouteName": "home"
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 5.2 更新角色菜单权限

根据角色 ID 更新角色的菜单权限列表及首页路由。

```
PUT /admin/v1/systemManage/role/:id/menus
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `menuIds` | []uint | 否 | 菜单 ID 列表 |
| `homeRouteName` | string | 否 | 首页路由名称 |

#### 请求示例

```json
{
  "menuIds": [1, 2, 3, 4],
  "homeRouteName": "dashboard"
}
```

### 5.3 获取角色按钮权限

根据角色 ID 获取角色已分配的按钮权限。

```
GET /admin/v1/systemManage/role/:id/buttons
```

### 5.4 更新角色按钮权限

根据角色 ID 更新角色的按钮权限列表。请求体为按钮 ID 数组。

```
PUT /admin/v1/systemManage/role/:id/buttons
```

#### 请求示例

```json
[1, 2, 3, 4]
```

### 5.5 获取角色 API 权限

根据角色 ID 获取角色已分配的 API 权限。

```
GET /admin/v1/systemManage/role/:id/apis
```

### 5.6 更新角色 API 权限

根据角色 ID 更新角色的 API 权限列表。请求体为 API ID 数组。

```
PUT /admin/v1/systemManage/role/:id/apis
```

#### 请求示例

```json
[1, 2, 3, 4, 5]
```
