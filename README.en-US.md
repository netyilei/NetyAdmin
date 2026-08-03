[中文](README.md)

# NetyAdmin - Enterprise Admin System Base

NetyAdmin is an enterprise-level admin system base built with **Go + Gin** backend and **Vue 3 + TypeScript** frontend. It adopts a modern BFF (Backend For Frontend) multi-terminal isolation architecture, providing high-performance, highly available, and feature-rich admin solutions.

---

## ✨ Core Features

### 🚀 Modern Tech Stack

- **Frontend**: Vue 3, TypeScript, Vite, Naive UI, UnoCSS, Pinia, Vue Router, vue-i18n
- **Backend**: Go 1.25+, Gin, GORM (PostgreSQL), Redis (optional), JWT

### 🏗️ Clear Architecture Design

- **BFF Multi-terminal Isolation**: Physical isolation of Admin/Client terminals to avoid business logic mixing
- **Layered Architecture**: Strictly follows `router -> handler -> service -> repository -> entity` call chain
- **API Version Control**: Explicit version management, supporting smooth evolution
- **Dependency Injection**: Uses Wire for dependency assembly, facilitating testing and replacement

### 🔒 Complete Authentication & Authorization

- **JWT Authentication**: Secure and reliable user authentication mechanism
- **RBAC Permission System**: Fine-grained permission control based on roles, menus, buttons, and APIs
- **Dynamic Routing**: Backend dynamically generates route tree, frontend renders menus based on permissions

### ⚡ High Performance & High Availability

- **Transparent Caching**: Redis + BigCache dual engines, supporting dynamic switches and batch invalidation
- **Unified Event Bus**: PubSubBus consolidates Redis Pub/Sub, driver-based design supports standalone/cluster switching
- **Unified Log Buffer**: LogBus asynchronously aggregates all system logs, tiered backpressure (P0/P1/P2), dual-trigger batch writing
- **Hot Config Sync**: Real-time configuration synchronization across the network via PubSubBus
- **Task Scheduling**: Built-in task engine, supporting scheduled tasks, backend management, and log persistence; automatically enables distributed lock for multi-instance deduplication when Redis is enabled
- **Database Migration**: Automatically executes SQL migration scripts during startup
- **Email Driver**: Based on go-simple-mail, supporting SSL/TLS, STARTTLS, and multiple SMTP authentication methods

### 📦 Rich Feature Modules

| Module | Features |
|--------|----------|
| **RBAC** | Admin, Role, Menu, Button, API management |
| **User Module** | Client-side user system, multi-device login (platform-scoped session kick-in), TokenStore abstraction (cache/database), account lock & auto-unlock, secure encryption, OAuth third-party account binding base (OAuthBindingService), multi-type user JWT auth extension (TypedUserJWTAuth + RegisterTypedAuthModule) |
| **Message Hub** | Unified sending entry (SMS/Email/Internal), template rendering, async retry, STARTTLS support |
| **Open Platform** | AppKey authentication, secure signature, configurable distributed rate limiting (token bucket), Scope permissions, app-level storage binding |
| **IP Access Control** | Global/App-level IP governance, CIDR matching, high-performance memory filtering |
| **Content Management** | Categories, Articles, Banner, supporting rich text, scheduled publishing, client-facing public APIs |
| **Storage Management** | Multi-storage source configuration, upload credentials, upload records, app-level storage isolation, Client-side upload API |
| **Log Audit** | Operation logs, error logs, open platform logs, task logs, LogBus unified buffer, sensitive field desensitization |
| **System Config** | Dynamic dictionaries, system parameters, task scheduling |
| **Unified Event Bus** | PubSubBus, driver-based design, Topic registry, distributed cache sync |
| **Captcha** | Graphic captcha, supporting multiple types and storage schemes, scene-based verification config (login/register/password reset) |
| **Downstream Extension** | Module interface (App.RegisterModule) — downstream projects assemble business routes/jobs/callbacks with zero base code changes (ClientRouter/AdminRouter/Job/Engine 4 sub-interfaces) |

---

## 📚 Documentation Index

### Project Standards

| Document | Description |
|----------|-------------|
| [AGENTS.md](AGENTS.md) | AI collaboration standards: required workflow, development conventions, document index, commit standards |
| [RULES.md](RULES.md) | **Red-line rules**: layered architecture, transaction management, deletion strategies, security standards, code quality |
| [SHARED.md](SHARED.md) | Shared knowledge base: pitfalls, architecture decisions, warnings |

### Architecture Design Documents

| Document | Description |
|----------|-------------|
| [Server Architecture & Directory Structure](docs/server-architecture.md) | Backend architecture concepts, layered design, key standards |
| [Admin-Web Architecture & Directory Structure](docs/admin-web-architecture.md) | Frontend architecture concepts, directory standards, development standards |

### Module Detail Documents

| Document | Description |
|----------|-------------|
| [User Module Details](docs/server-module-user.md) | Client user system, multi-terminal login, TokenStore abstraction, account lock mechanism |
| [Message Hub Details](docs/server-module-message.md) | Unified sending entry (SMS/Email/Internal), driver extension, async tasks |
| [Open Platform Details](docs/server-module-open-platform.md) | AppKey authentication, signature verification, configurable distributed rate limiting, Scope permissions |
| [IP Access Control Details](docs/server-module-ipac.md) | High-performance memory matching, CIDR network, hierarchical governance |
| [Captcha Module Details](docs/server-module-captcha.md) | Captcha types, storage schemes, dynamic configuration, scene-based verification |
| [Cache Module Details](docs/server-module-cache.md) | Dual-engine caching, Tags batch invalidation, dynamic switches |
| [Unified Event Bus Details](docs/server-module-pubsub.md) | PubSubBus architecture, driver mechanism, Topic registry, secondary development |
| [Task System Details](docs/server-module-task.md) | Task scheduling, queue mechanism, backend management |
| [Dictionary Module Details](docs/server-module-dict.md) | Dynamic dictionaries, caching strategies, usage examples |
| [Content Management Module Details](docs/server-module-content.md) | Articles, categories, Banner, scheduled publishing |
| [Storage Module Details](docs/server-module-storage.md) | Object storage, upload credentials, driver extension |
| [Log Module Details](docs/server-module-log.md) | Operation logs, error logs, LogBus unified buffer, sensitive desensitization |
| [Data Migration Details](docs/server-module-migration.md) | Migration scripts, version control, idempotent execution |

### Development Standards

| Document | Description |
|----------|-------------|
| [Secondary Development Guide](docs/development-guide.md) | Full new module walkthrough (Entity → Repository → DTO → Service → Handler → Router → Wire), downstream Module assembly (§6) |
| [Server Architecture & Directory Structure](docs/server-architecture.md) | Backend architecture concepts, layered design, key standards |
| [Admin-Web Architecture & Directory Structure](docs/admin-web-architecture.md) | Frontend architecture concepts, directory standards, development standards |
| [Status Code Specification](docs/status-codes.md) | Error code encoding rules, full code table, addition process |
| [API Management Guide](docs/api-management.md) | Frontend and backend API definitions, addition process, best practices |
| [Quick Deployment Guide](docs/quick-deployment.md) | Environment preparation, configuration instructions, deployment steps |

### Client API Documents (client-api-ws)

| Document | Description |
|----------|-------------|
| [Client API Index](docs/client-api-ws/README.md) | All client API endpoints overview |
| [Authentication & Signing Guide](docs/client-api-ws/00-authentication.md) | Open platform signature, JWT Token, unified response format, error codes |
| [Auth Module API](docs/client-api-ws/01-auth.md) | Captcha, scene config, verification code sending |
| [User Module API](docs/client-api-ws/02-user.md) | Login/Register/Forgot password/Profile/Password change/Logout/Upload |
| [Content Module API](docs/client-api-ws/03-content.md) | Article list/detail/like, Banner |
| [Storage Module API](docs/client-api-ws/04-storage.md) | Upload credentials, direct upload flow, upload record callback |
| [Message Module API](docs/client-api-ws/05-message.md) | Message list, detail, read status, unread count |
| [Error Code Reference](docs/client-api-ws/06-error-codes.md) | Client-side error code full table |

### Admin API Documents (admin-api-ws)

| Document | Description |
|----------|-------------|
| [Admin API Index](docs/admin-api-ws/README.md) | All admin API endpoints overview (110+) |
| [Authentication Guide](docs/admin-api-ws/00-authentication.md) | JWT auth flow, token format, middleware groups |
| [Auth API](docs/admin-api-ws/01-auth.md) | Login, refresh token, user info, profile, change password, logout |
| [Admin Management API](docs/admin-api-ws/02-admin.md) | Admin CRUD, batch delete, self-delete protection |
| [RBAC System API](docs/admin-api-ws/03-system-rbac.md) | Role CRUD+permissions, Menu CRUD+tree, Button CRUD, API CRUD+tree |
| [Content Management API](docs/admin-api-ws/04-content.md) | Category CRUD+tree, Article CRUD+publish/pin, Banner group/item CRUD |
| [Dictionary Management API](docs/admin-api-ws/05-dict.md) | Dict type CRUD, dict data CRUD, public query |
| [Storage Management API](docs/admin-api-ws/06-storage.md) | Storage config CRUD+test upload, upload records, 3-step upload |
| [Ops Management API](docs/admin-api-ws/07-ops.md) | Operation logs, error logs, IP access control, open platform logs |
| [Open Platform API](docs/admin-api-ws/08-open-platform.md) | App CRUD+reset secret, permission scopes, open API CRUD |
| [Message Management API](docs/admin-api-ws/09-message.md) | Message template CRUD, send records, direct send |
| [Task Management API](docs/admin-api-ws/10-task.md) | Task list, run/start/stop/reload, logs |
| [System Config API](docs/admin-api-ws/12-config.md) | Config query, update, email test |
| [Common API](docs/admin-api-ws/13-common.md) | Captcha, user routes, route check |
| [Error Code Reference](docs/admin-api-ws/14-error-codes.md) | Full error code table (50+) |

---

## 🚀 Quick Start

### Environment Requirements

- **Go** >= 1.25
- **Node.js** >= 18
- **PostgreSQL** >= 14
- **Redis** >= 6.0 (optional)

### One-click Start

```bash
# 1. Clone code
git clone https://github.com/netyilei/NetyAdmin.git
cd NetyAdmin

# 2. Start server
cd server
# Edit config.yaml to configure database
go mod download
go run cmd/server/main.go

# 3. Start frontend (new terminal)
cd ../admin-web
pnpm install
pnpm dev
```

### Default Account

- **Username**: `admin`
- **Password**: `admin123`

> ⚠️ **Security Tip**: Please change the default password immediately after deployment!

### Default Open Platform App

The default app can sign and call client APIs right after cloning the base project (no secret reset needed):

- **AppKey**: `01JQDEFAULTAPP001`
- **AppSecret**: `netyadmin-default-app-secret`

> ⚠️ **Security Tip**: After production deployment, rotate the default Secret via "Open Platform → Reset Secret" (same as the default password).

---

## 🏗️ Project Structure

```
NetyAdmin/
├── server/                    # Backend service (Go + Gin)
│   ├── cmd/server/           # Process entry
│   ├── internal/             # Business code
│   │   ├── app/              # Application startup
│   │   ├── domain/           # Domain models
│   │   ├── interface/        # Access layer (BFF)
│   │   ├── pkg/              # Infrastructure
│   │   ├── repository/       # Data access
│   │   └── service/          # Business services
│   ├── migrations/           # Database migrations
│   └── config.yaml           # Configuration file
│
├── admin-web/                 # Admin frontend (Vue 3)
│   ├── src/
│   │   ├── components/       # Components
│   │   ├── service/api/      # API encapsulation
│   │   ├── store/            # State management
│   │   ├── views/            # Pages
│   │   └── locales/          # Internationalization
│   └── package.json
│
└── docs/                      # Documentation
```

---

## 🛠️ Secondary Development

For detailed development workflow, red-line rules, and code examples, refer to the **[Secondary Development Guide](docs/development-guide.md)**.

### Quick Overview

| Layer | Responsibility | Red Line |
|-------|---------------|----------|
| **Handler** | Param binding → Call Service → Unified response | No entity import, no direct cacheFast/cacheSlow/repo access |
| **Service** | Business rules + multi-repo orchestration | No `*gin.Context`, receives DTOs, use TM for multi-step ops |
| **Repository** | CRUD + query assembly | No self-managed transactions, use `getDB(ctx)` for DB |
| **Entity** | GORM model definition | Pure data structure, no business logic |

### Add Frontend Page

Refer to the secondary development example in [Admin-Web Architecture Design](docs/admin-web-architecture.md):

1. Define types (`typings/api/v1`)
2. Encapsulate API (`service/api/v1`)
3. Create page (`views/xxx/index.vue`)
4. Add internationalization (`locales/langs`)

---

## 📖 Applicable Scenarios

- **Enterprise Admin Systems**: User, content, permission, configuration management
- **Rapid Development Platform**: Quick start base for new projects, reducing repetitive development
- **Multi-client Support**: BFF architecture supports Admin, mobile, Web terminals
- **Fine-grained Permission Control**: RBAC system meets enterprise fine-grained control needs
- **Internationalization Projects**: Both frontend and backend support internationalization
- **Microservice Foundation Services**: Authentication, configuration management, content management

---

## 🤝 Contributing

Issues and Pull Requests are welcome.

---

## 📄 License

This project is open-sourced under the MIT License.

---

**Note**: NetyAdmin is refactored and streamlined based on soybean-admin. Thanks to the soybean-admin team for their contribution.