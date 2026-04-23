# API 管理指南

本文档详细介绍 NetyAdmin 系统中 API 的管理方式，包括后端路由定义、前端接口封装，以及如何新增和维护 API。

---

## 一、API 架构概览

### 1.1 设计原则

- **版本控制**：API 按版本划分（当前为 v1）
- **端隔离**：Admin 端、Client 端 API 物理隔离
- **权限分级**：公开接口、JWT 接口、RBAC 接口三级权限
- **RESTful 风格**：遵循 RESTful 设计规范

### 1.2 命名空间

| 端 | 前缀 | 示例 |
|---|---|---|
| Admin | `/admin/v1` | `/admin/v1/system/admins` |
| Client | `/client/v1` | `/client/v1/user/profile` |

---

## 二、后端 API 管理

### 2.1 路由注册位置

```
server/internal/interface/admin/http/router/v1/
├── admin.go          # 管理员路由
├── auth.go           # 认证路由
├── content.go        # 内容管理路由
├── log.go            # 日志路由
├── router.go         # 路由聚合入口
├── storage.go        # 存储路由
└── system.go         # 系统管理路由
```

### 2.2 权限级别

```go
// 1. 公开接口（无需登录）
public := router.Group("/admin/v1")
{
    public.POST("/auth/login", authHandler.Login)
    public.POST("/auth/refreshToken", authHandler.RefreshToken)
}

// 2. JWT 接口（需登录，不走 RBAC）
auth := router.Group("/admin/v1")
auth.Use(middleware.JWT())
{
    auth.GET("/auth/getUserInfo", authHandler.GetUserInfo)
    auth.GET("/route/getUserRoutes", routeHandler.GetUserRoutes)
}

// 3. RBAC 接口（需登录 + 权限）
permission := router.Group("/admin/v1")
permission.Use(middleware.JWT(), middleware.RBAC())
{
    permission.GET("/system/admins", adminHandler.List)
    permission.POST("/system/admins", adminHandler.Create)
}
```

### 2.3 新增 API 步骤

#### 步骤1：定义 DTO

```go
// internal/interface/admin/dto/order/order.go

package order

// CreateOrderReq 创建订单请求
type CreateOrderReq struct {
    ProductID uint    `json:"product_id" binding:"required"`
    Quantity  int     `json:"quantity" binding:"required,min=1"`
    AddressID uint    `json:"address_id" binding:"required"`
    Remark    string  `json:"remark"`
}

// OrderResp 订单响应
type OrderResp struct {
    ID         uint    `json:"id"`
    OrderNo    string  `json:"order_no"`
    TotalPrice float64 `json:"total_price"`
    Status     int8    `json:"status"`
    CreatedAt  int64   `json:"created_at"`
}

// ListOrderReq 订单列表请求
type ListOrderReq struct {
    Status int    `form:"status"`
    Page   int    `form:"page,default=1"`
    Size   int    `form:"size,default=20"`
}
```

#### 步骤2：创建 Handler

```go
// internal/interface/admin/http/handler/v1/order/order_handler.go

package order

import (
    "net/http"
    "server/internal/interface/admin/dto/order"
    "server/internal/pkg/errorx"
    "server/internal/pkg/response"
    "server/internal/service/order"

    "github.com/gin-gonic/gin"
)

type OrderHandler struct {
    service order.OrderService
}

func NewOrderHandler(service order.OrderService) *OrderHandler {
    return &OrderHandler{service: service}
}

// List 订单列表
func (h *OrderHandler) List(c *gin.Context) {
    var req order.ListOrderReq
    if err := c.ShouldBindQuery(&req); err != nil {
        response.Error(c, errorx.CodeInvalidParams)
        return
    }

    result, total, err := h.service.List(c.Request.Context(), req)
    if err != nil {
        response.Error(c, errorx.CodeInternalError)
        return
    }

    response.Success(c, gin.H{
        "list":  result,
        "total": total,
    })
}

// Create 创建订单
func (h *OrderHandler) Create(c *gin.Context) {
    var req order.CreateOrderReq
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Error(c, errorx.CodeInvalidParams)
        return
    }

    result, err := h.service.Create(c.Request.Context(), req)
    if err != nil {
        response.Error(c, errorx.CodeInternalError)
        return
    }

    response.Success(c, result)
}

// Get 订单详情
func (h *OrderHandler) Get(c *gin.Context) {
    id := c.Param("id")
    orderID, err := strconv.ParseUint(id, 10, 64)
    if err != nil {
        response.Error(c, errorx.CodeInvalidParams)
        return
    }

    result, err := h.service.Get(c.Request.Context(), uint(orderID))
    if err != nil {
        response.Error(c, errorx.CodeNotFound)
        return
    }

    response.Success(c, result)
}

// Update 更新订单
func (h *OrderHandler) Update(c *gin.Context) {
    // ...
}

// Delete 删除订单
func (h *OrderHandler) Delete(c *gin.Context) {
    // ...
}
```

#### 步骤3：注册路由

```go
// internal/interface/admin/http/router/v1/order.go

package v1

import (
    "github.com/gin-gonic/gin"
    orderHandler "server/internal/interface/admin/http/handler/v1/order"
)

type OrderRouter struct {
    handler *orderHandler.OrderHandler
}

func NewOrderRouter(handler *orderHandler.OrderHandler) *OrderRouter {
    return &OrderRouter{handler: handler}
}

func (r *OrderRouter) Register(group *gin.RouterGroup) {
    orderGroup := group.Group("/orders")
    {
        orderGroup.GET("", r.handler.List)
        orderGroup.POST("", r.handler.Create)
        orderGroup.GET("/:id", r.handler.Get)
        orderGroup.PUT("/:id", r.handler.Update)
        orderGroup.DELETE("/:id", r.handler.Delete)
    }
}
```

#### 步骤4：添加到主路由

```go
// internal/interface/admin/http/router/v1/router.go

func NewRouter(
    // ... 现有依赖
    orderRouter *OrderRouter,  // 新增
) *Router {
    return &Router{
        // ... 现有路由
        orderRouter:   orderRouter,  // 新增
    }
}

type Router struct {
    // ... 现有路由
    orderRouter   *OrderRouter  // 新增
}

func (r *Router) Register(engine *gin.Engine) {
    v1 := engine.Group("/admin/v1")
    
    // ... 现有路由注册
    
    // 订单管理（需要RBAC权限）
    rbacGroup := v1.Group("")
    rbacGroup.Use(middleware.JWT(), middleware.RBAC())
    r.orderRouter.Register(rbacGroup)
}
```

#### 步骤5：Wire 注入

```go
// internal/app/wire.go

var handlerSet = wire.NewSet(
    // ... 现有 Handler
    orderHandler.NewOrderHandler,  // 新增
)

var routerSet = wire.NewSet(
    // ... 现有 Router
    router.NewOrderRouter,  // 新增
)

func NewRouter(
    // ... 现有参数
    orderHandler *orderHandler.OrderHandler,  // 新增
) *router.Router {
    // ...
}
```

---

## 三、前端 API 管理

### 3.1 接口封装位置

```
admin-web/src/service/api/v1/
├── auth.ts           # 认证接口
├── content.ts        # 内容管理接口
├── log.ts            # 日志接口
├── route.ts          # 路由接口
├── storage.ts        # 存储接口
├── system-dict.ts    # 字典接口
├── system-manage.ts  # 系统管理接口
└── system-task.ts    # 任务接口
```

### 3.2 类型定义位置

```
admin-web/src/typings/api/v1/
├── auth.d.ts         # 认证类型
├── common.d.ts       # 通用类型
├── content.d.ts      # 内容管理类型
├── log.d.ts          # 日志类型
├── route.d.ts        # 路由类型
├── storage.d.ts      # 存储类型
├── system-dict.d.ts  # 字典类型
└── system-manage.d.ts # 系统管理类型
```

### 3.3 新增 API 步骤

#### 步骤1：定义类型

```typescript
// src/typings/api/v1/order.d.ts

declare namespace ApiV1 {
  /** 订单项 */
  interface Order {
    id: number
    order_no: string
    total_price: number
    status: number
    created_at: number
  }

  /** 订单列表请求 */
  interface GetOrderListRequest {
    status?: number
    page?: number
    size?: number
  }

  /** 订单列表响应 */
  interface GetOrderListResponse {
    list: Order[]
    total: number
  }

  /** 创建订单请求 */
  interface CreateOrderRequest {
    product_id: number
    quantity: number
    address_id: number
    remark?: string
  }

  /** 更新订单请求 */
  interface UpdateOrderRequest {
    id: number
    status?: number
    remark?: string
  }
}
```

#### 步骤2：封装 API

```typescript
// src/service/api/v1/order.ts

import { request } from '@/service/request'

/** 获取订单列表 */
export function fetchGetOrderList(params: ApiV1.GetOrderListRequest) {
  return request<ApiV1.GetOrderListResponse>({
    url: '/admin/v1/orders',
    method: 'GET',
    params
  })
}

/** 创建订单 */
export function fetchCreateOrder(data: ApiV1.CreateOrderRequest) {
  return request<ApiV1.Order>({
    url: '/admin/v1/orders',
    method: 'POST',
    data
  })
}

/** 获取订单详情 */
export function fetchGetOrder(id: number) {
  return request<ApiV1.Order>({
    url: `/admin/v1/orders/${id}`,
    method: 'GET'
  })
}

/** 更新订单 */
export function fetchUpdateOrder(data: ApiV1.UpdateOrderRequest) {
  return request<ApiV1.Order>({
    url: `/admin/v1/orders/${data.id}`,
    method: 'PUT',
    data
  })
}

/** 删除订单 */
export function fetchDeleteOrder(id: number) {
  return request<null>({
    url: `/admin/v1/orders/${id}`,
    method: 'DELETE'
  })
}
```

#### 步骤3：在页面中使用

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { 
  fetchGetOrderList, 
  fetchCreateOrder, 
  fetchDeleteOrder 
} from '@/service/api/v1/order'

const orderList = ref<ApiV1.Order[]>([])
const loading = ref(false)

async function loadOrders() {
  loading.value = true
  const { data } = await fetchGetOrderList({ page: 1, size: 20 })
  if (data) {
    orderList.value = data.list
  }
  loading.value = false
}

async function handleCreate(orderData: ApiV1.CreateOrderRequest) {
  const { error } = await fetchCreateOrder(orderData)
  if (!error) {
    window.$message.success('创建成功')
    loadOrders()
  }
}

async function handleDelete(id: number) {
  const { error } = await fetchDeleteOrder(id)
  if (!error) {
    window.$message.success('删除成功')
    loadOrders()
  }
}

onMounted(loadOrders)
</script>
```

---

## 四、API 清单

### 4.1 认证模块

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| POST | /admin/v1/auth/login | 登录 | 公开 |
| POST | /admin/v1/auth/refreshToken | 刷新Token | 公开 |
| GET | /admin/v1/auth/getUserInfo | 获取用户信息 | JWT |
| GET | /admin/v1/auth/profile | 获取个人资料 | JWT |
| PUT | /admin/v1/auth/profile | 更新个人资料 | JWT |
| POST | /admin/v1/auth/changePassword | 修改密码 | JWT |

### 4.2 路由模块

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| GET | /admin/v1/route/getUserRoutes | 获取用户路由 | JWT |
| GET | /admin/v1/route/isRouteExist | 检查路由存在 | JWT |

### 4.3 系统管理模块

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| GET | /admin/v1/systemManage/getUserList | 管理员列表 | RBAC |
| POST | /admin/v1/systemManage/addUser | 添加管理员 | RBAC |
| PUT | /admin/v1/systemManage/updateUser | 更新管理员 | RBAC |
| DELETE | /admin/v1/systemManage/deleteUser | 删除管理员 | RBAC |
| GET | /admin/v1/systemManage/getRoleList | 角色列表 | RBAC |
| POST | /admin/v1/systemManage/addRole | 添加角色 | RBAC |
| PUT | /admin/v1/systemManage/updateRole | 更新角色 | RBAC |
| DELETE | /admin/v1/systemManage/deleteRole | 删除角色 | RBAC |
| GET | /admin/v1/systemManage/getMenuList | 菜单列表 | RBAC |
| GET | /admin/v1/systemManage/getMenuTree | 菜单树 | RBAC |
| POST | /admin/v1/systemManage/addMenu | 添加菜单 | RBAC |
| PUT | /admin/v1/systemManage/updateMenu | 更新菜单 | RBAC |
| DELETE | /admin/v1/systemManage/deleteMenu | 删除菜单 | RBAC |
| GET | /admin/v1/systemManage/role/:id/menus | 获取角色菜单 | RBAC |
| PUT | /admin/v1/systemManage/role/:id/menus | 设置角色菜单 | RBAC |

### 4.4 内容管理模块

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| GET | /admin/v1/content/categories | 分类列表 | RBAC |
| GET | /admin/v1/content/categories/tree | 分类树 | RBAC |
| POST | /admin/v1/content/categories | 创建分类 | RBAC |
| PUT | /admin/v1/content/categories/:id | 更新分类 | RBAC |
| DELETE | /admin/v1/content/categories/:id | 删除分类 | RBAC |
| GET | /admin/v1/content/articles | 文章列表 | RBAC |
| POST | /admin/v1/content/articles | 创建文章 | RBAC |
| PUT | /admin/v1/content/articles/:id | 更新文章 | RBAC |
| DELETE | /admin/v1/content/articles/:id | 删除文章 | RBAC |
| PUT | /admin/v1/content/articles/:id/publish | 发布文章 | RBAC |
| PUT | /admin/v1/content/articles/:id/unpublish | 下架文章 | RBAC |

### 4.5 存储模块

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| GET | /admin/v1/storage-configs | 存储配置列表 | RBAC |
| POST | /admin/v1/storage-configs | 创建存储配置 | RBAC |
| PUT | /admin/v1/storage-configs | 更新存储配置 | RBAC |
| DELETE | /admin/v1/storage-configs/:id | 删除存储配置 | RBAC |
| POST | /admin/v1/storage/upload-credentials | 获取上传凭证 | JWT |
| POST | /admin/v1/storage/upload-record | 记录上传 | JWT |
| GET | /admin/v1/upload-records | 上传记录列表 | RBAC |

### 4.6 开放平台模块

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| GET | /admin/v1/open/apps | 应用列表 | RBAC |
| POST | /admin/v1/open/apps | 创建应用 | RBAC |
| PUT | /admin/v1/open/apps | 修改应用（含存储绑定） | RBAC |
| DELETE | /admin/v1/open/apps/:id | 删除应用 | RBAC |
| PUT | /admin/v1/open/apps/reset-secret | 重置密钥 | RBAC |
| PUT | /admin/v1/open/apps/ip-rules | 关联IP规则 | RBAC |
| GET | /admin/v1/open/apps/scopes | 获取应用权限 | RBAC |
| GET | /admin/v1/open/apps/available-scopes | 获取可用权限 | RBAC |
| GET | /admin/v1/open/logs | 开放平台日志 | RBAC |
| GET | /admin/v1/open/scopes | 权限分组列表 | RBAC |
| POST | /admin/v1/open/scopes | 创建权限分组 | RBAC |
| PUT | /admin/v1/open/scopes | 修改权限分组 | RBAC |
| DELETE | /admin/v1/open/scopes/:id | 删除权限分组 | RBAC |

### 4.7 Client 端接口

> Client 端接口需通过开放平台签名验证（`X-App-Key` + `X-Signature`），并在 `sys_open_apis` 表中注册。详细参数说明请参考 [客户端API文档](./client-api/00-authentication.md)。

#### 4.7.1 认证模块

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| GET | /client/v1/auth/scene-config | 获取场景验证配置 | 签名 |
| GET | /client/v1/auth/captcha | 获取图形验证码 | 签名 |
| POST | /client/v1/auth/send-code | 发送验证码 | 签名 |
| POST | /client/v1/user/register | 用户注册 | 签名 |
| POST | /client/v1/user/login | 用户登录 | 签名 |
| POST | /client/v1/user/refresh-token | 刷新令牌 | 签名 |
| POST | /client/v1/user/reset-password | 找回密码 | 签名 |

#### 4.7.2 用户模块

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| GET | /client/v1/user/profile | 获取个人资料 | 签名 + JWT |
| PUT | /client/v1/user/profile | 更新个人资料 | 签名 + JWT |
| PUT | /client/v1/user/password | 修改密码 | 签名 + JWT |
| DELETE | /client/v1/user/account | 注销账号 | 签名 + JWT |
| GET | /client/v1/user/upload-token | 获取上传凭证 | 签名 + JWT |
| POST | /client/v1/user/logout | 退出登录 | 签名 + JWT |

#### 4.7.3 内容模块

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| GET | /client/v1/content/categories/tree | 获取分类树 | 签名 |
| GET | /client/v1/content/articles | 文章列表 | 签名 |
| GET | /client/v1/content/article/:id | 文章详情 | 签名 |
| POST | /client/v1/content/article/:id/like | 点赞文章 | 签名 |
| GET | /client/v1/content/banners/:code | 获取 Banner 组 | 签名 |
| POST | /client/v1/content/banners/:id/click | 记录 Banner 点击 | 签名 |

#### 4.7.4 存储模块

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| POST | /client/v1/storage/credentials | 获取上传凭证 | 签名 |
| POST | /client/v1/storage/records | 创建上传记录 | 签名 |

#### 4.7.5 消息模块

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| GET | /client/v1/message/internal | 站内信列表 | 签名 + JWT |
| GET | /client/v1/message/internal/:id | 站内信详情 | 签名 + JWT |
| PUT | /client/v1/message/internal/read | 标记已读 | 签名 + JWT |
| PUT | /client/v1/message/internal/read-all | 全部标记已读 | 签名 + JWT |
| GET | /client/v1/message/internal/unread-count | 未读消息数 | 签名 + JWT |

#### 4.7.6 Echo 测试

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| POST | /client/v1/echo | Echo 测试 | 签名 |

### 4.8 日志模块

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| GET | /admin/v1/operation-logs | 操作日志列表 | RBAC |
| DELETE | /admin/v1/operation-logs/:id | 删除操作日志 | RBAC |
| GET | /admin/v1/error-logs | 错误日志列表 | RBAC |
| PUT | /admin/v1/error-logs/:id/resolve | 标记错误已解决 | RBAC |
| DELETE | /admin/v1/error-logs/:id | 删除错误日志 | RBAC |

### 4.9 系统配置模块

| Method | Path | 说明 | 权限 |
|--------|------|------|------|
| GET | /admin/v1/system/configs | 获取配置 | RBAC |
| PUT | /admin/v1/system/configs | 更新配置 | RBAC |
| GET | /admin/v1/system/dict/types | 字典类型列表 | RBAC |
| GET | /admin/v1/system/dict/data/:code | 字典数据 | JWT |
| GET | /admin/v1/system/tasks | 任务列表 | RBAC |
| POST | /admin/v1/system/tasks/:name/run | 立即执行任务 | RBAC |
| PUT | /admin/v1/system/tasks/:name | 更新任务配置 | RBAC |

---

## 五、最佳实践

### 5.1 后端最佳实践

1. **参数校验**：使用 `binding` 标签进行参数校验
2. **错误处理**：统一使用 `errorx` 包的错误码
3. **响应格式**：统一使用 `response.Success/Error`
4. **日志记录**：敏感操作自动记录操作日志
5. **权限控制**：严格区分 JWT 和 RBAC 权限级别

### 5.2 前端最佳实践

1. **类型定义**：所有 API 参数和响应必须定义类型
2. **错误处理**：统一在 `backend-error.ts` 中处理错误码
3. **请求封装**：禁止直接调用 axios，必须使用封装后的 request
4. **API 版本**：显式导入指定版本的 API
5. **缓存策略**：合理设置 GET 请求的缓存策略

---

## 六、相关文档

- [Server架构设计](./server-architecture.md)
- [Admin-Web架构设计](./admin-web-architecture.md)
- [状态码规范](./status-codes.md)
