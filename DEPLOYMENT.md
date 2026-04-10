# NetyAdmin 部署指南

本指南将详细介绍如何部署 NetyAdmin 后端 (Go) 和前端 (Vue)。

## 1. 环境准备

在部署 NetyAdmin 之前，请确保您的服务器已安装以下软件：

*   **Go**: 1.21 或更高版本
*   **Node.js**: 20.x 或更高版本
*   **pnpm**: 8.x 或更高版本 (推荐使用 `npm install -g pnpm` 安装)
*   **PostgreSQL**: 12 或更高版本
*   **Redis**: 6 或更高版本
*   **Git**: 用于克隆项目代码

## 2. 后端部署 (`server`)

### 2.1 克隆项目

```bash
git clone https://github.com/your-org/NetyAdmin.git # 替换为您的实际仓库地址
cd NetyAdmin/server
```

### 2.2 配置数据库和 Redis

编辑 `server/config.toml` 文件，根据您的实际环境修改数据库和 Redis 连接信息。

```toml
# server/config.toml 示例
[database]
driver = "postgres"
source = "host=localhost port=5432 user=netyadmin password=your_password dbname=netyadmin sslmode=disable TimeZone=Asia/Shanghai"

[redis]
enabled = true
addr = "localhost:6379"
password = ""
db = 0
prefix = "netyadmin:"
```

### 2.3 数据库迁移

NetyAdmin 使用 GORM 进行数据库操作，并在服务启动时自动执行 `migrations` 目录下的所有 SQL 迁移脚本，创建必要的表结构和初始化数据。因此，您无需手动执行数据库迁移命令。

### 2.4 编译与运行

```bash
# 编译后端服务
go build -o netyadmin-server cmd/server/main.go

# 运行后端服务
./netyadmin-server
```

默认情况下，后端服务将在 `http://localhost:8010` 启动。您可以通过修改 `config.toml` 中的 `[server].port` 来更改端口。

## 3. 前端部署 (`admin-web`)

### 3.1 依赖安装

```bash
# 确保在 NetyAdmin 根目录下
cd NetyAdmin
pnpm install
```

### 3.2 环境变量配置

在 `admin-web` 目录下创建 `.env.production` 文件，配置生产环境相关的环境变量。

```
# .env.production 示例
VITE_APP_BASE_API="http://localhost:8010/admin/v1" # 后端 API 地址
VITE_APP_ROUTE_HOME_PATH="/dashboard" # 首页路由
```

### 3.3 编译

```bash
# 确保在 NetyAdmin 根目录下
cd NetyAdmin
pnpm build
```

编译完成后，将在 `NetyAdmin/admin-web/dist` 目录下生成前端静态文件。

### 3.4 Nginx 配置示例

您可以使用 Nginx 来部署前端静态文件，并配置反向代理将 API 请求转发到后端服务。

```nginx
server {
    listen 80;
    server_name your_domain.com; # 替换为您的域名

    # 前端静态文件
    location / {
        root /path/to/NetyAdmin/admin-web/dist; # 替换为您的前端静态文件路径
        index index.html;
        try_files $uri $uri/ /index.html;
    }

    # API 反向代理
    location /admin/v1/ {
        proxy_pass http://localhost:8010; # 替换为您的后端服务地址和端口
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    error_page 500 502 503 504 /50x.html;
    location = /50x.html {
        root html;
    }
}
```

请根据您的实际情况修改 Nginx 配置，并重新加载 Nginx 服务。

## 4. 启动与访问

1.  启动后端服务。
2.  启动 Nginx 服务。
3.  通过浏览器访问 `http://your_domain.com` 即可访问 NetyAdmin。
