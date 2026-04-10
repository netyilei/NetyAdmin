[中文](README.md)

# NetyAdmin - General Purpose Admin System Base

NetyAdmin is a general-purpose admin system base built with a Go backend and a Vue 3 frontend. It aims to provide a fast-developing, high-performance, highly available, and feature-rich administrative backend solution for enterprise-level applications. The project combines the latest frontend technology stack with a robust backend architecture, striving to be a solid foundation for building various management systems.

## Core Features

### 🚀 Modern Technology Stack

*   **Frontend**: Vue 3, TypeScript, Vite, Naive UI, UnoCSS, Pinia, Vue Router, vue-i18n
*   **Backend**: Go, Gin, GORM (PostgreSQL), Redis (Optional), JWT

### 💡 Clear Architectural Design

*   **Frontend**: Adopts a pnpm monorepo architecture with a clear directory structure, adhering to a strict page-layer architecture, high component cohesion, and API version isolation, making it easy to maintain and extend.
*   **Backend**: Employs a BFF (Backend For Frontend) pattern to achieve physical isolation for multiple clients, ensuring business logic and security. Follows a clear layered architecture (`router -> handler -> service -> repository -> entity`), with modules split by business domain.

### 🔒 Comprehensive Authentication and Authorization Management

*   **JWT Authentication**: Provides a secure and reliable user authentication mechanism.
*   **RBAC Authorization System**: Fine-grained permission control based on roles, menus, buttons, and APIs, supporting CRUD operations and authorization management for administrators, roles, menus, APIs, and buttons.
*   **Dynamic Routing**: The backend dynamically generates a route tree, and the frontend renders menus and controls page access based on user permissions.

### ⚡ High Performance and High Availability

*   **Go Language Backend**: Delivers high-performance web service capabilities.
*   **Transparent Caching**: Supports Redis and local memory (BigCache) dual-layer caching, unified management via `LazyCacheManager`, supporting Key standardization, Prefix, and Tags for batch invalidation, effectively improving data access performance.
*   **Hot Configuration Sync**: System configurations support Upsert updates and achieve real-time synchronization across the network via Redis Pub/Sub, supporting dynamic toggles for caching and task systems.

### ✨ Rich Functional Modules

*   **User Management**: Administrators, roles, permissions, etc.
*   **Content Management**: CRUD for categories, articles, banners, supporting article publishing, pinning, scheduled publishing, etc.
*   **System Configuration**: Dynamic dictionaries, system parameter configuration, hot synchronization, and task system.
*   **Storage Management**: Object storage configuration, upload credential issuance, and upload record management.
*   **Log Auditing**: Recording, querying, and managing operation logs and error logs, supporting sensitive field desensitization.

## Applicable Scenarios

NetyAdmin, as a comprehensive and architecturally advanced admin system base, is particularly suitable for the following scenarios:

*   **Enterprise-level Admin Systems**: Applicable to various admin systems that require managing users, content, permissions, system configurations, and other business aspects.
*   **Rapid Development Platform**: Can serve as a quick-start base for new projects, helping teams efficiently build enterprise-level applications and reduce repetitive development work.
*   **Multi-client Supported Businesses**: The backend BFF architecture provides a good foundation for future expansion to support multiple frontends (e.g., Admin, mobile, web).
*   **Systems Requiring Fine-grained Permission Control**: The comprehensive RBAC authorization system can meet enterprise demands for precise control over user operations.
*   **Projects Requiring Internationalization Support**: Both frontend and backend support internationalization, making it convenient for deployment in different language environments worldwide.
*   **Foundation Services in Microservice Architectures**: Can serve as foundational services for authentication, authorization, configuration management, content management, etc., in a microservice architecture.

## Quick Start

(Brief instructions for deployment and development environment setup will be placed here; detailed content will be in the deployment documentation)

## Documentation

*   [Admin Base: Status Codes and Frontend i18n Mapping](docs/admin_base_status_codes.md)
*   [Admin-Web Directory Structure and Architecture Specification](docs/admin_web_directory.md)
*   [Admin-Web Page Modules and Routing Behavior](docs/admin_web_modules.md)
*   [Admin-Web (Vue) Current Status Summary](docs/admin_web_summary.md)
*   [Server (Go) HTTP Route List](docs/server_api.md)
*   [Server Multi-client Access Architecture (BFF Pattern) Specification](docs/server_arch_bff.md)
*   [Server Base: Caching and Hot Configuration Sync](docs/server_cache_configsync.md)
*   [Server (Go) Directory Structure and Layering](docs/server_directory.md)
*   [Server (Go) Current Status Summary](docs/server_summary.md)

---

**Note**: `netyadmin-web` is the original project for `admin-web`, which we have refactored and streamlined. We thank the `netyadminjs/netyadmin-web` team for their contributions.
