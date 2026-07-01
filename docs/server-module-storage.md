# 存储模块详解

本文档详细介绍 NetyAdmin 存储模块的架构设计、使用方式和二次开发指南。

---

## 一、模块概述

存储模块提供对象存储的完整管理能力，支持多存储源配置、上传凭证下发、上传记录管理。

### 1.1 核心特性

- **统一 S3 协议**：基于 [minio-go v7.2.1](https://github.com/minio/minio-go) 实现，天然兼容所有 S3 协议存储（AWS S3 / 阿里云 OSS / 腾讯云 COS / 华为云 OBS / 七牛云 / MinIO / Cloudflare R2 等），无需为每个云厂商维护适配代码。
- **多存储源**：支持同时配置多个存储源，可设置默认源。
- **预签名直传**：为前端提供预签名 URL 直传，减轻服务端流量。
- **上传记录**：记录所有上传操作，含状态机闭环（凭证签发→上传成功通知）。
- **驱动扩展**：面向接口设计，支持自定义驱动。
- **应用级存储绑定**：开放平台应用可绑定独立存储配置，未绑定时回退到全局默认。

---

## 二、目录结构

```
server/internal/domain/entity/storage/
├── config.go           # 存储配置实体（含 Provider 字段标识云厂商）
└── record.go           # 上传记录实体（含状态机字段 status/secret/expires_at）

server/internal/repository/storage/
├── config.go           # 存储配置仓储
└── record.go           # 上传记录仓储

server/internal/service/storage/
├── config.go           # 存储配置服务（创建/更新/测试上传）
└── record.go           # 上传记录服务（凭证签发/上传通知闭环）

server/internal/pkg/storage/
├── driver.go           # Driver 接口、Config、Provider 类型定义
├── minio_driver.go     # 基于 minio-go 的 S3 兼容驱动实现（唯一实现）
└── manager.go          # 存储管理器（多驱动注册/查找）+ 工具函数

server/internal/interface/admin/http/handler/v1/storage/
└── storage_handler.go  # Admin 端存储 Handler

server/internal/interface/client/http/handler/v1/
└── storage_handler.go  # Client 端存储上传 Handler
```

---

## 三、存储驱动架构

### 3.1 驱动接口

```go
// Driver 对象存储驱动接口（面向 S3 兼容协议抽象）。
// 当前唯一实现为基于 minio-go 的 minioDriver。
type Driver interface {
    Upload(ctx, key, reader, size, contentType) (*UploadResult, error)
    UploadFile(ctx, key, filePath, contentType) (*UploadResult, error)
    Download(ctx, key) (io.ReadCloser, *ObjectInfo, error)
    Delete(ctx, key) error
    DeleteMultiple(ctx, keys) error
    Exists(ctx, key) (bool, error)
    GetObjectInfo(ctx, key) (*ObjectInfo, error)
    GetPresignedUploadURL(ctx, key, contentType, expires) (string, error)
    GetPresignedDownloadURL(ctx, key, expires) (string, error)
    ListObjects(ctx, prefix, maxKeys) ([]*ObjectInfo, error)
    Copy(ctx, srcKey, destKey) error
}
```

### 3.2 Provider 类型

`Provider` 仅用于路径风格判断（MinIO/自定义 endpoint 需 path-style 寻址），**不再硬编码各云厂商的 endpoint**。用户在存储配置中直接填写完整 endpoint（如 `https://oss-cn-hangzhou.aliyuncs.com`），minio-go 自动处理兼容性。

```go
const (
    ProviderMinio  Provider = "minio"  // 需 path-style 寻址
    ProviderCustom Provider = "custom" // 需 path-style 寻址
)

// 其他云厂商（aws/aliyun/tencent 等）使用 BucketLookupAuto，
// 由 minio-go 根据 endpoint 自动判断寻址方式。
```

> **注意**：云厂商的完整清单（aliyun/tencent/huawei/qiniu/aws/cloudflare）由 entity 层 `StorageProvider` 常量维护，用于前端展示和校验，避免与 pkg/storage 重复定义。

### 3.3 驱动工厂

```go
// wire.go 中注册
storageMgr := storagePkg.NewManager(storagePkg.NewMinioDriverFactory())
```

---

## 四、配置说明

### 4.1 存储配置字段（storage_config 表）

创建存储配置时，**Endpoint 为必填**（用户需知道自己的存储端点地址）：

| 字段 | 说明 | 示例 |
|------|------|------|
| Provider | 云厂商标识（用于前端展示） | aws / aliyun / tencent / minio / custom |
| Endpoint | **完整端点（含协议）** | `https://s3.amazonaws.com` |
| Region | 区域（可为空，minio-go 自动处理） | `us-east-1` |
| Bucket | 存储桶名 | `my-bucket` |
| AccessKey | 访问密钥 ID | — |
| SecretKey | 访问密钥（加密存储） | — |
| Domain | 自定义域名（可选，用于生成访问 URL） | `https://cdn.example.com` |

### 4.2 常见云厂商 Endpoint 参考

| 云厂商 | Endpoint 格式 |
|--------|--------------|
| AWS S3 | `https://s3.<region>.amazonaws.com` |
| 阿里云 OSS | `https://oss-<region>.aliyuncs.com` |
| 腾讯云 COS | `https://cos.<region>.myqcloud.com` |
| 华为云 OBS | `https://obs.<region>.myhuaweicloud.com` |
| Cloudflare R2 | `https://<account>.r2.cloudflarestorage.com` |
| MinIO | `http://localhost:9000`（自建） |

---

## 五、API 接口

### 5.1 存储配置管理（Admin）

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/storage-configs | 配置列表 |
| POST | /admin/v1/storage-configs | 创建配置（endpoint 必填） |
| PUT | /admin/v1/storage-configs | 更新配置 |
| DELETE | /admin/v1/storage-configs/:id | 删除配置 |
| PUT | /admin/v1/storage-configs/:id/default | 设为默认 |
| POST | /admin/v1/storage-configs/test-upload | 测试上传 |

### 5.2 上传凭证与记录

| Method | Path | 说明 |
|--------|------|------|
| POST | /admin/v1/storage/upload-credentials | 获取预签名上传 URL |
| POST | /admin/v1/storage/upload-record | 上传成功通知（状态机闭环） |
| GET | /admin/v1/upload-records | 上传记录列表 |
| DELETE | /admin/v1/upload-records/:id | 删除记录 |

### 5.3 Client 端上传（需开放平台签名）

| Method | Path | 说明 |
|--------|------|------|
| POST | /client/v1/storage/credentials | 获取上传凭证（应用绑定存储优先） |
| POST | /client/v1/storage/records | 创建上传记录 |

---

## 六、上传凭证状态机闭环

为防止前端只传 recordID 伪造上传成功，上传记录采用状态机 + HMAC 签名校验：

```
[凭证签发] pending → 前端持签名直传 → [上传通知] uploaded
                                        ↓（超期未通知）
                                     expired（定时任务标记）
```

- 凭证签发时生成 HMAC 签名（`secret` 字段），与 recordID/objectKey/source 等绑定
- 上传通知时校验签名，任一字段篡改都会失败
- 超期未通知的 pending 记录由定时任务标记为 expired

---

## 七、二次开发

### 7.1 自定义存储驱动

实现 `Driver` 接口并注册工厂即可：

```go
// internal/pkg/storage/my_driver.go

type MyDriver struct { /* ... */ }

func (d *MyDriver) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (*UploadResult, error) {
    // 实现上传逻辑
}

// ... 实现其余 Driver 接口方法

type MyDriverFactory struct{}
func (f *MyDriverFactory) Create(config *Config) (Driver, error) {
    return NewMyDriver(config)
}
```

在 `wire.go` 中替换工厂：
```go
storageMgr := storagePkg.NewManager(storagePkg.NewMyDriverFactory())
```

---

## 八、相关文档

- [Server 架构设计](./server-architecture.md)
- [任务系统详解](./server-module-task.md)
- minio-go 官方文档：https://github.com/minio/minio-go
