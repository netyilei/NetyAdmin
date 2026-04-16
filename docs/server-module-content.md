# 内容管理模块详解

本文档详细介绍 NetyAdmin 内容管理模块的架构设计、使用方式和二次开发指南。

---

## 一、模块概述

内容管理模块（CMS）提供分类、文章、Banner等内容的完整管理能力，支持富文本编辑、定时发布、置顶等功能。

### 1.1 核心特性

- **分类管理**：支持树形结构的无限级分类
- **文章管理**：富文本编辑、定时发布、置顶、状态控制
- **Banner管理**：分组管理，支持图片、链接配置
- **定时任务**：文章定时自动发布

---

## 二、目录结构

```
server/internal/domain/entity/content/
├── article.go          # 文章实体
├── category.go         # 分类实体
├── banner_group.go     # Banner分组实体
└── banner_item.go      # Banner项实体

server/internal/repository/content/
├── article.go          # 文章仓储
├── category.go         # 分类仓储
├── banner_group.go     # Banner分组仓储
└── banner_item.go      # Banner项仓储

server/internal/service/content/
├── article.go          # 文章服务
├── category.go         # 分类服务
├── banner_group.go     # Banner分组服务
└── banner_item.go      # Banner项服务

server/internal/interface/admin/http/handler/v1/content/
├── article_handler.go      # 文章Handler
├── category_handler.go     # 分类Handler
├── banner_group_handler.go # Banner分组Handler
└── banner_item_handler.go  # Banner项Handler
```

---

## 三、数据模型

### 3.1 分类（categories）

```go
type Category struct {
    ID        uint           `gorm:"primarykey"`
    ParentID  uint           `gorm:"default:0;index"`               // 父分类ID，0为根
    Name      string         `gorm:"size:128;not null"`             // 分类名称
    Slug      string         `gorm:"size:128;uniqueIndex"`          // URL别名
    Sort      int            `gorm:"default:0"`                     // 排序
    Status    int8           `gorm:"default:1"`                     // 状态：1启用 2禁用
    CreatedAt int64          `gorm:"autoCreateTime"`
    UpdatedAt int64          `gorm:"autoUpdateTime"`
    DeletedAt gorm.DeletedAt `gorm:"index"`
}
```

### 3.2 文章（articles）

```go
type Article struct {
    ID          uint           `gorm:"primarykey"`
    CategoryID  uint           `gorm:"not null;index"`                // 所属分类
    Title       string         `gorm:"size:256;not null"`             // 标题
    Summary     string         `gorm:"size:512"`                      // 摘要
    Content     string         `gorm:"type:text"`                     // 内容（HTML）
    CoverImage  string         `gorm:"size:512"`                      // 封面图
    Author      string         `gorm:"size:64"`                       // 作者
    Status      int8           `gorm:"default:1"`                     // 状态：1草稿 2已发布 3已下架
    IsTop       bool           `gorm:"default:false"`                 // 是否置顶
    ViewCount   int            `gorm:"default:0"`                     // 浏览量
    PublishTime int64          `gorm:"index"`                         // 发布时间
    CreatedAt   int64          `gorm:"autoCreateTime"`
    UpdatedAt   int64          `gorm:"autoUpdateTime"`
    DeletedAt   gorm.DeletedAt `gorm:"index"`
}
```

### 3.3 Banner分组（banner_groups）

```go
type BannerGroup struct {
    ID          uint           `gorm:"primarykey"`
    Code        string         `gorm:"size:64;not null;uniqueIndex"`  // 分组编码
    Name        string         `gorm:"size:128;not null"`             // 分组名称
    Description string         `gorm:"size:256"`                      // 描述
    Status      int8           `gorm:"default:1"`                     // 状态：1启用 2禁用
    CreatedAt   int64          `gorm:"autoCreateTime"`
    UpdatedAt   int64          `gorm:"autoUpdateTime"`
    DeletedAt   gorm.DeletedAt `gorm:"index"`
}
```

### 3.4 Banner项（banner_items）

```go
type BannerItem struct {
    ID          uint           `gorm:"primarykey"`
    GroupID     uint           `gorm:"not null;index"`                // 所属分组
    Title       string         `gorm:"size:256"`                      // 标题
    ImageURL    string         `gorm:"size:512;not null"`             // 图片URL
    LinkURL     string         `gorm:"size:512"`                      // 链接URL
    Sort        int            `gorm:"default:0"`                     // 排序
    Status      int8           `gorm:"default:1"`                     // 状态：1启用 2禁用
    CreatedAt   int64          `gorm:"autoCreateTime"`
    UpdatedAt   int64          `gorm:"autoUpdateTime"`
    DeletedAt   gorm.DeletedAt `gorm:"index"`
}
```

---

## 四、API接口

### 4.1 分类管理

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/content/categories | 分类列表 |
| GET | /admin/v1/content/categories/tree | 分类树 |
| GET | /admin/v1/content/categories/:id | 分类详情 |
| POST | /admin/v1/content/categories | 创建分类 |
| PUT | /admin/v1/content/categories/:id | 更新分类 |
| DELETE | /admin/v1/content/categories/:id | 删除分类 |

### 4.2 文章管理

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/content/articles | 文章列表 |
| GET | /admin/v1/content/articles/:id | 文章详情 |
| POST | /admin/v1/content/articles | 创建文章 |
| PUT | /admin/v1/content/articles/:id | 更新文章 |
| DELETE | /admin/v1/content/articles/:id | 删除文章 |
| PUT | /admin/v1/content/articles/:id/publish | 发布文章 |
| PUT | /admin/v1/content/articles/:id/unpublish | 下架文章 |
| PUT | /admin/v1/content/articles/:id/top | 置顶/取消置顶 |

### 4.3 Banner管理

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/content/banner-groups | Banner分组列表 |
| POST | /admin/v1/content/banner-groups | 创建分组 |
| PUT | /admin/v1/content/banner-groups/:id | 更新分组 |
| DELETE | /admin/v1/content/banner-groups/:id | 删除分组 |
| GET | /admin/v1/content/banner-items | Banner项列表 |
| POST | /admin/v1/content/banner-items | 创建Banner项 |
| PUT | /admin/v1/content/banner-items/:id | 更新Banner项 |
| DELETE | /admin/v1/content/banner-items/:id | 删除Banner项 |

---

## 五、定时发布实现

### 5.1 任务逻辑

```go
// internal/job/article_publish.go

func (t *ArticlePublishTask) Run(ctx context.Context) error {
    // 查询待发布的文章（状态=草稿 AND 发布时间<=当前时间）
    articles, err := t.articleRepo.GetPendingPublish(ctx)
    if err != nil {
        return err
    }
    
    for _, article := range articles {
        if err := t.articleRepo.UpdateStatus(ctx, article.ID, 2); err != nil {
            log.Printf("发布文章 %d 失败: %v", article.ID, err)
        }
    }
    
    return nil
}
```

### 5.2 仓储查询

```go
// 获取待发布的文章
func (r *articleRepository) GetPendingPublish(ctx context.Context) ([]*entity.Article, error) {
    var articles []*entity.Article
    now := time.Now().Unix()
    
    err := r.db.WithContext(ctx).
        Where("status = ? AND publish_time <= ?", 1, now).
        Find(&articles).Error
    
    return articles, err
}
```

---

## 六、二次开发示例

### 6.1 新增文章字段

```go
// 1. 修改实体
// internal/domain/entity/content/article.go

type Article struct {
    // ... 现有字段
    
    Tags       string `gorm:"size:512"`          // 标签（逗号分隔）
    Source     string `gorm:"size:256"`          // 文章来源
    SourceURL  string `gorm:"size:512"`          // 来源链接
}

// 2. 修改DTO
// internal/interface/admin/dto/content/article.go

type CreateArticleReq struct {
    // ... 现有字段
    Tags      string `json:"tags"`
    Source    string `json:"source"`
    SourceURL string `json:"source_url"`
}

// 3. 数据库迁移
// migrations/table_content.sql

ALTER TABLE articles ADD COLUMN tags VARCHAR(512);
ALTER TABLE articles ADD COLUMN source VARCHAR(256);
ALTER TABLE articles ADD COLUMN source_url VARCHAR(512);
```

### 6.2 实现文章搜索

```go
// internal/repository/content/article.go

func (r *articleRepository) Search(ctx context.Context, keyword string, page, size int) ([]*entity.Article, int64, error) {
    var articles []*entity.Article
    var total int64
    
    query := r.db.WithContext(ctx).Model(&entity.Article{})
    
    if keyword != "" {
        query = query.Where("title LIKE ? OR content LIKE ?", 
            "%"+keyword+"%", "%"+keyword+"%")
    }
    
    query.Count(&total)
    
    err := query.Order("is_top DESC, publish_time DESC").
        Offset((page - 1) * size).
        Limit(size).
        Find(&articles).Error
    
    return articles, total, err
}
```

### 6.3 前端富文本编辑器集成

```vue
<!-- src/components/custom/toast-ui-editor.vue -->
<template>
  <div ref="editorRef" class="toast-ui-editor" />
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Editor from '@toast-ui/editor'

const props = defineProps<{
  modelValue: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const editorRef = ref<HTMLElement>()
let editor: Editor

onMounted(() => {
  editor = new Editor({
    el: editorRef.value!,
    initialEditType: 'wysiwyg',
    previewStyle: 'vertical',
    height: '500px',
    initialValue: props.modelValue,
    events: {
      change: () => {
        emit('update:modelValue', editor.getHTML())
      }
    }
  })
})

watch(() => props.modelValue, (val) => {
  if (editor && val !== editor.getHTML()) {
    editor.setHTML(val)
  }
})
</script>
```

---

## 七、最佳实践

1. **内容安全**：富文本内容需要进行XSS过滤
2. **图片处理**：建议接入CDN，支持图片压缩和水印
3. **版本控制**：重要文章支持版本历史记录
4. **SEO优化**：自动生成meta信息、URL别名
5. **缓存策略**：已发布文章可缓存，草稿实时查询

---

## 八、相关文档

- [Server架构设计](./server-architecture.md)
- [任务系统详解](./server-module-task.md)
- [存储模块详解](./server-module-storage.md)
