# 内容管理 API

本模块涵盖内容分类、文章及 Banner（轮播图）的管理功能，包括分类的 CRUD 与树形结构、文章的 CRUD 与发布/置顶操作、Banner 分组与 Banner 项的 CRUD。

所有接口均需要 JWT Token + RBAC 权限校验。

## 接口概览

### 分类管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/content/categories` | 获取分类列表 |
| GET | `/admin/v1/content/categories/tree` | 获取分类树 |
| GET | `/admin/v1/content/categories/:id` | 获取分类详情 |
| POST | `/admin/v1/content/categories` | 创建分类 |
| PUT | `/admin/v1/content/categories/:id` | 更新分类 |
| DELETE | `/admin/v1/content/categories/:id` | 删除分类 |

### 文章管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/content/articles` | 获取文章列表 |
| GET | `/admin/v1/content/articles/:id` | 获取文章详情 |
| POST | `/admin/v1/content/articles` | 创建文章 |
| PUT | `/admin/v1/content/articles/:id` | 更新文章 |
| DELETE | `/admin/v1/content/articles/:id` | 删除文章 |
| PUT | `/admin/v1/content/articles/:id/publish` | 发布文章 |
| PUT | `/admin/v1/content/articles/:id/unpublish` | 撤销发布 |
| PUT | `/admin/v1/content/articles/:id/top` | 置顶/取消置顶 |

### Banner 分组管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/content/banner-groups` | 获取 Banner 分组列表 |
| GET | `/admin/v1/content/banner-groups/:id` | 获取 Banner 分组详情 |
| POST | `/admin/v1/content/banner-groups` | 创建 Banner 分组 |
| PUT | `/admin/v1/content/banner-groups/:id` | 更新 Banner 分组 |
| DELETE | `/admin/v1/content/banner-groups/:id` | 删除 Banner 分组 |

### Banner 项管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/content/banner-items` | 获取 Banner 项列表 |
| GET | `/admin/v1/content/banner-items/:id` | 获取 Banner 项详情 |
| POST | `/admin/v1/content/banner-items` | 创建 Banner 项 |
| PUT | `/admin/v1/content/banner-items/:id` | 更新 Banner 项 |
| DELETE | `/admin/v1/content/banner-items/:id` | 删除 Banner 项 |

---

## 一、分类管理

### 1.1 获取分类列表

分页查询分类列表，支持按名称、编码、内容类型、状态筛选。

```
GET /admin/v1/content/categories
```

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 当前页码 |
| `size` | int | 否 | 每页数量 |
| `name` | string | 否 | 分类名称 |
| `code` | string | 否 | 分类编码 |
| `contentType` | string | 否 | 内容类型（`plaintext`/`richtext`） |
| `status` | string | 否 | 状态（`0`:禁用 `1`:启用） |

### 1.2 获取分类树

获取分类的树形结构。

```
GET /admin/v1/content/categories/tree
```

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": [
    {
      "id": 1,
      "parentId": 0,
      "name": "新闻",
      "code": "news",
      "icon": "news-icon",
      "sort": 1,
      "storageConfigId": null,
      "contentType": "richtext",
      "status": "1",
      "children": [
        {
          "id": 2,
          "parentId": 1,
          "name": "公司新闻",
          "code": "company_news",
          "status": "1",
          "children": []
        }
      ]
    }
  ],
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 1.3 获取分类详情

```
GET /admin/v1/content/categories/:id
```

| 路径参数 | 类型 | 说明 |
|----------|------|------|
| `id` | uint | 分类 ID |

### 1.4 创建分类

```
POST /admin/v1/content/categories
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `parentId` | uint | 否 | 父分类 ID |
| `name` | string | 是 | 分类名称（最大 50 字符） |
| `code` | string | 否 | 分类编码（最大 50 字符） |
| `icon` | string | 否 | 图标（最大 100 字符） |
| `sort` | int | 否 | 排序 |
| `storageConfigId` | *uint | 否 | 存储配置 ID |
| `contentType` | string | 否 | 内容类型（`plaintext`/`richtext`） |
| `status` | string | 否 | 状态（`0`:禁用 `1`:启用） |
| `remark` | string | 否 | 备注 |

#### 请求示例

```json
{
  "parentId": 0,
  "name": "公司新闻",
  "code": "company_news",
  "contentType": "richtext",
  "status": "1",
  "sort": 1
}
```

### 1.5 更新分类

```
PUT /admin/v1/content/categories/:id
```

请求参数同创建分类。

### 1.6 删除分类

```
DELETE /admin/v1/content/categories/:id
```

---

## 二、文章管理

### 2.1 获取文章列表

分页查询文章列表，支持按分类、标题、发布状态、置顶、热门、推荐、作者、时间范围筛选。

```
GET /admin/v1/content/articles
```

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 当前页码 |
| `size` | int | 否 | 每页数量 |
| `categoryId` | uint | 否 | 分类 ID |
| `title` | string | 否 | 文章标题 |
| `publishStatus` | string | 否 | 发布状态（`draft`/`published`/`scheduled`） |
| `isTop` | bool | 否 | 是否置顶 |
| `isHot` | bool | 否 | 是否热门 |
| `isRecommend` | bool | 否 | 是否推荐 |
| `author` | string | 否 | 作者 |
| `startTime` | string | 否 | 开始时间 |
| `endTime` | string | 否 | 结束时间 |

### 2.2 获取文章详情

```
GET /admin/v1/content/articles/:id
```

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "id": 1,
    "categoryId": 1,
    "categoryName": "公司新闻",
    "title": "公司成立公告",
    "titleColor": "",
    "coverImage": "https://example.com/cover.jpg",
    "summary": "公司正式成立",
    "content": "<p>详细内容...</p>",
    "contentType": "richtext",
    "author": "admin",
    "source": "官方",
    "keywords": "公司,公告",
    "tags": "公告",
    "isTop": true,
    "topSort": 1,
    "isHot": false,
    "isRecommend": true,
    "allowComment": true,
    "viewCount": 100,
    "likeCount": 20,
    "commentCount": 5,
    "publishStatus": "published",
    "publishedAt": "2025-01-01T12:00:00Z",
    "scheduledAt": null,
    "createdBy": 1,
    "updatedBy": 1,
    "createdAt": "2025-01-01T12:00:00Z",
    "updatedAt": "2025-01-01T12:00:00Z"
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 2.3 创建文章

```
POST /admin/v1/content/articles
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `categoryId` | uint | 是 | 分类 ID |
| `title` | string | 是 | 文章标题（最大 200 字符） |
| `titleColor` | string | 否 | 标题颜色（最大 20 字符） |
| `coverImage` | string | 否 | 封面图 URL |
| `summary` | string | 否 | 摘要（最大 500 字符） |
| `content` | string | 否 | 正文内容 |
| `contentType` | string | 否 | 内容类型（`plaintext`/`richtext`） |
| `author` | string | 否 | 作者（最大 50 字符） |
| `source` | string | 否 | 来源（最大 100 字符） |
| `keywords` | string | 否 | 关键词（最大 200 字符） |
| `tags` | string | 否 | 标签（最大 200 字符） |
| `isTop` | bool | 否 | 是否置顶 |
| `topSort` | int | 否 | 置顶排序 |
| `isHot` | bool | 否 | 是否热门 |
| `isRecommend` | bool | 否 | 是否推荐 |
| `allowComment` | bool | 否 | 是否允许评论 |
| `publishStatus` | string | 否 | 发布状态（`draft`/`published`/`scheduled`） |
| `scheduledAt` | *string | 否 | 定时发布时间 |

#### 请求示例

```json
{
  "categoryId": 1,
  "title": "公司成立公告",
  "summary": "公司正式成立",
  "content": "<p>详细内容...</p>",
  "contentType": "richtext",
  "author": "admin",
  "publishStatus": "draft"
}
```

### 2.4 更新文章

```
PUT /admin/v1/content/articles/:id
```

请求参数同创建文章，所有字段均为可选。

### 2.5 删除文章

```
DELETE /admin/v1/content/articles/:id
```

### 2.6 发布文章

将文章状态改为已发布。

```
PUT /admin/v1/content/articles/:id/publish
```

### 2.7 撤销发布

将文章状态从已发布改回草稿。

```
PUT /admin/v1/content/articles/:id/unpublish
```

### 2.8 置顶/取消置顶

设置或取消文章置顶状态。

```
PUT /admin/v1/content/articles/:id/top
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `isTop` | bool | 是 | 是否置顶 |
| `topSort` | int | 否 | 置顶排序值 |

#### 请求示例

```json
{
  "isTop": true,
  "topSort": 1
}
```

---

## 三、Banner 分组管理

### 3.1 获取 Banner 分组列表

```
GET /admin/v1/content/banner-groups
```

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 当前页码 |
| `size` | int | 否 | 每页数量 |
| `name` | string | 否 | 分组名称 |
| `code` | string | 否 | 分组编码 |
| `description` | string | 否 | 描述 |
| `position` | string | 否 | 位置 |
| `status` | string | 否 | 状态 |

### 3.2 获取 Banner 分组详情

```
GET /admin/v1/content/banner-groups/:id
```

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "id": 1,
    "name": "首页轮播",
    "code": "home_banner",
    "description": "首页顶部轮播图",
    "position": "home_top",
    "width": 1920,
    "height": 600,
    "maxItems": 5,
    "autoPlay": true,
    "interval": 3000,
    "sort": 1,
    "storageConfigId": 1,
    "status": "1",
    "remark": ""
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 3.3 创建 Banner 分组

```
POST /admin/v1/content/banner-groups
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 分组名称（最大 100 字符） |
| `code` | string | 是 | 分组编码（最大 50 字符） |
| `description` | string | 否 | 描述（最大 255 字符） |
| `position` | string | 否 | 位置（最大 50 字符） |
| `width` | int | 否 | 宽度 |
| `height` | int | 否 | 高度 |
| `maxItems` | int | 否 | 最大项数（最小 1） |
| `autoPlay` | bool | 否 | 是否自动播放 |
| `interval` | int | 否 | 轮播间隔（最小 1000ms） |
| `sort` | int | 否 | 排序 |
| `storageConfigId` | *uint | 否 | 存储配置 ID |
| `status` | string | 否 | 状态（`0`:禁用 `1`:启用） |
| `remark` | string | 否 | 备注 |

### 3.4 更新 Banner 分组

```
PUT /admin/v1/content/banner-groups/:id
```

请求参数同创建分组，所有字段均为可选更新。

### 3.5 删除 Banner 分组

```
DELETE /admin/v1/content/banner-groups/:id
```

---

## 四、Banner 项管理

### 4.1 获取 Banner 项列表

```
GET /admin/v1/content/banner-items
```

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 当前页码 |
| `size` | int | 否 | 每页数量 |
| `groupId` | uint | 否 | 所属分组 ID |
| `title` | string | 否 | 标题 |
| `status` | string | 否 | 状态 |
| `startTime` | string | 否 | 开始时间 |
| `endTime` | string | 否 | 结束时间 |

### 4.2 获取 Banner 项详情

```
GET /admin/v1/content/banner-items/:id
```

#### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "id": 1,
    "groupId": 1,
    "groupName": "首页轮播",
    "title": "新品发布",
    "subtitle": "最新产品",
    "imageUrl": "https://example.com/banner.jpg",
    "imageAlt": "新品发布图",
    "linkType": "internal",
    "linkUrl": "/product/new",
    "linkArticleId": null,
    "linkArticleTitle": "",
    "content": "",
    "customParams": "{}",
    "sort": 1,
    "startTime": null,
    "endTime": null,
    "viewCount": 0,
    "clickCount": 0,
    "status": "1",
    "createdBy": 1,
    "updatedBy": 1,
    "createdAt": "2025-01-01T12:00:00Z",
    "updatedAt": "2025-01-01T12:00:00Z"
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 4.3 创建 Banner 项

```
POST /admin/v1/content/banner-items
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `groupId` | uint | 是 | 所属分组 ID |
| `title` | string | 是 | 标题（最大 200 字符） |
| `subtitle` | string | 否 | 副标题（最大 200 字符） |
| `imageUrl` | string | 是 | 图片 URL |
| `imageAlt` | string | 否 | 图片 Alt 文本（最大 200 字符） |
| `linkType` | string | 否 | 链接类型（`none`/`internal`/`external`/`article`） |
| `linkUrl` | string | 否 | 链接 URL |
| `linkArticleId` | *uint | 否 | 关联文章 ID |
| `content` | string | 否 | 内容 |
| `customParams` | string | 否 | 自定义参数 |
| `sort` | int | 否 | 排序 |
| `startTime` | *string | 否 | 开始时间 |
| `endTime` | *string | 否 | 结束时间 |
| `status` | string | 否 | 状态（`0`:禁用 `1`:启用） |

#### 请求示例

```json
{
  "groupId": 1,
  "title": "新品发布",
  "subtitle": "最新产品",
  "imageUrl": "https://example.com/banner.jpg",
  "linkType": "internal",
  "linkUrl": "/product/new",
  "sort": 1,
  "status": "1"
}
```

### 4.4 更新 Banner 项

```
PUT /admin/v1/content/banner-items/:id
```

请求参数同创建项，所有字段均为可选更新。

### 4.5 删除 Banner 项

```
DELETE /admin/v1/content/banner-items/:id
```
