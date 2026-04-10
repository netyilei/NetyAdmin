# Server 多端接入架构 (BFF Pattern) 规范

作为通用中台基座，服务端采用 **“多端物理隔离中台 (BFF Pattern)”** 架构设计。

## 1. 为什么采用多端隔离？

在作为“通用中台基座”时，系统将面临多种终端（Admin、代理端、客户端端等）的接入需求：
- **业务逻辑差异**：Admin 的登录与 Client (客户) 的登录逻辑完全不同，入参 DTO 不同。混合存放会导致大量 `AdminLoginReq`、`ClientLoginReq` 前缀，命名极其混乱。
- **权限安全隐患**：Admin 需要 RBAC 中间件，而 Client 只需要基础 JWT。混合在同一个路由注册文件中容易配错，导致越权漏洞。

## 2. BFF 目录结构解析

架构引入 `internal/interface/` 作为网络接入的唯一边界，实行严格的“端分离”。

```text
server/internal/
├── interface/             # 【接入层】完全按“端”隔离
│   ├── admin/             # 面向 Admin-Web 的接口
│   │   ├── dto/           # Admin 专用的 Request/Response DTO
│   │   ├── http/          # HTTP 协议接入
│   │   │   ├── handler/   # Admin 专用的 Handler
│   │   │   └── router/    # Admin 专用的路由注册 (Router Registry)
│   │   └── ws/            # WebSocket 协议 (未来可扩展)
│   │
│   ├── client/            # 面向 C端(移动/Web) 的接口 (待开发)
│   └── ea/                # 面向 EA/客户端 的接口 (待开发)
│
├── service/               # 【核心业务层】端无关，纯粹的领域逻辑
├── repository/            # 【数据访问层】DB / Cache
└── domain/                # 【领域模型】Entity / VO
```

## 3. 开发规范红线

基于此架构，任何新增模块必须遵守以下规则：

1. **下沉与隔离**：
   - 所有的 HTTP 请求入参（`BindJSON`）、参数校验、鉴权拦截，**只能且必须发生在 `interface/[端]/http` 中**。
   - `internal/service/` 层的函数参数中，**绝对禁止出现 `*gin.Context`** 或任何特定网络协议的痕迹。它只接收基础 Go 类型或 VO，返回结果或 Error。

2. **DTO 专属于端**：
   - 不允许在 `internal/domain/` 下建立全局的 DTO。
   - 比如 `CreateUserRequest`，如果是 Admin 端的请求，就放在 `interface/admin/dto/user.go`。如果是 Client 端注册，就放在 `interface/client/dto/register.go`。两个端的开发者互不影响，各自演进。

3. **路由注册器模式 (Registry Pattern)**：
   - 新增业务 Handler 时，不需要修改全局的主 `router.go`。
   - 只需让新的 Router 结构体实现 `ModuleRouter` 接口（`RegisterPublic`, `RegisterAuth`, `RegisterPermission`），并在 `app/wire.go` 的 `NewRouter` 参数列表中注入即可。主引擎会自动遍历执行。
