# 系统配置 API

本模块提供系统配置的获取与更新功能，以及邮件发送测试功能。系统配置采用分组管理，支持多种配置类型（字符串、数字、布尔、JSON 等）。

## 接口概览

| 方法 | 路径 | 认证级别 | 说明 |
|------|------|----------|------|
| GET | `/admin/v1/system/configs` | 公开 | 获取配置分组（前端初始化） |
| PUT | `/admin/v1/system/configs` | 权限 | 更新系统配置 |
| POST | `/admin/v1/system/test-email` | 权限 | 测试邮件发送 |

---

## 1. 获取配置分组

获取指定配置分组下的所有配置项。该接口为公开接口，用于前端应用初始化时加载所需配置（如站点名称、Logo 等）。

```
GET /admin/v1/system/configs
```

### 认证级别

公开接口（无需 Token）

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `group` | string | 否 | 配置分组名称。不指定时返回所有公开配置 |

### 请求示例

```
GET /admin/v1/system/configs?group=site_config
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "site_config": {
      "site_name": "NetyAdmin",
      "site_logo": "https://example.com/logo.png",
      "site_description": "NetyAdmin 管理后台",
      "icp": "京ICP备12345678号",
      "captcha_config": {
        "admin_login_enabled": "0"
      }
    }
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `data` | object | 以分组名为 Key、配置项键值对为 Value 的对象 |

> 仅返回 `is_public` 为 `1` 的配置项。敏感配置（如密钥）不会通过此接口暴露。

---

## 2. 更新系统配置

更新指定配置分组下的配置项。

```
PUT /admin/v1/system/configs
```

### 认证级别

权限接口（JWT + RBAC）

### 请求参数

请求体为配置分组更新对象，以分组名为 Key，配置项键值对为 Value：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `<group_name>` | object | 是 | 配置分组名作为 Key，值为该分组下配置项的键值对 |

### 请求示例

```json
{
  "site_config": {
    "site_name": "NetyAdmin Pro",
    "site_logo": "https://example.com/new-logo.png",
    "site_description": "NetyAdmin 管理后台 Pro 版",
    "icp": "京ICP备87654321号"
  },
  "captcha_config": {
    "admin_login_enabled": "1"
  }
}
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "配置更新成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 配置项说明

系统主要配置分组包括：

| 分组 | 说明 | 主要配置项 |
|------|------|------------|
| `site_config` | 站点配置 | `site_name`、`site_logo`、`site_description`、`icp` |
| `captcha_config` | 验证码配置 | `admin_login_enabled`（管理员登录验证码开关） |
| `email_config` | 邮件配置 | `smtp_host`、`smtp_port`、`smtp_username`、`smtp_password`、`smtp_from` |

> 更新配置后，系统会自动刷新内存中的配置缓存。

---

## 3. 测试邮件发送

使用当前系统邮件配置发送测试邮件，用于验证邮件配置是否正确。

```
POST /admin/v1/system/test-email
```

### 认证级别

权限接口（JWT + RBAC）

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `to` | string | 是 | 收件人邮箱地址 |

### 请求示例

```json
{
  "to": "test@example.com"
}
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "测试邮件发送成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 常见错误码

| 错误码 | 说明 |
|--------|------|
| `100001` | 参数错误（收件人邮箱为空或格式不正确） |
| `101203` | 邮件发送失败（SMTP 配置错误或网络异常） |
