# NetyAdmin GM Admin API 文档

NetyAdmin 管理后台 API 完整文档。基础路径为 `/admin/v1`，采用 JWT 认证机制，支持 RBAC 权限控制。

## 统一响应格式

```json
{
  "code": "100000",
  "msg": "",
  "data": {},
  "request_id": "uuid"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | string | 业务状态码，`100000` 表示成功 |
| `msg` | string | 提示信息，成功时为空 |
| `data` | any | 响应数据 |
| `request_id` | string | 请求唯一标识（UUID） |

## 认证说明

所有需要认证的接口须在请求头携带：

```
Authorization: Bearer <accessToken>
```

- AccessToken 有效期：168 小时（7 天）
- RefreshToken 有效期：336 小时（14 天）

详见 [认证指南](00-authentication.md)。

---

## 文档目录

| 文档 | 说明 |
|------|------|
| [00-authentication.md](00-authentication.md) | JWT 认证指南：登录流程、Token 格式、中间件分组、认证错误码 |
| [01-auth.md](01-auth.md) | 认证管理 API：登录、刷新令牌、用户信息、个人资料、修改密码、退出登录 |
| [02-admin.md](02-admin.md) | 管理员管理 API：列表、创建、详情、更新、删除、批量删除 |
| [03-system-rbac.md](03-system-rbac.md) | 系统 RBAC 管理 API：角色、菜单、按钮、API 管理及权限分配 |
| [04-content.md](04-content.md) | 内容管理 API：分类、文章、Banner 分组、Banner 项 |
| [05-dict.md](05-dict.md) | 字典管理 API：字典类型、字典数据、公开字典查询 |
| [06-storage.md](06-storage.md) | 存储管理 API：存储配置、上传记录、上传凭证、上传通知 |
| [07-ops.md](07-ops.md) | 运维管理 API：操作日志、错误日志、IP 访问控制、开放平台日志 |
| [08-open-platform.md](08-open-platform.md) | 开放平台 API：应用管理、权限范围、开放 API 管理 |
| [09-message.md](09-message.md) | 消息管理 API：模板管理、发送记录、直接发送 |
| [10-task.md](10-task.md) | 任务管理 API：列表、执行、启停、重载、更新、日志 |
| [12-config.md](12-config.md) | 系统配置 API：配置获取、配置更新、邮件测试 |
| [13-common.md](13-common.md) | 通用 API：验证码、用户路由、路由存在性检查 |
| [14-error-codes.md](14-error-codes.md) | 错误码参考：全部业务错误码列表 |

---

## 完整 API 端点总览

### 认证级别说明

- **公开**：无需 Token
- **认证**：需 Bearer Token，无需 RBAC
- **权限**：需 Bearer Token + RBAC 权限校验

---

### 认证管理（01-auth.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| POST | `/admin/v1/auth/login` | 公开 | 管理员登录 |
| POST | `/admin/v1/auth/refreshToken` | 公开 | 刷新令牌 |
| GET | `/admin/v1/auth/getUserInfo` | 认证 | 获取当前登录用户信息 |
| GET | `/admin/v1/auth/profile` | 认证 | 获取个人资料 |
| PUT | `/admin/v1/auth/profile` | 认证 | 更新个人资料 |
| POST | `/admin/v1/auth/changePassword` | 认证 | 修改密码 |
| POST | `/admin/v1/auth/logout` | 认证 | 退出登录 |

### 管理员管理（02-admin.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/admins` | 权限 | 获取管理员列表 |
| POST | `/admin/v1/admins` | 权限 | 创建管理员 |
| GET | `/admin/v1/admins/:id` | 权限 | 获取管理员详情 |
| PUT | `/admin/v1/admins/:id` | 权限 | 更新管理员 |
| DELETE | `/admin/v1/admins/:id` | 权限 | 删除管理员 |
| DELETE | `/admin/v1/admins/batch` | 权限 | 批量删除管理员 |

### 系统 RBAC - 角色管理（03-system-rbac.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/systemManage/getRoleList` | 权限 | 获取角色列表 |
| GET | `/admin/v1/systemManage/getRole/:id` | 权限 | 获取角色详情 |
| GET | `/admin/v1/systemManage/getAllRoles` | 权限 | 获取全部角色 |
| POST | `/admin/v1/systemManage/addRole` | 权限 | 创建角色 |
| PUT | `/admin/v1/systemManage/updateRole` | 权限 | 更新角色 |
| DELETE | `/admin/v1/systemManage/deleteRole` | 权限 | 删除角色 |
| DELETE | `/admin/v1/systemManage/deleteRoles` | 权限 | 批量删除角色 |

### 系统 RBAC - 菜单管理（03-system-rbac.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/systemManage/getMenuList` | 权限 | 获取菜单列表 |
| GET | `/admin/v1/systemManage/getMenuTree` | 权限 | 获取菜单树 |
| GET | `/admin/v1/systemManage/getButtonTree` | 权限 | 获取菜单按钮树 |
| GET | `/admin/v1/systemManage/getApiTree` | 权限 | 获取菜单 API 树 |
| GET | `/admin/v1/systemManage/getAllPages` | 权限 | 获取全部页面 |
| GET | `/admin/v1/systemManage/getMenu/:id` | 权限 | 获取菜单详情 |
| POST | `/admin/v1/systemManage/addMenu` | 权限 | 创建菜单 |
| PUT | `/admin/v1/systemManage/updateMenu` | 权限 | 更新菜单 |
| DELETE | `/admin/v1/systemManage/deleteMenu` | 权限 | 删除菜单 |
| DELETE | `/admin/v1/systemManage/deleteMenus` | 权限 | 批量删除菜单 |

### 系统 RBAC - 按钮管理（03-system-rbac.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/systemManage/getButtonList` | 权限 | 获取按钮列表 |
| GET | `/admin/v1/systemManage/getAllButton` | 权限 | 获取全部按钮 |
| GET | `/admin/v1/systemManage/getButton/:id` | 权限 | 获取按钮详情 |
| POST | `/admin/v1/systemManage/createButton` | 权限 | 创建按钮 |
| PUT | `/admin/v1/systemManage/updateButton` | 权限 | 更新按钮 |
| DELETE | `/admin/v1/systemManage/deleteButton` | 权限 | 删除按钮 |

### 系统 RBAC - API 管理（03-system-rbac.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/systemManage/getApiList` | 权限 | 获取 API 列表 |
| GET | `/admin/v1/systemManage/getAllApi` | 权限 | 获取全部 API |
| GET | `/admin/v1/systemManage/getApi/:id` | 权限 | 获取 API 详情 |
| POST | `/admin/v1/systemManage/createApi` | 权限 | 创建 API |
| PUT | `/admin/v1/systemManage/updateApi` | 权限 | 更新 API |
| DELETE | `/admin/v1/systemManage/deleteApi/:id` | 权限 | 删除 API |

### 系统 RBAC - 角色权限分配（03-system-rbac.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/systemManage/role/:id/menus` | 权限 | 获取角色菜单权限 |
| PUT | `/admin/v1/systemManage/role/:id/menus` | 权限 | 更新角色菜单权限 |
| GET | `/admin/v1/systemManage/role/:id/buttons` | 权限 | 获取角色按钮权限 |
| PUT | `/admin/v1/systemManage/role/:id/buttons` | 权限 | 更新角色按钮权限 |
| GET | `/admin/v1/systemManage/role/:id/apis` | 权限 | 获取角色 API 权限 |
| PUT | `/admin/v1/systemManage/role/:id/apis` | 权限 | 更新角色 API 权限 |

### 系统 RBAC - 用户管理（03-system-rbac.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/systemManage/users/autocomplete` | 权限 | 用户自动补全 |
| GET | `/admin/v1/systemManage/users` | 权限 | 获取用户列表 |
| POST | `/admin/v1/systemManage/users` | 权限 | 创建用户 |
| PUT | `/admin/v1/systemManage/users/:id` | 权限 | 更新用户 |
| PATCH | `/admin/v1/systemManage/users/:id/status` | 权限 | 更新用户状态 |
| POST | `/admin/v1/systemManage/users/:id/unlock` | 权限 | 解锁用户 |
| DELETE | `/admin/v1/systemManage/users/:id` | 权限 | 删除用户 |

### 内容管理 - 分类（04-content.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/content/categories` | 权限 | 获取分类列表 |
| GET | `/admin/v1/content/categories/tree` | 权限 | 获取分类树 |
| GET | `/admin/v1/content/categories/:id` | 权限 | 获取分类详情 |
| POST | `/admin/v1/content/categories` | 权限 | 创建分类 |
| PUT | `/admin/v1/content/categories/:id` | 权限 | 更新分类 |
| DELETE | `/admin/v1/content/categories/:id` | 权限 | 删除分类 |

### 内容管理 - 文章（04-content.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/content/articles` | 权限 | 获取文章列表 |
| GET | `/admin/v1/content/articles/:id` | 权限 | 获取文章详情 |
| POST | `/admin/v1/content/articles` | 权限 | 创建文章 |
| PUT | `/admin/v1/content/articles/:id` | 权限 | 更新文章 |
| DELETE | `/admin/v1/content/articles/:id` | 权限 | 删除文章 |
| PUT | `/admin/v1/content/articles/:id/publish` | 权限 | 发布文章 |
| PUT | `/admin/v1/content/articles/:id/unpublish` | 权限 | 撤销发布 |
| PUT | `/admin/v1/content/articles/:id/top` | 权限 | 置顶/取消置顶 |

### 内容管理 - Banner 分组（04-content.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/content/banner-groups` | 权限 | 获取 Banner 分组列表 |
| GET | `/admin/v1/content/banner-groups/:id` | 权限 | 获取 Banner 分组详情 |
| POST | `/admin/v1/content/banner-groups` | 权限 | 创建 Banner 分组 |
| PUT | `/admin/v1/content/banner-groups/:id` | 权限 | 更新 Banner 分组 |
| DELETE | `/admin/v1/content/banner-groups/:id` | 权限 | 删除 Banner 分组 |

### 内容管理 - Banner 项（04-content.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/content/banner-items` | 权限 | 获取 Banner 项列表 |
| GET | `/admin/v1/content/banner-items/:id` | 权限 | 获取 Banner 项详情 |
| POST | `/admin/v1/content/banner-items` | 权限 | 创建 Banner 项 |
| PUT | `/admin/v1/content/banner-items/:id` | 权限 | 更新 Banner 项 |
| DELETE | `/admin/v1/content/banner-items/:id` | 权限 | 删除 Banner 项 |

### 字典管理 - 字典类型（05-dict.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/system/dict/types` | 权限 | 获取字典类型列表 |
| POST | `/admin/v1/system/dict/types` | 权限 | 创建字典类型 |
| PUT | `/admin/v1/system/dict/types` | 权限 | 更新字典类型 |
| DELETE | `/admin/v1/system/dict/types/:id` | 权限 | 删除字典类型 |

### 字典管理 - 字典数据（05-dict.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/system/dict/data` | 权限 | 获取字典数据列表 |
| POST | `/admin/v1/system/dict/data` | 权限 | 创建字典数据 |
| PUT | `/admin/v1/system/dict/data` | 权限 | 更新字典数据 |
| DELETE | `/admin/v1/system/dict/data/:id` | 权限 | 删除字典数据 |
| GET | `/admin/v1/system/dict/data/:code` | 公开 | 根据编码获取字典数据 |

### 存储管理 - 存储配置（06-storage.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/storage-configs` | 权限 | 获取存储配置列表 |
| GET | `/admin/v1/storage-configs/all-enabled` | 权限 | 获取所有启用的存储配置 |
| GET | `/admin/v1/storage-configs/:id` | 权限 | 获取存储配置详情 |
| POST | `/admin/v1/storage-configs` | 权限 | 创建存储配置 |
| PUT | `/admin/v1/storage-configs` | 权限 | 更新存储配置 |
| DELETE | `/admin/v1/storage-configs/:id` | 权限 | 删除存储配置 |
| PUT | `/admin/v1/storage-configs/:id/default` | 权限 | 设置默认存储配置 |
| POST | `/admin/v1/storage-configs/test-upload` | 权限 | 测试存储上传 |

### 存储管理 - 上传记录（06-storage.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/upload-records` | 权限 | 获取上传记录列表 |
| GET | `/admin/v1/upload-records/:id` | 权限 | 获取上传记录详情 |
| DELETE | `/admin/v1/upload-records/:id` | 权限 | 删除上传记录 |
| POST | `/admin/v1/upload-records/batch-delete` | 权限 | 批量删除上传记录 |

### 存储管理 - 文件上传（06-storage.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| POST | `/admin/v1/storage/upload-credentials` | 认证 | 获取上传凭证 |
| POST | `/admin/v1/storage/upload-record` | 认证 | 上传成功通知 |

### 运维管理 - 操作日志（07-ops.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/operation-logs` | 权限 | 获取操作日志列表 |
| DELETE | `/admin/v1/operation-logs/:id` | 权限 | 删除操作日志 |
| POST | `/admin/v1/operation-logs/batch-delete` | 权限 | 批量删除操作日志 |

### 运维管理 - 错误日志（07-ops.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/error-logs` | 权限 | 获取错误日志列表 |
| PUT | `/admin/v1/error-logs/:id/resolve` | 权限 | 标记错误日志已解决 |
| DELETE | `/admin/v1/error-logs/:id` | 权限 | 删除错误日志 |
| POST | `/admin/v1/error-logs/batch-delete` | 权限 | 批量删除错误日志 |

### 运维管理 - IP 访问控制（07-ops.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/open-platform/ip-access` | 权限 | 获取 IP 访问规则列表 |
| POST | `/admin/v1/open-platform/ip-access` | 权限 | 新增 IP 访问规则 |
| PUT | `/admin/v1/open-platform/ip-access` | 权限 | 修改 IP 访问规则 |
| DELETE | `/admin/v1/open-platform/ip-access/:id` | 权限 | 删除 IP 访问规则 |
| DELETE | `/admin/v1/open-platform/ip-access/batch` | 权限 | 批量删除 IP 访问规则 |

### 运维管理 - 开放平台日志（07-ops.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/ops/open-platform-log` | 权限 | 获取开放平台调用日志列表 |

### 开放平台 - 应用管理（08-open-platform.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/open/apps` | 权限 | 获取应用列表 |
| POST | `/admin/v1/open/apps` | 权限 | 创建应用 |
| PUT | `/admin/v1/open/apps` | 权限 | 更新应用 |
| DELETE | `/admin/v1/open/apps/:id` | 权限 | 删除应用 |
| PUT | `/admin/v1/open/apps/reset-secret` | 权限 | 重置密钥 |
| PUT | `/admin/v1/open/apps/ip-rules` | 权限 | 关联 IP 规则 |
| GET | `/admin/v1/open/apps/scopes` | 权限 | 获取应用权限范围 |
| GET | `/admin/v1/open/apps/available-scopes` | 权限 | 获取可用权限范围 |

### 开放平台 - 权限范围管理（08-open-platform.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/open/scopes` | 权限 | 获取权限分组列表 |
| POST | `/admin/v1/open/scopes` | 权限 | 创建权限分组 |
| PUT | `/admin/v1/open/scopes` | 权限 | 更新权限分组 |
| DELETE | `/admin/v1/open/scopes/:id` | 权限 | 删除权限分组 |

### 开放平台 - 开放 API 管理（08-open-platform.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/open/apis` | 权限 | 获取开放 API 列表 |
| POST | `/admin/v1/open/apis` | 权限 | 创建开放 API |
| PUT | `/admin/v1/open/apis` | 权限 | 更新开放 API |
| DELETE | `/admin/v1/open/apis/:id` | 权限 | 删除开放 API |
| GET | `/admin/v1/open/apis/grouped` | 权限 | 获取分组 API 列表 |
| GET | `/admin/v1/open/apis/scope-apis` | 权限 | 获取权限范围关联的 API |
| PUT | `/admin/v1/open/apis/scope-apis` | 权限 | 更新权限范围关联的 API |

### 消息管理 - 模板管理（09-message.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/message/templates` | 权限 | 获取消息模板列表 |
| POST | `/admin/v1/message/templates` | 权限 | 创建消息模板 |
| PUT | `/admin/v1/message/templates/:id` | 权限 | 更新消息模板 |
| DELETE | `/admin/v1/message/templates/:id` | 权限 | 删除消息模板 |

### 消息管理 - 记录与发送（09-message.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/message/records` | 权限 | 获取消息记录列表 |
| POST | `/admin/v1/message/records/:id/retry` | 权限 | 重发消息 |
| POST | `/admin/v1/message/send` | 权限 | 直接发送消息 |

### 任务管理（10-task.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/system/tasks` | 权限 | 获取任务列表 |
| POST | `/admin/v1/system/tasks/:name/run` | 权限 | 手动执行任务 |
| POST | `/admin/v1/system/tasks/:name/start` | 权限 | 启动任务 |
| POST | `/admin/v1/system/tasks/:name/stop` | 权限 | 停止任务 |
| POST | `/admin/v1/system/tasks/:name/reload` | 权限 | 重载任务配置 |
| PUT | `/admin/v1/system/tasks/update` | 权限 | 更新任务配置 |
| GET | `/admin/v1/system/tasks/logs` | 权限 | 获取任务日志列表 |

### 系统配置（12-config.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/system/configs` | 公开 | 获取配置分组 |
| PUT | `/admin/v1/system/configs` | 权限 | 更新系统配置 |
| POST | `/admin/v1/system/test-email` | 权限 | 测试邮件发送 |

### 通用 API（13-common.md）

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/common/captcha` | 公开 | 获取图形验证码 |
| GET | `/admin/v1/route/getUserRoutes` | 认证 | 获取当前用户路由 |
| GET | `/admin/v1/route/isRouteExist` | 认证 | 检查路由是否存在 |

---

## 统计

| 统计项 | 数量 |
|--------|------|
| 文档文件 | 14 篇 |
| API 端点 | 110+ 个 |
| 公开接口 | 5 个 |
| 认证接口 | 7 个 |
| 权限接口 | 98+ 个 |
| 错误码 | 50+ 个 |
