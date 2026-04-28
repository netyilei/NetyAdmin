[中文](README.md)

# NetyAdmin - Enterprise Admin System Base

NetyAdmin is an enterprise-level admin system base built with **Go + Gin** backend and **Vue 3 + TypeScript** frontend. It adopts a modern BFF (Backend For Frontend) multi-terminal isolation architecture, providing high-performance, highly available, and feature-rich admin solutions.

---

## ✨ Core Features

### 🚀 Modern Tech Stack

- **Frontend**: Vue 3, TypeScript, Vite, Naive UI, UnoCSS, Pinia, Vue Router, vue-i18n
- **Backend**: Go 1.21+, Gin, GORM (PostgreSQL), Redis (optional), JWT

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
- **Task Scheduling**: Built-in task engine, supporting scheduled tasks, backend management, and log persistence
- **Database Migration**: Automatically executes SQL migration scripts during startup
- **Email Driver**: Based on go-simple-mail, supporting SSL/TLS, STARTTLS, and multiple SMTP authentication methods

### 📦 Rich Feature Modules

| Module | Features |
|--------|----------|
| **RBAC** | Admin, Role, Menu, Button, API management |
| **User Module** | Client-side user system, multi-terminal login, TokenStore abstraction (cache/database), account lock & auto-unlock, secure encryption |
| **Message Hub** | Unified sending entry (SMS/Email/Internal), template rendering, async retry, STARTTLS support |
| **Open Platform** | AppKey authentication, secure signature, configurable distributed rate limiting (token bucket), Scope permissions, app-level storage binding |
| **IP Access Control** | Global/App-level IP governance, CIDR matching, high-performance memory filtering |
| **Content Management** | Categories, Articles, Banner, supporting rich text and scheduled publishing |
| **Storage Management** | Multi-storage source configuration, upload credentials, upload records, app-level storage isolation, Client-side upload API |
| **Log Audit** | Operation logs, error logs, open platform logs, task logs, LogBus unified buffer, sensitive field desensitization |
| **System Config** | Dynamic dictionaries, system parameters, task scheduling |
| **Unified Event Bus** | PubSubBus, driver-based design, Topic registry, distributed cache sync |
| **Captcha** | Graphic captcha, supporting multiple types and storage schemes, scene-based verification config (login/register/password reset) |

---

## 📚 Documentation Index

### Architecture Design Documents

| Document | Description |
|----------|-------------|
| [Server Architecture & Directory Structure](docs/server-architecture.md) | Backend architecture concepts, layered design, secondary development guide |
| [Admin-Web Architecture & Directory Structure](docs/admin-web-architecture.md) | Frontend architecture concepts, directory specifications, development standards |

### Module Detail Documents

| Document | Description |
|----------|-------------|
| [User Module Details](docs/server-module-user.md) | Client user system, multi-terminal login, TokenStore abstraction, account lock mechanism, login storage backend |
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

### Development Standard Documents

| Document | Description |
|----------|-------------|
| [Status Code Specification](docs/status-codes.md) | Error code encoding rules, full code table, addition process |
| [API Management Guide](docs/api-management.md) | Frontend and backend API definitions, addition process, best practices |
| [Quick Deployment Guide](docs/quick-deployment.md) | Environment preparation, configuration instructions, deployment steps |

### Client API Documents

| Document | Description |
|----------|-------------|
| [Authentication & Signing Guide](docs/client-api/00-authentication.md) | Open platform signature, JWT Token, unified response format, error codes |
| [Login/Register/Password Reset](docs/client-api/01-auth.md) | Scene verification config, captcha, login/register flow |
| [User Profile Management](docs/client-api/02-user.md) | Profile, change password, account deletion, upload token |
| [Content Management](docs/client-api/03-content.md) | Category tree, article list/detail/like, Banner |
| [File Upload](docs/client-api/04-storage.md) | Upload credentials, direct upload flow, upload record callback |
| [Internal Messages](docs/client-api/05-message.md) | Message list, detail, read status, unread count |
| [Echo Test](docs/client-api/06-echo.md) | Connectivity test endpoint |

---

## 🚀 Quick Start

### Environment Requirements

- **Go** >= 1.21
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
# Edit config.toml to configure database
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
│   └── config.toml           # Configuration file
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

### Add Backend Module

Refer to the secondary development example in [Server Architecture Design](docs/server-architecture.md):

1. Define entity (`domain/entity`)
2. Create repository (`repository`)
3. Create DTO (`interface/admin/dto`)
4. Create service (`service`)
5. Create Handler (`interface/admin/http/handler`)
6. Register route (`interface/admin/http/router`)
7. Wire injection (`app/wire.go`)

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
