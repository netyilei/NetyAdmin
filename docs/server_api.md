# Server（Go）HTTP 路由清单（按当前代码注册）

说明：本清单来自 `server/internal/router` 与 `server/internal/router/v1` 的实际路由注册代码；不等同于 `docs/openapi.yaml`。

## 管理后台 API（`/admin/v1`）

### 1) 公开接口（无需 JWT）

| Method | Path |
|---|---|
| POST | `/admin/v1/auth/login` |
| POST | `/admin/v1/auth/refreshToken` |

### 2) 仅需 JWT（不走 RBAC）

#### Auth

| Method | Path |
|---|---|
| GET | `/admin/v1/auth/getUserInfo` |
| GET | `/admin/v1/auth/profile` |
| PUT | `/admin/v1/auth/profile` |
| POST | `/admin/v1/auth/changePassword` |

#### Route（动态路由）

| Method | Path |
|---|---|
| GET | `/admin/v1/route/getUserRoutes` |
| GET | `/admin/v1/route/isRouteExist` |

#### Storage（上传凭证与上传记录写入）

| Method | Path |
|---|---|
| POST | `/admin/v1/storage/upload-credentials` |
| POST | `/admin/v1/storage/upload-record` |

### 3) JWT + RBAC（权限中间件）

#### Admins（管理员 CRUD）

| Method | Path |
|---|---|
| GET | `/admin/v1/admins` |
| POST | `/admin/v1/admins` |
| GET | `/admin/v1/admins/:id` |
| PUT | `/admin/v1/admins/:id` |
| DELETE | `/admin/v1/admins/:id` |

#### SystemManage（RBAC 管理）

Admins

| Method | Path |
|---|---|
| GET | `/admin/v1/systemManage/getUserList` |
| POST | `/admin/v1/systemManage/addUser` |
| PUT | `/admin/v1/systemManage/updateUser` |
| DELETE | `/admin/v1/systemManage/deleteUser` |
| DELETE | `/admin/v1/systemManage/deleteUsers` |

Roles

| Method | Path |
|---|---|
| GET | `/admin/v1/systemManage/getRoleList` |
| GET | `/admin/v1/systemManage/getRole/:id` |
| GET | `/admin/v1/systemManage/getAllRoles` |
| POST | `/admin/v1/systemManage/addRole` |
| PUT | `/admin/v1/systemManage/updateRole` |
| DELETE | `/admin/v1/systemManage/deleteRole` |
| DELETE | `/admin/v1/systemManage/deleteRoles` |

Menus

| Method | Path |
|---|---|
| GET | `/admin/v1/systemManage/getMenuList` |
| GET | `/admin/v1/systemManage/getMenuTree` |
| GET | `/admin/v1/systemManage/getButtonTree` |
| GET | `/admin/v1/systemManage/getApiTree` |
| GET | `/admin/v1/systemManage/getAllPages` |
| GET | `/admin/v1/systemManage/getMenu/:id` |
| POST | `/admin/v1/systemManage/addMenu` |
| PUT | `/admin/v1/systemManage/updateMenu` |
| DELETE | `/admin/v1/systemManage/deleteMenu` |
| DELETE | `/admin/v1/systemManage/deleteMenus` |

APIs

| Method | Path |
|---|---|
| GET | `/admin/v1/systemManage/getApiList` |
| GET | `/admin/v1/systemManage/getAllApi` |
| GET | `/admin/v1/systemManage/getApi/:id` |
| POST | `/admin/v1/systemManage/createApi` |
| PUT | `/admin/v1/systemManage/updateApi` |
| DELETE | `/admin/v1/systemManage/deleteApi/:id` |

Buttons

| Method | Path |
|---|---|
| GET | `/admin/v1/systemManage/getButtonList` |
| GET | `/admin/v1/systemManage/getAllButton` |
| GET | `/admin/v1/systemManage/getButton/:id` |
| POST | `/admin/v1/systemManage/createButton` |
| PUT | `/admin/v1/systemManage/updateButton` |
| DELETE | `/admin/v1/systemManage/deleteButton` |

Role Permissions

| Method | Path |
|---|---|
| GET | `/admin/v1/systemManage/role/:id/menus` |
| PUT | `/admin/v1/systemManage/role/:id/menus` |
| GET | `/admin/v1/systemManage/role/:id/buttons` |
| PUT | `/admin/v1/systemManage/role/:id/buttons` |
| GET | `/admin/v1/systemManage/role/:id/apis` |
| PUT | `/admin/v1/systemManage/role/:id/apis` |

#### System（系统配置 / 任务 / 字典）

Configs

| Method | Path |
|---|---|
| GET | `/admin/v1/system/configs` |
| PUT | `/admin/v1/system/configs` |

Tasks

| Method | Path |
|---|---|
| GET | `/admin/v1/system/tasks` |
| POST | `/admin/v1/system/tasks/:name/run` |
| POST | `/admin/v1/system/tasks/:name/start` |
| POST | `/admin/v1/system/tasks/:name/stop` |
| POST | `/admin/v1/system/tasks/:name/reload` |
| PUT | `/admin/v1/system/tasks/:name` |
| GET | `/admin/v1/system/tasks/logs` |

Dict

| Method | Path |
|---|---|
| GET | `/admin/v1/system/dict/data/:code` |
| GET | `/admin/v1/system/dict/types` |
| POST | `/admin/v1/system/dict/types` |
| PUT | `/admin/v1/system/dict/types` |
| DELETE | `/admin/v1/system/dict/types/:id` |
| GET | `/admin/v1/system/dict/data` |
| POST | `/admin/v1/system/dict/data` |
| PUT | `/admin/v1/system/dict/data` |
| DELETE | `/admin/v1/system/dict/data/:id` |

#### Storage Configs（对象存储配置）

| Method | Path |
|---|---|
| GET | `/admin/v1/storage-configs` |
| GET | `/admin/v1/storage-configs/:id` |
| POST | `/admin/v1/storage-configs` |
| PUT | `/admin/v1/storage-configs` |
| DELETE | `/admin/v1/storage-configs/:id` |
| PUT | `/admin/v1/storage-configs/:id/default` |
| POST | `/admin/v1/storage-configs/test-upload` |

#### Upload Records（上传记录）

| Method | Path |
|---|---|
| GET | `/admin/v1/upload-records` |
| GET | `/admin/v1/upload-records/:id` |
| DELETE | `/admin/v1/upload-records/:id` |
| POST | `/admin/v1/upload-records/batch-delete` |

#### Content（内容管理）

Categories

| Method | Path |
|---|---|
| GET | `/admin/v1/content/categories` |
| GET | `/admin/v1/content/categories/tree` |
| GET | `/admin/v1/content/categories/:id` |
| POST | `/admin/v1/content/categories` |
| PUT | `/admin/v1/content/categories/:id` |
| DELETE | `/admin/v1/content/categories/:id` |

Articles

| Method | Path |
|---|---|
| GET | `/admin/v1/content/articles` |
| GET | `/admin/v1/content/articles/:id` |
| POST | `/admin/v1/content/articles` |
| PUT | `/admin/v1/content/articles/:id` |
| DELETE | `/admin/v1/content/articles/:id` |
| PUT | `/admin/v1/content/articles/:id/publish` |
| PUT | `/admin/v1/content/articles/:id/unpublish` |
| PUT | `/admin/v1/content/articles/:id/top` |

Banner Groups

| Method | Path |
|---|---|
| GET | `/admin/v1/content/banner-groups` |
| GET | `/admin/v1/content/banner-groups/:id` |
| POST | `/admin/v1/content/banner-groups` |
| PUT | `/admin/v1/content/banner-groups/:id` |
| DELETE | `/admin/v1/content/banner-groups/:id` |

Banner Items

| Method | Path |
|---|---|
| GET | `/admin/v1/content/banner-items` |
| GET | `/admin/v1/content/banner-items/:id` |
| POST | `/admin/v1/content/banner-items` |
| PUT | `/admin/v1/content/banner-items/:id` |
| DELETE | `/admin/v1/content/banner-items/:id` |

#### Logs（审计/错误）

Operation Logs

| Method | Path |
|---|---|
| GET | `/admin/v1/operation-logs` |
| DELETE | `/admin/v1/operation-logs/:id` |
| POST | `/admin/v1/operation-logs/batch-delete` |

Error Logs

| Method | Path |
|---|---|
| GET | `/admin/v1/error-logs` |
| PUT | `/admin/v1/error-logs/:id/resolve` |
| DELETE | `/admin/v1/error-logs/:id` |
| POST | `/admin/v1/error-logs/batch-delete` |
