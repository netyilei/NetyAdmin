# 内容模块 API

> 本文档包含分类树、文章列表/详情、Banner 等内容相关接口。所有接口均需开放平台签名，部分接口需额外携带用户 JWT Token。

---

## 一、接口总览

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | /client/v1/content/categories/tree | 签名 | 获取分类树 |
| GET | /client/v1/content/articles | 签名 | 文章列表 |
| GET | /client/v1/content/article/:id | 签名 | 文章详情 |
| POST | /client/v1/content/article/:id/like | 签名 | 点赞文章 |
| GET | /client/v1/content/banners/:code | 签名 | 获取 Banner 组 |
| POST | /client/v1/content/banners/:id/click | 签名 | 记录 Banner 点击 |

---

## 二、获取分类树

获取所有内容分类的树形结构。

```
GET /client/v1/content/categories/tree
```

**权限**：开放平台签名

**请求参数**：无

**响应示例**：

```json
{
  "code": "100000",
  "data": [
    {
      "id": 1,
      "parentId": 0,
      "name": "技术文章",
      "code": "tech",
      "icon": "code",
      "contentType": "article",
      "children": [
        {
          "id": 2,
          "parentId": 1,
          "name": "前端开发",
          "code": "frontend",
          "icon": "layout",
          "contentType": "article",
          "children": []
        }
      ]
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 分类 ID |
| parentId | uint | 父级分类 ID，0 表示顶级 |
| name | string | 分类名称 |
| code | string | 分类编码 |
| icon | string | 图标标识 |
| contentType | string | 内容类型 |
| children | array | 子分类列表（递归结构） |

**可能错误码**：`100005`（服务器内部错误）

---

## 三、文章列表

根据分类 ID 获取已发布的文章列表，支持关键词搜索。

```
GET /client/v1/content/articles
```

**权限**：开放平台签名

**请求参数**（Query）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| categoryId | uint | 是 | 分类 ID（会包含该分类的所有子分类文章） |
| page | int | 否 | 页码，默认 1 |
| pageSize | int | 否 | 每页条数，默认 10 |
| keyword | string | 否 | 搜索关键词 |

**请求示例**：

```
GET /client/v1/content/articles?categoryId=1&page=1&pageSize=10&keyword=Vue
```

**响应示例**：

```json
{
  "code": "100000",
  "data": {
    "records": [
      {
        "id": 1,
        "categoryId": 1,
        "categoryName": "技术文章",
        "title": "Vue 3 组合式 API 入门",
        "titleColor": "",
        "coverImage": "https://cdn.example.com/cover.jpg",
        "summary": "本文介绍 Vue 3 组合式 API 的基本用法...",
        "contentType": "article",
        "author": "管理员",
        "source": "原创",
        "isTop": false,
        "isHot": true,
        "isRecommend": true,
        "viewCount": 1024,
        "likeCount": 56,
        "commentCount": 12,
        "publishedAt": "2025-01-15T10:00:00Z",
        "createdAt": "2025-01-14T08:00:00Z"
      }
    ],
    "current": 1,
    "size": 10,
    "total": 42
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 文章 ID |
| categoryId | uint | 所属分类 ID |
| categoryName | string | 所属分类名称 |
| title | string | 标题 |
| titleColor | string | 标题颜色 |
| coverImage | string | 封面图 URL |
| summary | string | 摘要 |
| contentType | string | 内容类型：`article` / `video` / `gallery` 等 |
| author | string | 作者 |
| source | string | 来源 |
| isTop | boolean | 是否置顶 |
| isHot | boolean | 是否热门 |
| isRecommend | boolean | 是否推荐 |
| viewCount | int | 浏览数 |
| likeCount | int | 点赞数 |
| commentCount | int | 评论数 |
| publishedAt | string | 发布时间（ISO 8601） |
| createdAt | string | 创建时间（ISO 8601） |

**可能错误码**：`100001`（categoryId 必填）

---

## 四、文章详情

获取单篇已发布文章的完整内容。

```
GET /client/v1/content/article/:id
```

**权限**：开放平台签名

**请求参数**（Path）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | uint | 是 | 文章 ID |

**响应示例**：

```json
{
  "code": "100000",
  "data": {
    "id": 1,
    "categoryId": 1,
    "categoryName": "技术文章",
    "title": "Vue 3 组合式 API 入门",
    "titleColor": "",
    "coverImage": "https://cdn.example.com/cover.jpg",
    "summary": "本文介绍 Vue 3 组合式 API 的基本用法...",
    "content": "<p>完整的 HTML 正文内容...</p>",
    "contentType": "article",
    "author": "管理员",
    "source": "原创",
    "keywords": "Vue3,组合式API,Composition API",
    "tags": "Vue3,前端,教程",
    "isTop": false,
    "isHot": true,
    "isRecommend": true,
    "allowComment": true,
    "viewCount": 1025,
    "likeCount": 56,
    "commentCount": 12,
    "publishedAt": "2025-01-15T10:00:00Z",
    "createdAt": "2025-01-14T08:00:00Z"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 文章 ID |
| categoryId | uint | 所属分类 ID |
| categoryName | string | 所属分类名称 |
| title | string | 标题 |
| titleColor | string | 标题颜色 |
| coverImage | string | 封面图 URL |
| summary | string | 摘要 |
| content | string | 正文 HTML |
| contentType | string | 内容类型 |
| author | string | 作者 |
| source | string | 来源 |
| keywords | string | SEO 关键词（逗号分隔） |
| tags | string | 标签（逗号分隔） |
| isTop | boolean | 是否置顶 |
| isHot | boolean | 是否热门 |
| isRecommend | boolean | 是否推荐 |
| allowComment | boolean | 是否允许评论 |
| viewCount | int | 浏览数 |
| likeCount | int | 点赞数 |
| commentCount | int | 评论数 |
| publishedAt | string | 发布时间（ISO 8601） |
| createdAt | string | 创建时间（ISO 8601） |

> **注意**：每次获取详情会自动增加浏览数（viewCount +1）。

**可能错误码**：`100001`（无效的 ID）、`100004`（文章不存在或未发布）

---

## 五、点赞文章

对指定文章进行点赞，浏览数 +1。

```
POST /client/v1/content/article/:id/like
```

**权限**：开放平台签名

**请求参数**（Path）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | uint | 是 | 文章 ID |

**请求 Body**：无

**响应示例**：

```json
{
  "code": "100000",
  "data": null
}
```

**可能错误码**：`100001`（无效的 ID）、`100004`（文章不存在）

---

## 六、获取 Banner 组

根据 Banner 组编码获取当前有效的 Banner 列表。仅返回在有效时间范围内的 Banner 条目。

```
GET /client/v1/content/banners/:code
```

**权限**：开放平台签名

**请求参数**（Path）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| code | string | 是 | Banner 组编码 |

**响应示例**：

```json
{
  "code": "100000",
  "data": {
    "id": 1,
    "name": "首页轮播",
    "code": "home_banner",
    "description": "首页顶部轮播图",
    "position": "home_top",
    "width": 1200,
    "height": 400,
    "autoPlay": true,
    "interval": 5000,
    "banners": [
      {
        "id": 1,
        "title": "春季促销",
        "subtitle": "全场 8 折",
        "imageUrl": "https://cdn.example.com/banner1.jpg",
        "imageAlt": "春季促销活动",
        "linkType": "url",
        "linkUrl": "https://example.com/promotion",
        "content": "",
        "customParams": "",
        "sort": 1
      }
    ]
  }
}
```

**Banner 组字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | Banner 组 ID |
| name | string | 组名称 |
| code | string | 组编码 |
| description | string | 描述 |
| position | string | 展示位置标识 |
| width | int | 建议宽度 |
| height | int | 建议高度 |
| autoPlay | boolean | 是否自动播放 |
| interval | int | 自动播放间隔（毫秒） |
| banners | array | Banner 条目列表 |

**Banner 条目字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | Banner 条目 ID |
| title | string | 标题 |
| subtitle | string | 副标题 |
| imageUrl | string | 图片 URL |
| imageAlt | string | 图片 Alt 文本 |
| linkType | string | 链接类型：`url` / `article` / `page` / `none` |
| linkUrl | string | 跳转链接 |
| content | string | 富文本内容 |
| customParams | string | 自定义参数（JSON 字符串） |
| sort | int | 排序值，越小越靠前 |

> **注意**：仅返回当前时间在有效时间范围内的 Banner 条目。

**可能错误码**：`100001`（code 不能为空）、`100004`（Banner 组不存在）

---

## 七、记录 Banner 点击

记录用户点击 Banner 的行为，点击数 +1。

```
POST /client/v1/content/banners/:id/click
```

**权限**：开放平台签名

**请求参数**（Path）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | uint | 是 | Banner 条目 ID |

**请求 Body**：无

**响应示例**：

```json
{
  "code": "100000",
  "data": null
}
```

**可能错误码**：`100001`（无效的 ID）、`100004`（Banner 条目不存在）
