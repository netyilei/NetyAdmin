# Admin-Web 架构设计与目录结构

本文档详细介绍 NetyAdmin 管理后台前端（Vue 3 + TypeScript）的架构设计理念、分层结构和二次开发指南。

---

## 一、架构设计理念

### 1.1 设计目标

NetyAdmin Admin-Web 采用 **严格的页面层架构（Strict Page-Layer Architecture）**，旨在解决以下问题：

- **组件耦合**：避免跨视图的"意大利面条式"引用
- **API版本管理**：支持多版本API平滑演进
- **国际化维护**：消灭"魔法状态码"和硬编码文本
- **代码质量**：保持 0 Error, 0 Warning 的绝对干净状态

### 1.2 核心设计原则

| 原则 | 说明 |
|------|------|
| **高内聚** | 页面专属组件严格存放在 `views/.../components/` 下 |
| **API版本隔离** | 按版本划分API文件（`v1/`），显式导入 |
| **状态码解耦** | 后端返回code，前端映射为i18n文本 |
| **零死代码** | 严格Lint规则，禁止未使用变量和导入 |
| **类型安全** | 全TypeScript覆盖，API类型定义独立维护 |

---

## 二、目录结构详解

```
admin-web/
├── build/                         # 构建相关配置
│   ├── config/                   # Vite配置
│   │   ├── index.ts             # 基础配置
│   │   ├── proxy.ts             # 代理配置
│   │   └── time.ts              # 构建时间戳
│   └── plugins/                  # Vite插件
│       ├── html.ts              # HTML插件
│       ├── unocss.ts            # UnoCSS插件
│       └── unplugin.ts          # 自动导入插件
│
├── packages/                      # pnpm workspace内部包
│   ├── alova/                    # HTTP请求库封装
│   ├── axios/                    # Axios封装（备用）
│   ├── color/                    # 颜色处理工具
│   ├── hooks/                    # 通用组合式函数
│   ├── materials/                # UI组件库
│   │   ├── admin-layout/        # 后台布局组件
│   │   ├── page-tab/            # 页面标签组件
│   │   └── simple-scrollbar/    # 滚动条组件
│   ├── scripts/                  # 工程化脚本
│   ├── uno-preset/               # UnoCSS预设
│   └── utils/                    # 工具函数
│
├── public/                        # 静态资源
│   └── favicon.svg
│
└── src/                           # 业务源码
    ├── assets/                    # 图片与图标
    │   ├── imgs/                 # 图片资源
    │   └── svg-icon/             # SVG图标
    │
    ├── components/                # 组件层
    │   ├── advanced/             # 高级组件（表格操作等）
    │   │   ├── table-column-setting.vue
    │   │   └── table-header-operation.vue
    │   │
    │   ├── common/               # 通用基础组件
    │   │   ├── app-provider.vue
    │   │   ├── dark-mode-container.vue
    │   │   ├── exception-base.vue
    │   │   ├── full-screen.vue
    │   │   ├── lang-switch.vue
    │   │   ├── menu-toggler.vue
    │   │   ├── pin-toggler.vue
    │   │   ├── reload-button.vue
    │   │   ├── system-logo.vue
    │   │   └── theme-schema-switch.vue
    │   │
    │   └── custom/               # 业务组件（跨页面复用）
    │       ├── app-dict-radio-group.vue
    │       ├── app-dict-select.vue
    │       ├── button-icon.vue
    │       ├── count-to.vue
    │       ├── custom-icon-select.vue
    │       ├── netyadmin-avatar.vue
    │       ├── storage-config-select.vue
    │       ├── svg-icon.vue
    │       └── toast-ui-editor.vue
    │
    ├── config/                    # 应用配置
    │   └── index.ts
    │
    ├── constants/                 # 常量定义
    │   ├── app.ts                # 应用常量
    │   ├── map-sdk.ts            # 地图SDK配置
    │   └── reg.ts                # 正则表达式
    │
    ├── enum/                      # 枚举定义
    │   └── index.ts
    │
    ├── hooks/                     # 组合式函数
    │   ├── business/             # 业务Hooks
    │   │   └── auth.ts           # 认证相关
    │   └── common/               # 通用Hooks
    │       ├── dict.ts           # 字典Hook
    │       ├── echarts.ts        # ECharts Hook
    │       ├── form.ts           # 表单Hook
    │       ├── icon.ts           # 图标Hook
    │       ├── operation.ts      # 操作Hook
    │       ├── router.ts         # 路由Hook
    │       ├── table.ts          # 表格Hook
    │       └── vchart.ts         # VChart Hook
    │
    ├── layouts/                   # 布局组件
    │   ├── base-layout/          # 基础布局
    │   ├── blank-layout/         # 空白布局
    │   ├── context/              # 布局上下文
    │   └── modules/              # 布局模块
    │       ├── global-breadcrumb/ # 全局面包屑
    │       ├── global-content/    # 全局内容区
    │       ├── global-footer/     # 全局页脚
    │       ├── global-header/     # 全局头部
    │       ├── global-logo/       # 全局Logo
    │       ├── global-menu/       # 全局菜单
    │       ├── global-search/     # 全局搜索
    │       ├── global-sider/      # 全局侧边栏
    │       ├── global-tab/        # 全局标签页
    │       └── theme-drawer/      # 主题抽屉
    │
    ├── locales/                   # 国际化资源
    │   ├── langs/                # 语言包
    │   │   ├── en-us/            # 英文
    │   │   │   ├── page/         # 页面文案
    │   │   │   ├── common.ts
    │   │   │   ├── datatable.ts
    │   │   │   ├── form.ts
    │   │   │   ├── request.ts    # 后端错误码映射
    │   │   │   ├── route.ts
    │   │   │   └── system.ts
    │   │   └── zh-cn/            # 中文
    │   │       ├── page/
    │   │       ├── common.ts
    │   │       ├── datatable.ts
    │   │       ├── form.ts
    │   │       ├── request.ts    # 后端错误码映射
    │   │       ├── route.ts
    │   │       └── system.ts
    │   ├── dayjs.ts              # Day.js国际化
    │   ├── index.ts              # 入口
    │   ├── locale.ts             # 语言切换逻辑
    │   └── naive.ts              # Naive UI国际化
    │
    ├── plugins/                   # 应用插件
    │   ├── app.ts                # 应用初始化
    │   ├── assets.ts             # 资源加载
    │   ├── dayjs.ts              # Day.js配置
    │   ├── iconify.ts            # Iconify图标
    │   ├── index.ts              # 插件入口
    │   ├── loading.ts            # 加载动画
    │   └── nprogress.ts          # 进度条
    │
    ├── router/                    # 路由配置
    │   ├── guard/                # 路由守卫
    │   │   ├── index.ts          # 守卫入口
    │   │   ├── progress.ts       # 进度条守卫
    │   │   ├── route.ts          # 路由守卫
    │   │   └── title.ts          # 标题守卫
    │   ├── routes/               # 路由定义
    │   │   ├── builtin.ts        # 内置路由
    │   │   └── index.ts          # 路由入口
    │   └── index.ts              # 路由实例
    │
    ├── service/                   # 【严格接口层】
    │   ├── api/                  # API定义
    │   │   └── v1/               # V1版本API
    │   │       ├── auth.ts       # 认证API
    │   │       ├── content.ts    # 内容API
    │   │       ├── log.ts        # 日志API
    │   │       ├── route.ts      # 路由API
    │   │       ├── storage.ts    # 存储API
    │   │       ├── system-dict.ts # 字典API
    │   │       ├── system-manage.ts # 系统管理API
    │   │       └── system-task.ts   # 任务API
    │   │
    │   └── request/              # 请求封装
    │       ├── backend-error.ts  # 后端错误处理
    │       ├── index.ts          # Axios实例
    │       ├── shared.ts         # 共享逻辑
    │       └── type.ts           # 类型定义
    │
    ├── store/                     # Pinia状态管理
    │   ├── modules/              # 状态模块
    │   │   ├── app/              # 应用状态
    │   │   ├── auth/             # 认证状态
    │   │   ├── dict/             # 字典状态
    │   │   ├── route/            # 路由状态
    │   │   ├── tab/              # 标签页状态
    │   │   └── theme/            # 主题状态
    │   ├── plugins/              # 状态插件
    │   └── index.ts              # Store入口
    │
    ├── styles/                    # 全局样式
    │   ├── css/                  # CSS文件
    │   │   ├── global.css
    │   │   ├── nprogress.css
    │   │   ├── reset.css
    │   │   └── transition.css
    │   └── scss/                 # SCSS文件
    │       ├── global.scss
    │       └── scrollbar.scss
    │
    ├── theme/                     # 主题配置
    │   ├── settings.ts           # 主题设置
    │   └── vars.ts               # 主题变量
    │
    ├── typings/                   # TypeScript类型声明
    │   ├── api/                  # API类型
    │   │   └── v1/               # V1版本类型
    │   │       ├── auth.d.ts
    │   │       ├── common.d.ts
    │   │       ├── content.d.ts
    │   │       ├── log.d.ts
    │   │       ├── route.d.ts
    │   │       ├── storage.d.ts
    │   │       ├── system-dict.d.ts
    │   │       └── system-manage.d.ts
    │   ├── app/                  # 应用类型
    │   ├── components.d.ts       # 组件类型
    │   ├── global.d.ts           # 全局类型
    │   ├── router.d.ts           # 路由类型
    │   └── vite-env.d.ts         # Vite环境类型
    │
    ├── utils/                     # 工具函数
    │   ├── agent.ts              # 浏览器检测
    │   ├── common.ts             # 通用工具
    │   ├── icon.ts               # 图标工具
    │   ├── iconify-icons.ts      # Iconify图标
    │   ├── service.ts            # 服务工具
    │   ├── storage.ts            # 存储工具
    │   └── upload.ts             # 上传工具
    │
    ├── views/                     # 【严格页面层】
    │   ├── _builtin/             # 内置页面
    │   │   ├── 403/
    │   │   ├── 404/
    │   │   └── 500/
    │   │
    │   ├── content/              # 内容管理
    │   │   ├── article/
    │   │   │   ├── components/   # 【页面级组件】
    │   │   │   └── index.vue
    │   │   ├── banner/
    │   │   └── category/
    │   │
    │   ├── home/                 # 首页
    │   ├── manage/               # 系统管理（RBAC）
    │   │   ├── admin/
    │   │   ├── role/
    │   │   ├── menu/
    │   │   └── dict/
    │   │
    │   ├── ops/                  # 运维审计
    │   │   ├── error-log/
    │   │   ├── operation-log/
    │   │   └── upload-record/
    │   │
    │   └── settings/             # 系统设置
    │       ├── config/
    │       └── task/
    │
    ├── App.vue                    # 根组件
    └── main.ts                    # 应用入口
```

---

## 三、核心开发规范

### 3.1 红线规则（必须遵守）

| 规则 | 说明 | 违规后果 |
|------|------|----------|
| **禁止跨Views引用** | `views/manage/` 下的文件不允许 `import` `views/content/` 下的组件 | 破坏模块独立性 |
| **API职责单一** | `.vue` 文件中严禁硬编码URL或直接调用axios | 维护困难 |
| **状态码统一收口** | 业务代码中严禁写死数字状态码 | 国际化失效 |
| **命名规范** | Vue文件和普通TS文件使用 `kebab-case`，类/接口使用 `PascalCase` | 风格不一致 |

### 3.2 组件使用规范

```typescript
// ✅ 正确：从service/api导入
import { fetchGetArticleList } from '@/service/api/v1/content'

// ❌ 错误：直接调用axios或在组件中写URL
axios.get('/admin/v1/content/articles')
```

### 3.3 页面组件组织

```
views/content/article/
├── components/                    # 【仅限本页面使用的组件】
│   ├── article-form.vue          # 文章表单
│   ├── article-table.vue         # 文章表格
│   └── article-filter.vue        # 筛选组件
└── index.vue                      # 页面入口
```

### 3.4 状态码处理流程

```
后端返回: { code: "101001", msg: "", data: null }
           ↓
request/index.ts 拦截器接收
           ↓
backend-error.ts 映射 code -> i18n key
           ↓
locales/langs/zh-cn/request.ts 获取中文文本
           ↓
ElMessage.error("用户不存在")
```

---

## 四、二次开发示例

### 4.1 新增页面模块（以"评论管理"为例）

#### 步骤1：定义API类型

```typescript
// src/typings/api/v1/content.d.ts

/** 评论项 */
interface Comment {
  id: number
  article_id: number
  content: string
  status: number
  created_at: number
}

/** 获取评论列表请求 */
interface GetCommentListRequest {
  article_id?: number
  page?: number
  size?: number
}

/** 获取评论列表响应 */
interface GetCommentListResponse {
  list: Comment[]
  total: number
}

/** 创建评论请求 */
interface CreateCommentRequest {
  article_id: number
  content: string
}
```

#### 步骤2：创建API函数

```typescript
// src/service/api/v1/content.ts

import { request } from '@/service/request'

/** 获取评论列表 */
export function fetchGetCommentList(params: ApiV1.GetCommentListRequest) {
  return request<ApiV1.GetCommentListResponse>({
    url: '/admin/v1/content/comments',
    method: 'GET',
    params
  })
}

/** 创建评论 */
export function fetchCreateComment(data: ApiV1.CreateCommentRequest) {
  return request<ApiV1.Comment>({
    url: '/admin/v1/content/comments',
    method: 'POST',
    data
  })
}

/** 删除评论 */
export function fetchDeleteComment(id: number) {
  return request<null>({
    url: `/admin/v1/content/comments/${id}`,
    method: 'DELETE'
  })
}
```

#### 步骤3：创建页面组件

```vue
<!-- src/views/content/comment/index.vue -->
<template>
  <div class="comment-management">
    <NCard title="评论管理">
      <NDataTable
        :columns="columns"
        :data="tableData"
        :loading="loading"
        :pagination="pagination"
        @update:page="handlePageChange"
      />
    </NCard>
  </div>
</template>

<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { NButton, NCard, NDataTable, NSpace, useDialog, useMessage } from 'naive-ui'
import { fetchGetCommentList, fetchDeleteComment } from '@/service/api/v1/content'

const message = useMessage()
const dialog = useDialog()

const loading = ref(false)
const tableData = ref<ApiV1.Comment[]>([])
const pagination = ref({
  page: 1,
  pageSize: 20,
  itemCount: 0
})

const columns = [
  { title: 'ID', key: 'id' },
  { title: '文章ID', key: 'article_id' },
  { title: '内容', key: 'content', ellipsis: { tooltip: true } },
  { title: '状态', key: 'status' },
  {
    title: '操作',
    key: 'actions',
    render(row: ApiV1.Comment) {
      return h(NSpace, {}, {
        default: () => [
          h(NButton, {
            size: 'small',
            type: 'error',
            onClick: () => handleDelete(row)
          }, { default: () => '删除' })
        ]
      })
    }
  }
]

async function loadData() {
  loading.value = true
  const { data } = await fetchGetCommentList({
    page: pagination.value.page,
    size: pagination.value.pageSize
  })
  if (data) {
    tableData.value = data.list
    pagination.value.itemCount = data.total
  }
  loading.value = false
}

function handleDelete(row: ApiV1.Comment) {
  dialog.warning({
    title: '确认删除',
    content: `确定要删除评论 #${row.id} 吗？`,
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      const { error } = await fetchDeleteComment(row.id)
      if (!error) {
        message.success('删除成功')
        loadData()
      }
    }
  })
}

function handlePageChange(page: number) {
  pagination.value.page = page
  loadData()
}

onMounted(loadData)
</script>
```

#### 步骤4：配置路由

后端动态路由会自动返回菜单配置，无需在前端硬编码路由。

如需本地测试，可在 `src/router/routes/builtin.ts` 添加：

```typescript
{
  name: 'comment',
  path: '/content/comment',
  component: () => import('@/views/content/comment/index.vue'),
  meta: {
    title: '评论管理',
    i18nKey: 'route.content.comment'
  }
}
```

#### 步骤5：添加国际化

```typescript
// src/locales/langs/zh-cn/route.ts
export default {
  content: {
    comment: '评论管理'
  }
}

// src/locales/langs/en-us/route.ts
export default {
  content: {
    comment: 'Comment Management'
  }
}
```

---

## 五、状态管理使用

### 5.1 使用Pinia Store

```typescript
// 在组件中使用
import { useAuthStore } from '@/store/modules/auth'
import { useDictStore } from '@/store/modules/dict'

const authStore = useAuthStore()
const dictStore = useDictStore()

// 获取用户信息
const userInfo = computed(() => authStore.userInfo)

// 获取字典数据
const statusOptions = computed(() => dictStore.getDictData('article_status'))
```

### 5.2 字典Hook使用

```typescript
import { useDict } from '@/hooks/common/dict'

const { dictData, loadDict } = useDict('article_status')

// 在模板中使用
<NSelect :options="dictData.value" />
```

---

## 六、相关文档

- [Server架构设计](./server-architecture.md)
- [状态码规范](./status-codes.md)
- [API管理指南](./api-management.md)
- [快速部署指南](./quick-deployment.md)
