# 通用 API

本模块提供验证码获取、用户路由获取及路由存在性检查等通用功能。

## 接口概览

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/common/captcha` | 公开 | 获取图形验证码 |
| GET | `/admin/v1/route/getUserRoutes` | 认证 | 获取当前用户路由 |
| GET | `/admin/v1/route/isRouteExist` | 认证 | 检查路由是否存在 |

---

## 1. 获取图形验证码

生成图形验证码，返回验证码 ID 和 Base64 编码的验证码图片。用于管理员登录等需要人机验证的场景。

```
GET /admin/v1/common/captcha
```

### 认证级别

公开接口（无需 Token）

### 请求示例

```
GET /admin/v1/common/captcha
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "captchaId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "captchaImg": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA..."
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `captchaId` | string | 验证码唯一 ID，登录时需提交 |
| `captchaImg` | string | Base64 编码的验证码图片（Data URI 格式） |

### 使用流程

1. 调用此接口获取 `captchaId` 和 `captchaImg`
2. 前端展示 `captchaImg` 图片，用户输入验证码
3. 登录时将 `captchaId` 和 `captchaValue` 一起提交到登录接口
4. 验证码校验通过后自动销毁，不可重复使用

---

## 2. 获取当前用户路由

获取当前登录管理员可见的动态路由列表，用于前端动态菜单和路由生成。

```
GET /admin/v1/route/getUserRoutes
```

### 认证级别

认证接口（需 Bearer Token，无需 RBAC）

### 请求示例

```
GET /admin/v1/route/getUserRoutes
Authorization: Bearer <token>
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "home": "home",
    "routes": [
      {
        "name": "home",
        "path": "/home",
        "component": "layout.base",
        "meta": {
          "title": "首页",
          "i18nKey": "route.home",
          "icon": "mdi:monitor",
          "order": 1,
          "hideInMenu": false,
          "keepAlive": true
        }
      },
      {
        "name": "system",
        "path": "/system",
        "component": "layout.base",
        "meta": {
          "title": "系统管理",
          "icon": "carbon:cloud-service-management",
          "order": 2
        },
        "children": [
          {
            "name": "system_admin",
            "path": "/system/admin",
            "component": "view.system_admin",
            "meta": {
              "title": "管理员管理",
              "icon": "mdi:account-group"
            }
          }
        ]
      }
    ]
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `home` | string | 首页路由名称 |
| `routes` | array | 路由列表（树形结构） |
| `routes[].name` | string | 路由名称 |
| `routes[].path` | string | 路由路径 |
| `routes[].component` | string | 组件路径 |
| `routes[].meta` | object | 路由元信息 |
| `routes[].meta.title` | string | 菜单标题 |
| `routes[].meta.i18nKey` | string | 国际化 Key |
| `routes[].meta.icon` | string | 图标 |
| `routes[].meta.order` | int | 排序 |
| `routes[].meta.hideInMenu` | bool | 是否在菜单中隐藏 |
| `routes[].meta.keepAlive` | bool | 是否缓存 |
| `routes[].children` | array | 子路由列表 |

> 路由列表基于当前管理员的角色权限动态生成，仅返回有权限访问的菜单。

---

## 3. 检查路由是否存在

检查指定路由名称是否存在。用于前端路由守卫，验证目标路由是否在当前用户的可访问路由中。

```
GET /admin/v1/route/isRouteExist
```

### 认证级别

认证接口（需 Bearer Token，无需 RBAC）

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `routeName` | string | 是 | 路由名称 |

### 请求示例

```
GET /admin/v1/route/isRouteExist?routeName=system_admin
Authorization: Bearer <token>
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "isExist": true
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `isExist` | bool | 路由是否存在（且当前用户有权限访问） |

> 此接口仅检查当前用户有权限访问的路由。如果路由存在但当前用户无权限，返回 `false`。
