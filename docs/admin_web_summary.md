# Admin-Web（Vue）现状总结

## 技术栈与工程特征

- Vue 3 + TypeScript + Vite
- UI：Naive UI
- 样式：UnoCSS（含 preset 与工程化插件）
- 状态管理：Pinia
- 路由：Vue Router
- 国际化：vue-i18n（双语资源在 `src/locales/langs/*`）
- 请求层：统一 `src/service/request` 封装，业务 API 位于 `src/service/api/v1`
- 工程组织：pnpm workspace（`packages/` 下内置多个 workspace 包）

## 路由与权限的运行方式（当前实现口径）

- 登录前：只挂载常量路由（登录/403/404/500/iframe 等）
- 登录后：
  - 前端会调用后端 `/admin/v1/route/getUserRoutes` 拉取动态路由树
  - 根据路由树渲染菜单与页面访问控制
- 后端侧 RBAC 决定“动态路由树”和 API 访问权限

## 已实现的页面模块（按前端代码目录）

### 1) 登录与内置页

- 登录页（`src/views/_builtin/login`）
- 403 / 404 / 500
- iframe 页面容器（用于菜单配置外链/内嵌页）

### 2) Dashboard / Home

- 首页（含卡片数据、折线图、饼图等模块组件）

### 3) 内容管理（Content）

- 文章管理（列表、创建/编辑、发布/取消发布、置顶）
- Banner 管理（分组、条目）
- 分类管理（支持树形结构）

### 4) 系统管理（System Manage / RBAC）

- 管理员管理
- 角色管理（含菜单/按钮/API 授权）
- 菜单管理（含页面组件映射、菜单树）

### 5) 系统功能（System）

- 系统配置（开关/参数）
- 动态字典（类型与数据）
- 任务管理（任务列表、运行/启停/重载、日志）

### 6) 运维与审计（Ops）

- 操作日志
- 错误日志
- 上传记录

### 7) 对象存储（Storage）

- 存储配置管理
- 测试上传
- 上传凭证与上传记录写入（结合上传组件）

