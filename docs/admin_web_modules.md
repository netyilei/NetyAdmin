# Admin-Web 页面模块与路由侧行为

## 1) 动态路由机制（当前实现）

- 常量路由定义：`src/router/routes/index.ts`
  - 仅包含登录/错误页/iframe 容器等“登录前必需页面”
  - `createStaticRoutes()` 明确返回 `authRoutes: []`，表示“业务路由完全由后端供给”
- 动态路由数据来源：后端接口 `/admin/v1/route/getUserRoutes`
- 典型用途：
  - 生成侧边栏菜单
  - route guard 中用于鉴权与跳转控制
  - 通过 `/admin/v1/route/isRouteExist` 做路由存在性校验

## 2) 业务模块与页面目录映射（按 views/）

| 业务域 | 页面目录 | 说明 |
|---|---|---|
| 登录/内置页 | `views/_builtin/*` | login/403/404/500 |
| 首页 | `views/home/*` | dashboard 模块 |
| 内容管理 | `views/content/*` | article/banner/category |
| 系统管理（RBAC） | `views/manage/*` | admin/role/menu/dict |
| 运维审计 | `views/ops/*` | error-log/operation-log/task/upload-record |

## 3) 后端 API 调用入口（按 service/api/v1）

前端对 `/admin/v1/*` 的请求集中在以下文件：

- `src/service/api/v1/auth.ts`
- `src/service/api/v1/route.ts`
- `src/service/api/v1/system-manage.ts`
- `src/service/api/v1/system-task.ts`
- `src/service/api/v1/system-dict.ts`
- `src/service/api/v1/content.ts`
- `src/service/api/v1/storage.ts`
- `src/service/api/v1/log.ts`

抽离底座项目时，建议把这一层作为“标准 SDK 层”，对外保持：

- URL 命名空间：`/admin/v1`
- 业务域文件拆分：auth/route/system/content/storage/log
- 全局 request 封装：统一拦截 Token、错误码、重试策略等

## 4) 国际化资源分布

- 语言包：`src/locales/langs/zh-cn/*` 与 `src/locales/langs/en-us/*`
- 路由 i18n key：通常在路由 meta 的 `i18nKey` 中体现，最终由 `vue-i18n` 映射

