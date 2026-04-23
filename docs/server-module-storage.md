# 存储模块详解

本文档详细介绍 NetyAdmin 存储模块的架构设计、使用方式和二次开发指南。

---

## 一、模块概述

存储模块提供对象存储的完整管理能力，支持多存储源配置、上传凭证下发、上传记录管理。

### 1.1 核心特性

- **多存储源**：支持多个S3兼容的对象存储配置
- **默认配置**：可设置默认存储源
- **上传凭证**：为前端直传提供临时凭证
- **上传记录**：记录所有上传操作
- **驱动扩展**：支持自定义存储驱动
- **应用级存储绑定**：开放平台应用可绑定独立存储配置，未绑定时自动回退到全局默认
- **Client端上传**：开放平台应用可通过签名验证后获取上传凭证和创建上传记录

---

## 二、目录结构

```
server/internal/domain/entity/storage/
├── config.go           # 存储配置实体
└── record.go           # 上传记录实体（含AppID字段）

server/internal/repository/storage/
├── config.go           # 存储配置仓储
└── record.go           # 上传记录仓储

server/internal/service/storage/
├── config.go           # 存储配置服务
└── record.go           # 上传记录服务（含应用存储配置解析）

server/internal/pkg/storage/
├── driver.go           # 存储驱动接口
├── manager.go          # 存储管理器
└── s3_driver.go        # S3驱动实现

server/internal/interface/admin/http/handler/v1/storage/
└── storage_handler.go  # Admin端存储Handler

server/internal/interface/client/http/handler/v1/
└── storage_handler.go  # Client端存储上传Handler

server/internal/interface/client/http/router/v1/
└── storage_router.go   # Client端存储路由

server/internal/interface/client/dto/v1/
└── storage.go          # Client端存储DTO
```

---

## 三、数据模型

### 3.1 存储配置（storage_configs）

```go
type StorageConfig struct {
    ID            uint           `gorm:"primarykey"`
    Name          string         `gorm:"size:128;not null"`             // 配置名称
    Type          string         `gorm:"size:32;not null"`              // 存储类型：s3
    Endpoint      string         `gorm:"size:512;not null"`             // 服务端点
    Region        string         `gorm:"size:64"`                       // 区域
    Bucket        string         `gorm:"size:128;not null"`             // 存储桶
    AccessKey     string         `gorm:"size:256;not null"`             // AccessKey
    SecretKey     string         `gorm:"size:256;not null"`             // SecretKey
    BaseURL       string         `gorm:"size:512"`                      // 基础URL（CDN地址）
    IsDefault     bool           `gorm:"default:false"`                 // 是否默认
    Status        int8           `gorm:"default:1"`                     // 状态：1启用 2禁用
    CreatedAt     int64          `gorm:"autoCreateTime"`
    UpdatedAt     int64          `gorm:"autoUpdateTime"`
    DeletedAt     gorm.DeletedAt `gorm:"index"`
}
```

### 3.2 上传记录（upload_records）

```go
type UploadRecord struct {
    ID           uint           `gorm:"primarykey"`
    StorageID    uint           `gorm:"not null;index"`                // 存储配置ID
    FileName     string         `gorm:"size:256;not null"`             // 原始文件名
    FileKey      string         `gorm:"size:512;not null"`             // 存储Key
    FileURL      string         `gorm:"size:512;not null"`             // 访问URL
    FileSize     int64          `gorm:"not null"`                      // 文件大小（字节）
    FileType     string         `gorm:"size:64"`                       // 文件类型
    MimeType     string         `gorm:"size:128"`                      // MIME类型
    UploaderID   uint           `gorm:"index"`                         // 上传者ID
    UploaderType string         `gorm:"size:32"`                       // 上传者类型
    AppID        string         `gorm:"size:26;index"`                 // 开放平台应用ID
    CreatedAt    int64          `gorm:"autoCreateTime"`
    UpdatedAt    int64          `gorm:"autoUpdateTime"`
    DeletedAt    gorm.DeletedAt `gorm:"index"`
}
```

> **AppID 字段**：当上传来自开放平台应用时，`AppID` 记录来源应用的 `AppKey`，便于按应用统计和审计上传记录。非应用上传时该字段为空字符串。

---

## 四、存储驱动架构

### 4.1 驱动接口

```go
// Driver 存储驱动接口
type Driver interface {
    // 获取临时上传凭证（前端直传）
    GetUploadCredentials(ctx context.Context, key string, expire time.Duration) (*Credentials, error)
    
    // 生成访问URL
    GetURL(key string) string
    
    // 删除文件
    Delete(ctx context.Context, key string) error
    
    // 检查文件是否存在
    Exists(ctx context.Context, key string) (bool, error)
}

// Credentials 上传凭证
type Credentials struct {
    Endpoint  string            `json:"endpoint"`
    Bucket    string            `json:"bucket"`
    Region    string            `json:"region"`
    AccessKey string            `json:"access_key"`
    SecretKey string            `json:"secret_key"`
    Token     string            `json:"token"`
    Key       string            `json:"key"`
    Expires   int64             `json:"expires"`
    URL       string            `json:"url"`
}
```

### 4.2 驱动注册

```go
// 注册S3驱动
func init() {
    RegisterDriver("s3", func(config map[string]string) (Driver, error) {
        return NewS3Driver(config)
    })
}
```

---

## 五、API接口

### 5.1 存储配置管理

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/storage-configs | 配置列表 |
| GET | /admin/v1/storage-configs/:id | 配置详情 |
| GET | /admin/v1/storage-configs/all-enabled | 所有启用配置 |
| POST | /admin/v1/storage-configs | 创建配置 |
| PUT | /admin/v1/storage-configs | 更新配置 |
| DELETE | /admin/v1/storage-configs/:id | 删除配置 |
| PUT | /admin/v1/storage-configs/:id/default | 设为默认 |
| POST | /admin/v1/storage-configs/test-upload | 测试上传 |

### 5.2 上传凭证与记录

| Method | Path | 说明 |
|--------|------|------|
| POST | /admin/v1/storage/upload-credentials | 获取上传凭证 |
| POST | /admin/v1/storage/upload-record | 记录上传 |
| GET | /admin/v1/upload-records | 上传记录列表 |
| GET | /admin/v1/upload-records/:id | 记录详情 |
| DELETE | /admin/v1/upload-records/:id | 删除记录 |
| POST | /admin/v1/upload-records/batch-delete | 批量删除 |

### 5.3 Client 端上传接口（需开放平台签名）

| Method | Path | 说明 |
|--------|------|------|
| POST | /client/v1/storage/credentials | 获取上传凭证（自动使用应用绑定的存储配置） |
| POST | /client/v1/storage/records | 创建上传记录 |

> **存储配置解析逻辑**：Client 端接口通过签名验证中间件获取应用身份，然后按以下优先级选择存储配置：
>
> 1. 应用绑定的 `StorageID`（若 > 0）
> 2. 请求中指定的 `ConfigID`（若 > 0）
> 3. 全局默认存储配置

---

## 六、使用示例

### 6.1 获取上传凭证（前端直传）

```go
// Handler
func (h *StorageHandler) GetUploadCredentials(c *gin.Context) {
    var req dto.GetUploadCredentialsReq
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Error(c, errorx.CodeInvalidParams)
        return
    }
    
    // 生成唯一Key
    key := generateFileKey(req.FileName)
    
    // 获取凭证
    credentials, err := h.storageService.GetUploadCredentials(c.Request.Context(), key)
    if err != nil {
        response.Error(c, errorx.CodeInternalError)
        return
    }
    
    response.Success(c, credentials)
}
```

### 6.2 前端直传流程

```typescript
// 1. 获取上传凭证
const { data: credentials } = await fetchGetUploadCredentials({
  file_name: 'image.jpg',
  file_size: 1024000
})

// 2. 直传到对象存储
const formData = new FormData()
formData.append('key', credentials.key)
formData.append('policy', credentials.policy)
formData.append('signature', credentials.signature)
formData.append('file', file)

await fetch(credentials.endpoint, {
  method: 'POST',
  body: formData
})

// 3. 记录上传
await fetchRecordUpload({
  storage_id: credentials.storage_id,
  file_name: file.name,
  file_key: credentials.key,
  file_url: credentials.url,
  file_size: file.size
})
```

---

## 七、二次开发示例

### 7.1 新增存储驱动（以阿里云OSS为例）

```go
// internal/pkg/storage/oss_driver.go

package storage

import (
    "context"
    "time"
    
    "github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type OSSDriver struct {
    client *oss.Client
    bucket *oss.Bucket
    config map[string]string
}

func NewOSSDriver(config map[string]string) (*OSSDriver, error) {
    client, err := oss.New(
        config["endpoint"],
        config["access_key"],
        config["secret_key"],
    )
    if err != nil {
        return nil, err
    }
    
    bucket, err := client.Bucket(config["bucket"])
    if err != nil {
        return nil, err
    }
    
    return &OSSDriver{
        client: client,
        bucket: bucket,
        config: config,
    }, nil
}

func (d *OSSDriver) GetUploadCredentials(ctx context.Context, key string, expire time.Duration) (*Credentials, error) {
    // 生成临时凭证
    // ...
}

func (d *OSSDriver) GetURL(key string) string {
    baseURL := d.config["base_url"]
    if baseURL == "" {
        baseURL = d.config["endpoint"]
    }
    return baseURL + "/" + key
}

func (d *OSSDriver) Delete(ctx context.Context, key string) error {
    return d.bucket.DeleteObject(key)
}

func (d *OSSDriver) Exists(ctx context.Context, key string) (bool, error) {
    return d.bucket.IsObjectExist(key)
}
```

### 7.2 注册新驱动

```go
// internal/pkg/storage/oss_driver.go

func init() {
    RegisterDriver("oss", func(config map[string]string) (Driver, error) {
        return NewOSSDriver(config)
    })
}
```

### 7.3 文件Key生成策略

```go
// internal/pkg/storage/manager.go

func generateFileKey(originalName string) string {
    ext := filepath.Ext(originalName)
    date := time.Now().Format("2006/01/02")
    uuid := uuid.New().String()
    
    return fmt.Sprintf("uploads/%s/%s%s", date, uuid, ext)
}
```

---

## 八、安全配置

### 8.1 敏感信息处理

- SecretKey 数据库加密存储
- API返回时脱敏处理
- 临时凭证有效期限制（建议5分钟）

### 8.2 上传限制

```go
// 文件类型白名单
var AllowedMimeTypes = []string{
    "image/jpeg",
    "image/png",
    "image/gif",
    "application/pdf",
}

// 文件大小限制（10MB）
const MaxFileSize = 10 * 1024 * 1024
```

---

## 九、最佳实践

1. **CDN加速**：生产环境配置CDN域名作为BaseURL
2. **文件命名**：使用UUID避免文件名冲突
3. **目录组织**：按日期组织上传文件，便于管理
4. **定期清理**：结合任务系统清理无效上传记录
5. **监控告警**：监控存储桶容量和流量

---

## 十、相关文档

- [Server架构设计](./server-architecture.md)
- [任务系统详解](./server-module-task.md)
