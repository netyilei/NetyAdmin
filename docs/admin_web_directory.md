# Admin-Web 目录结构与架构规范

本文档定义 `admin-web` 的标准目录结构与分层架构核心规范，作为未来二次开发的准则。

## 1. 架构核心特性

### 1.1 视图组件的高内聚 (Strict Page-Layer Architecture)
- 页面专属的局部组件统一存放在 `src/views/.../components/` 下，明确其作为“页面级局部组件”的语义。
- 通过严格的目录语义限制，杜绝跨视图的“意大利面条式”引用。如果多个页面需要共用组件，必须提升到全局的 `src/components/custom/`，保证了模块的独立性和可拔插性。

### 1.2 API 版本隔离与显式导入 (Explicit Versioning)
- 所有的 API 文件和对应的 TypeScript 定义均按版本划分（如存放在 `v1` 目录下）。
- 业务层必须显式导入（如 `import { fetchGetArticle } from '@/service/api/v1/content'`）。代码意图极度清晰，彻底解决了循环依赖问题，并为未来多版本 API 的平滑演进铺平了道路。

### 1.3 状态码与多语言彻底解耦 (i18n Error Mapping)
- 服务端统一响应结构 `{ code: "200", msg: "", data, request_id }`，其中成功码固定为 `"200"`，`msg` 永远为空。
- 前端在 `src/service/request/backend-error.ts` 集中拦截，将 `code` 映射为当前语言的 i18n 文本。
- 消灭了前端所有的“魔法状态码”和硬编码文本。后端逻辑纯粹，前端业务层拿到的直接是翻译好的结构化错误，极大降低了国际化维护成本。

### 1.4 严格的代码防御 (Zero Dead Code)
- 配置了严格的 `unused-imports` 和 `no-unused-vars` Lint 规则。
- 当前基座保持 **0 Error, 0 Warning** 的绝对干净状态。配合 CI/CD 流程，有效防止代码“破窗效应”，确保交付质量。

---

## 2. 顶层目录结构

```text
admin-web/
├── build/                         # 构建相关：vite 插件、proxy、时间戳等
├── packages/                      # pnpm workspace 内部包（@na/*）
├── public/                        # 静态资源
├── src/                           # 业务源码
│   ├── assets/                    # 图片与 svg icon
│   ├── components/                # 通用组件（common）与跨模块业务组件（custom）
│   ├── constants/                 # 常量定义
│   ├── hooks/                     # 全局组合式 hooks
│   ├── layouts/                   # 布局组件（不属于业务域）
│   ├── locales/                   # 国际化资源（zh-cn, en-us）
│   ├── plugins/                   # 应用级插件（loading, nprogress 等）
│   ├── router/                    # 路由与守卫
│   ├── service/                   # 【严格接口层】
│   │   ├── api/
│   │   │   └── v1/                # V1 版本业务 API（按业务域拆分，如 auth.ts）
│   │   └── request/               # Axios 封装与全局状态码拦截
│   ├── store/                     # Pinia 全局状态
│   ├── styles/                    # 全局样式
│   ├── theme/                     # 主题配置
│   ├── typings/                   # 前端类型声明
│   │   ├── api/
│   │   │   └── v1/                # V1 版本接口的 TS 类型声明
│   │   └── app/                   # 应用全局类型
│   ├── utils/                     # 工具函数
│   ├── views/                     # 【严格页面层】按业务域拆分
│   │   ├── _builtin/              # 登录、404 等内置页面
│   │   ├── manage/                # 系统管理域（管理员、角色、菜单等）
│   │   │   └── admin/
│   │   │       ├── components/    # 【严格约束】仅限 admin 页面使用的局部组件
│   │   │       └── index.vue
│   │   ├── content/               # 内容管理域
│   │   ├── ops/                   # 运维监控域
│   │   └── settings/              # 系统设置域
│   ├── App.vue
│   └── main.ts
└── [配置文件]                      # vite.config.ts, tsconfig.json, eslint.config.js 等
```

## 3. 核心开发规范 (Redlines)

为保持基座的纯净与高可维护性，任何基于此架构的二次开发必须遵守以下红线：

1. **绝对禁止跨 Views 引用**：`views/manage/` 下的文件，绝对不允许 `import xxx from '@/views/content/.../components/xxx'`。共用组件必须提取到 `src/components/custom/`。
2. **API 职责单一与显式版本**：`.vue` 文件中严禁硬编码 URL 或直接调用 `axios`。必须从 `src/service/api/v1/${module}.ts` 显式引入。
3. **状态码统一收口**：业务代码中严禁写死数字状态码。新增后端错误码时，必须在 `src/service/request/backend-error.ts` 的字典和对应语言包中同步增加。
4. **命名规范兜底**：所有 Vue 文件、普通 TS 文件使用 `kebab-case`（如 `user-detail.vue`）。导出的 TS 类/接口使用 `PascalCase`，导出的函数/变量使用 `camelCase`。

