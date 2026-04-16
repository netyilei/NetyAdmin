# 验证码模块详解

本文档详细介绍 NetyAdmin 验证码模块的架构设计、配置方式和二次开发指南。

---

## 一、模块概述

验证码模块基于 `mojocn/base64Captcha` 实现，支持多种验证码类型和存储方案，可在管理后台动态配置。

### 1.1 核心特性

- **多种验证码类型**：数字、字符、算术
- **多存储方案**：Redis / 本地内存 / PostgreSQL
- **动态配置**：支持后台实时调整参数
- **安全性**：一次性使用，防止重放攻击

---

## 二、目录结构

```
server/internal/pkg/captcha/
├── captcha.go          # 验证码核心逻辑
└── store.go            # 存储适配器
```

---

## 三、架构设计

### 3.1 存储适配器模式

```go
// Store 验证码存储接口
type Store interface {
    Set(id string, value string) error
    Get(id string, clear bool) string
    Verify(id, answer string, clear bool) bool
}
```

支持三种存储实现：

| 存储类型 | 适用场景 | 特点 |
|---------|---------|------|
| RedisStore | 集群部署 | 共享存储，支持过期时间 |
| MemoryStore | 单机部署 | 零依赖，进程内存储 |
| DBStore | 特殊需求 | 持久化，可审计 |

### 3.2 验证码类型

```go
// 支持的验证码类型
const (
    CaptchaTypeDigit   = "digit"    // 纯数字
    CaptchaTypeString  = "string"   // 字母+数字
    CaptchaTypeMath    = "math"     // 算术题
)
```

---

## 四、配置说明

### 4.1 配置文件（config.toml）

```toml
[captcha]
# 存储类型：redis / memory / db
store_type = "redis"

# 验证码类型：digit / string / math
type = "string"

# 长度（数字/字符型）
length = 4

# 图片宽度
width = 240

# 图片高度
height = 80

# 过期时间（秒）
expire_seconds = 300

# 是否启用（总开关）
enabled = true
```

### 4.2 动态配置（sys_configs）

通过管理后台可实时调整以下配置：

| 配置项 | Group | Key | 说明 |
|--------|-------|-----|------|
| 验证码类型 | captcha | type | digit/string/math |
| 长度 | captcha | length | 4-6 |
| 宽度 | captcha | width | 像素 |
| 高度 | captcha | height | 像素 |
| 过期时间 | captcha | expire_seconds | 秒 |
| 启用状态 | captcha | enabled | true/false |

---

## 五、API接口

### 5.1 生成验证码

```http
GET /admin/v1/common/captcha
```

**响应示例**：

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "captcha_id": "abc123",
    "captcha_img": "data:image/png;base64,iVBORw0KG..."
  },
  "request_id": "req-xxx"
}
```

### 5.2 登录时使用

```http
POST /admin/v1/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "admin123",
  "captcha_id": "abc123",
  "captcha_code": "a1b2"
}
```

---

## 六、二次开发示例

### 6.1 自定义验证码类型

```go
// internal/pkg/captcha/captcha.go

// 新增中文验证码类型
func NewChineseCaptcha(store Store, width, height int) *base64Captcha.Captcha {
    driver := &base64Captcha.DriverChinese{
        Height:          height,
        Width:           width,
        NoiseCount:      5,
        ShowLineOptions: base64Captcha.OptionShowHollowLine,
        Length:          2,
        Source:          "一二三四五六七八九十",
    }
    return base64Captcha.NewCaptcha(driver, store)
}
```

### 6.2 自定义存储实现

```go
// internal/pkg/captcha/store.go

// RedisStore Redis存储实现
type RedisStore struct {
    client *redis.Client
    prefix string
    expire time.Duration
}

func NewRedisStore(client *redis.Client, prefix string, expire time.Duration) Store {
    return &RedisStore{
        client: client,
        prefix: prefix,
        expire: expire,
    }
}

func (s *RedisStore) Set(id string, value string) error {
    key := s.prefix + ":captcha:" + id
    return s.client.Set(context.Background(), key, value, s.expire).Err()
}

func (s *RedisStore) Get(id string, clear bool) string {
    key := s.prefix + ":captcha:" + id
    ctx := context.Background()
    
    val, err := s.client.Get(ctx, key).Result()
    if err != nil {
        return ""
    }
    
    if clear {
        s.client.Del(ctx, key)
    }
    
    return val
}

func (s *RedisStore) Verify(id, answer string, clear bool) bool {
    value := s.Get(id, clear)
    return strings.EqualFold(value, answer)
}
```

### 6.3 在Handler中使用

```go
// internal/interface/admin/http/handler/v1/auth/auth.go

func (h *AuthHandler) Login(c *gin.Context) {
    var req dto.LoginReq
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Error(c, errorx.CodeInvalidParams)
        return
    }
    
    // 验证码校验
    if h.captchaConfig.Enabled {
        if !h.captchaStore.Verify(req.CaptchaID, req.CaptchaCode, true) {
            response.Error(c, errorx.CodeCaptchaWrong)
            return
        }
    }
    
    // 继续登录逻辑...
}
```

---

## 七、状态码

| Code | 常量 | 说明 |
|------|------|------|
| 100009 | CodeCaptchaWrong | 验证码错误 |
| 100010 | CodeCaptchaRequired | 验证码必填 |

---

## 八、相关文档

- [Server架构设计](./server-architecture.md)
- [状态码规范](./status-codes.md)
