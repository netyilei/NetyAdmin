# 任务管理 API

本模块提供定时任务的管理功能，包括任务列表查询、手动执行、启停、重载、更新配置及查看任务日志。

## 接口概览

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/admin/v1/system/tasks` | 获取任务列表 |
| POST | `/admin/v1/system/tasks/:name/run` | 手动执行任务 |
| POST | `/admin/v1/system/tasks/:name/start` | 启动任务 |
| POST | `/admin/v1/system/tasks/:name/stop` | 停止任务 |
| POST | `/admin/v1/system/tasks/:name/reload` | 重载任务配置 |
| PUT | `/admin/v1/system/tasks/update` | 更新任务配置 |
| GET | `/admin/v1/system/tasks/logs` | 获取任务日志列表 |

---

## 1. 获取任务列表

获取所有已注册的定时任务列表。

```
GET /admin/v1/system/tasks
```

### 认证级别

权限接口（JWT + RBAC）

### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": [
    {
      "name": "clean_expired_tokens",
      "displayName": "清理过期Token",
      "type": "cron",
      "spec": "0 */6 * * *",
      "status": "running",
      "nextRunTime": "2025-01-01T18:00:00Z",
      "lastRunTime": "2025-01-01T12:00:00Z",
      "lastRunStatus": "success",
      "description": "定期清理过期的访问令牌"
    }
  ],
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 任务名称（唯一标识） |
| `displayName` | string | 显示名称 |
| `type` | string | 任务类型（`cron`） |
| `spec` | string | Cron 表达式 |
| `status` | string | 任务状态（`running`/`stopped`/`error`） |
| `nextRunTime` | string | 下次执行时间 |
| `lastRunTime` | string | 上次执行时间 |
| `lastRunStatus` | string | 上次执行状态（`success`/`failed`） |
| `description` | string | 任务描述 |

---

## 2. 手动执行任务

根据任务名称立即执行一次任务。

```
POST /admin/v1/system/tasks/:name/run
```

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 任务名称 |

### 请求示例

```
POST /admin/v1/system/tasks/clean_expired_tokens/run
Authorization: Bearer <token>
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "任务执行成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

---

## 3. 启动任务

根据任务名称启动已停止的任务。

```
POST /admin/v1/system/tasks/:name/start
```

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 任务名称 |

### 响应示例

```json
{
  "code": "100000",
  "msg": "任务启动成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

---

## 4. 停止任务

根据任务名称停止正在运行的任务。

```
POST /admin/v1/system/tasks/:name/stop
```

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 任务名称 |

### 响应示例

```json
{
  "code": "100000",
  "msg": "任务停止成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

---

## 5. 重载任务配置

根据任务名称重新加载任务配置（如 Cron 表达式等）。

```
POST /admin/v1/system/tasks/:name/reload
```

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 任务名称 |

### 响应示例

```json
{
  "code": "100000",
  "msg": "任务重载成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

---

## 6. 更新任务配置

更新指定任务的配置信息（如 Cron 表达式、显示名称等）。

```
PUT /admin/v1/system/tasks/update
```

### 请求参数

请求体为任务配置对象，包含以下字段：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 任务名称 |
| `displayName` | string | 否 | 显示名称 |
| `spec` | string | 否 | Cron 表达式 |
| `description` | string | 否 | 任务描述 |

### 请求示例

```json
{
  "name": "clean_expired_tokens",
  "displayName": "清理过期Token",
  "spec": "0 */3 * * *",
  "description": "每3小时清理一次过期Token"
}
```

### 响应示例

```json
{
  "code": "100000",
  "msg": "任务配置更新成功",
  "data": null,
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

---

## 7. 获取任务日志列表

获取任务执行日志列表。

```
GET /admin/v1/system/tasks/logs
```

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `current` | int | 否 | 页码 |
| `size` | int | 否 | 每页数量 |
| `name` | string | 否 | 任务名称 |
| `status` | string | 否 | 执行状态 |

### 响应示例

```json
{
  "code": "100000",
  "msg": "",
  "data": {
    "records": [
      {
        "id": 1,
        "taskName": "clean_expired_tokens",
        "status": "success",
        "startTime": "2025-01-01T12:00:00Z",
        "endTime": "2025-01-01T12:00:05Z",
        "duration": "5s",
        "result": "清理了 15 条过期 Token",
        "error": ""
      }
    ],
    "current": 1,
    "size": 10,
    "total": 1
  },
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```
