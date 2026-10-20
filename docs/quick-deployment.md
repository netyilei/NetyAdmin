# 快速部署指南

本文档介绍如何快速部署 NetyAdmin 系统，包含环境准备、配置说明、部署步骤和常见问题排查。

---

## 一、环境要求

### 1.1 服务端要求

| 组件 | 版本要求 | 说明 |
|------|----------|------|
| Go | >= 1.26 | 后端运行环境（与 server/go.mod 一致） |
| PostgreSQL | >= 14 | 主数据库 |
| Redis | >= 6.0 | 可选：单机可关闭（L1 本地缓存承担全部缓存）；多节点部署必须启用 |

### 1.2 前端要求

| 组件 | 版本要求 | 说明 |
|------|----------|------|
| Node.js | >= 20.19 | 前端运行环境（与 admin-web/package.json engines 一致） |
| pnpm | >= 10.5 | 包管理器 |

### 1.3 系统要求

- **操作系统**：Linux / macOS / Windows
- **内存**：最低 2GB，推荐 4GB+
- **磁盘**：最低 10GB 可用空间

---

## 二、环境准备

### 2.1 安装 Go

```bash
# Linux/macOS
wget https://go.dev/dl/go1.26.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz

# 配置环境变量
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 验证安装
go version
```

### 2.2 安装 PostgreSQL

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install postgresql postgresql-contrib

# CentOS/RHEL
sudo yum install postgresql-server postgresql-contrib
sudo postgresql-setup initdb
sudo systemctl start postgresql

# macOS
brew install postgresql
brew services start postgresql
```

### 2.3 安装 Redis（可选）

```bash
# Ubuntu/Debian
sudo apt-get install redis-server
sudo systemctl start redis

# CentOS/RHEL
sudo yum install redis
sudo systemctl start redis

# macOS
brew install redis
brew services start redis
```

### 2.4 安装 Node.js 和 pnpm

```bash
# 使用 nvm 安装 Node.js
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
source ~/.bashrc
nvm install 20
nvm use 20

# 安装 pnpm（建议 corepack，按 package.json 的 packageManager 版本）
corepack enable
corepack prepare pnpm@latest --activate

# 验证安装
node -v
pnpm -v
```

---

## 三、数据库初始化

### 3.1 创建数据库

```bash
# 切换到 postgres 用户
sudo -u postgres psql

# 创建数据库
CREATE DATABASE netyadmin;

# 创建用户（可选）
CREATE USER netyadmin WITH PASSWORD 'your_password';
GRANT ALL PRIVILEGES ON DATABASE netyadmin TO netyadmin;

# 退出
\q
```

### 3.2 配置数据库访问

编辑 PostgreSQL 配置文件：

```bash
# 找到配置文件位置
sudo -u postgres psql -c "SHOW config_file;"

# 编辑 postgresql.conf，允许远程连接
listen_addresses = '*'

# 编辑 pg_hba.conf，添加访问规则
# IPv4 local connections:
host    all             all             0.0.0.0/0               md5

# 重启 PostgreSQL
sudo systemctl restart postgresql
```

---

## 四、服务端部署

### 4.1 获取代码

```bash
git clone https://github.com/netyilei/NetyAdmin.git
cd NetyAdmin/server
```

### 4.2 配置服务

复制配置文件模板：

```bash
cp config.example.yaml config.yaml
```

编辑 `config.yaml`（YAML 格式，完整项见 [server-architecture.md 12-Factor 配置](./server-architecture.md#12-factor-配置)）：

```yaml
server:
  port: 8010
  mode: release  # debug/release；release 模式启动期强校验敏感配置
  trusted_proxies: []  # 部署在 Nginx/CDN 后填写真实代理 IP/CIDR

database:
  host: localhost
  port: 5432
  user: "netyadmin"
  password: "your_password"
  dbname: "netyadmin"
  sslmode: disable

redis:
  enabled: true
  host: localhost
  port: 6379
  password: ""
  db: 0
  prefix: "netyadmin"

jwt:
  # RS256 非对称密钥（二选一：文件路径或 PEM 内容）
  private_key_file: "./keys/jwt_private.pem"
  public_key_file: "./keys/jwt_public.pem"
  access_token_ttl: "30m"     # 必须 ≤ 30 分钟
  refresh_token_ttl: "168h"

security:
  aes_key: "<32字节随机密钥>"
  upload_hmac_key: "<32字节随机HMAC密钥>"

migration:
  enabled: true   # 启动期自动执行 SQL 迁移（SQL 文件已 embed 进二进制）
```

### 4.3 安装依赖

```bash
go mod download
```

### 4.4 构建运行

```bash
# 开发模式
go run cmd/server/main.go

# 生产构建
go build -o netyadmin-server cmd/server/main.go

# 运行
./netyadmin-server
```

### 4.5 使用 systemd 管理（Linux）

创建服务文件 `/etc/systemd/system/netyadmin.service`：

```ini
[Unit]
Description=NetyAdmin Server
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=netyadmin
WorkingDirectory=/opt/netyadmin/server
ExecStart=/opt/netyadmin/server/netyadmin-server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable netyadmin
sudo systemctl start netyadmin

# 查看状态
sudo systemctl status netyadmin

# 查看日志
sudo journalctl -u netyadmin -f
```

---

## 五、前端部署

### 5.1 获取代码

```bash
cd NetyAdmin/admin-web
```

### 5.2 安装依赖

```bash
pnpm install
```

### 5.3 配置环境变量

编辑 `.env.prod`（`pnpm build` 以 prod 模式构建时读取）：

```env
# API 基础地址
VITE_SERVICE_BASE_URL=http://your-server-ip:8010
```

### 5.4 构建

```bash
# 开发模式
pnpm dev

# 生产构建
pnpm build
```

### 5.5 部署到 Nginx

安装 Nginx：

```bash
# Ubuntu/Debian
sudo apt-get install nginx

# CentOS/RHEL
sudo yum install nginx

# 启动
sudo systemctl start nginx
```

配置 Nginx `/etc/nginx/sites-available/netyadmin`：

```nginx
server {
    listen 80;
    server_name your-domain.com;

    # 前端静态文件
    location / {
        root /opt/netyadmin/admin-web/dist;
        index index.html;
        try_files $uri $uri/ /index.html;
    }

    # API 代理
    location /admin/v1/ {
        proxy_pass http://localhost:8010;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

启用配置：

```bash
sudo ln -s /etc/nginx/sites-available/netyadmin /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### 5.6 使用 HTTPS（Let's Encrypt）

```bash
# 安装 Certbot
sudo apt-get install certbot python3-certbot-nginx

# 获取证书
sudo certbot --nginx -d your-domain.com

# 自动续期
sudo certbot renew --dry-run
```

---

## 六、Docker 部署

### 6.1 使用 Docker Compose

创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:14-alpine
    environment:
      POSTGRES_DB: netyadmin
      POSTGRES_USER: netyadmin
      POSTGRES_PASSWORD: your_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data
    ports:
      - "6379:6379"

  server:
    build: ./server
    environment:
      - CONFIG_PATH=/app/config.yaml
    volumes:
      - ./server/config.yaml:/app/config.yaml
    ports:
      - "8010:8010"
    depends_on:
      - postgres
      - redis

  web:
    build: ./admin-web
    ports:
      - "80:80"
    depends_on:
      - server

volumes:
  postgres_data:
  redis_data:
```

启动：

```bash
docker-compose up -d
```

### 6.2 服务端 Dockerfile

```dockerfile
# server/Dockerfile
FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o server cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# 迁移 SQL 已通过 embed 打进二进制，无需单独拷贝 migrations 目录
COPY --from=builder /app/server .

EXPOSE 8010

CMD ["./server"]
```

### 6.3 前端 Dockerfile

```dockerfile
# admin-web/Dockerfile
FROM node:20-alpine AS builder

WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN npm install -g pnpm
RUN pnpm install

COPY . .
RUN pnpm build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
```

---

## 七、验证部署

### 7.1 服务端验证

```bash
# 健康检查端点（在路由注册之前独立挂载，不经过业务码流程）
curl http://localhost:8010/health

# 返回 HTTP 200 即健康（hellofresh/health-go 格式，含 db/redis 探活结果）
```

### 7.2 前端验证

访问 `http://your-domain.com`，使用默认账号登录：

- **账号**：`admin`
- **密码**：`admin123`

### 7.3 数据库验证

```bash
sudo -u postgres psql -d netyadmin -c "\dt"
```

---

## 八、常见问题

### 8.1 数据库连接失败

```
错误：dial tcp localhost:5432: connect: connection refused
```

解决方案：

```bash
# 检查 PostgreSQL 状态
sudo systemctl status postgresql

# 检查端口监听
sudo netstat -tlnp | grep 5432

# 检查防火墙
sudo ufw allow 5432
```

### 8.2 迁移失败

```
错误：migration failed: pq: relation "xxx" already exists
```

解决方案：

```yaml
# server/config.yaml
migration:
  enabled: false   # 先关闭自动迁移，修复库状态后再开启
```

> 迁移 SQL 已 embed 进二进制，按版本号幂等执行；手动修复建议用 golang-migrate CLI 对准 `internal/pkg/migration/migrations/` 目录，或直接修复库中的 `schema_migrations` 版本记录。

### 8.3 前端构建失败

```
错误：Cannot find module 'xxx'
```

解决方案：

```bash
# 清除缓存重新安装
rm -rf node_modules pnpm-lock.yaml
pnpm install
```

### 8.4 跨域问题

```
错误：CORS policy: No 'Access-Control-Allow-Origin'
```

解决方案：

```yaml
# server/config.yaml
cors:
  # Origin 精确白名单（空数组 = 拒绝所有跨域，fail-closed）
  allowed_origins:
    - "http://localhost:5173"
    - "https://your-domain.com"
```

---

## 九、生产环境建议

### 9.1 安全配置

1. **修改默认密码**：部署后立即修改 admin 默认密码
2. **使用 HTTPS**：生产环境必须启用 HTTPS
3. **配置防火墙**：仅开放必要端口（80/443/22）
4. **定期备份**：配置数据库自动备份
5. **日志监控**：配置日志收集和告警

### 9.2 性能优化

1. **启用缓存**：配置 Redis 缓存
2. **数据库索引**：定期检查慢查询，优化索引
3. **连接池**：调整数据库连接池大小
4. **静态资源**：使用 CDN 加速静态资源

### 9.3 监控告警

- **错误追踪**：配置 `[sentry].dsn`（可用环境变量 `NETYADMIN_SENTRY_DSN` 覆盖），panic 与错误日志自动上报
- **健康探测**：`/health` 端点含 DB/Redis 探活，可接入 K8s liveness/readiness 或负载均衡
- 配置文件中没有 `[metrics]` Prometheus 端点；如需指标暴露需自行扩展

---

## 十、相关文档

- [Server架构设计](./server-architecture.md)
- [Admin-Web架构设计](./admin-web-architecture.md)
- [API管理指南](./api-management.md)
