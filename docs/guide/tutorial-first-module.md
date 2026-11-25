# 新手教程：从零做一个完整业务模块（product 商品管理）

> 读者：第一次基于 NetyAdmin V2 做业务开发的工程师。
> 目标：跟着做，从一个空骨架得到一个**可迁移、可鉴权、可调用**的完整业务模块（商品管理）。
> 预计：30–45 分钟。每一步都有完整代码、验证命令和预期输出，可以单独停下来核对。
> 配套：[下游开发指南](downstream-development.md) · [下游开发规则（AI 提示词包）](downstream-development-rules.md) · [数据库迁移规范](../rules/migrations.md) · [API 风格](../rules/api-style.md)。

---

## 读前准备

1. 先按 [README · 快速开始（5 分钟）](../../README.md#快速开始5-分钟) 把基座跑起来：建库 → `config.yaml` → 迁移 → 启动 → 用 `admin` 登录一次。
2. 确认 `curl http:/7.0.0.1:8080/health/ready` 返回 `{"status":"ok","checks":{"pg":"ok","redis":"ok"}}`。
3. 本教程只改三处：`internal/modules/product/`（新模块）、`contracts/*.local.yaml`（下游登记）、`cmd/netyadmin/`（三行装配登记）+ `config.yaml`（模块段）。内核、平台服务、接入面代码不动。
4. 命令默认在仓库根目录执行；Windows 用 Git Bash / PowerShell 等价替换即可。

---

## 目标说明

本教程完成一个「商品管理」业务模块 `product`，从零到可用：

| 能力 | 落地位置 |
|------|---------|
| 数据表 `products`（创建/查询/上下架） | 模块迁移 |
| 领域模型 + 仓储接口 + 应用服务 | `domain` / `application` / `adapters` |
| 管理端 HTTP：创建 / 列表 / 详情 / 上下架 | `adapters/adminapi.go` |
| 权限码 + `rbac_apis` 种子 | `api/keys` + 迁移 seed |
| Manifest / 装配 / 迁移源登记 | `module.yaml` + `cmd/netyadmin` |
| 一条命令验证全链路 | 教程 Step 8 |

不在本教程范围（Step 9 给入口）：前端页面、网关开放、WebSocket、领域事件、任务调度。

**最终效果**（Step 8 会实际跑出）：

```bash
curl -s -X POST http:/7.0.0.1:8080/admin/v1/products \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"演示商品","price_cents":1999}'
# → {"code":"10000","message_key":"common.success","data":{"id":"01J...","name":"演示商品","price_cents":1999,"status":1,...}}
```

---

## Step 0 环境与代码基线

**目的**：确认基座可编译、可迁移、可启动，后面每一步的失败都能区分「我的模块问题」和「环境问题」。

```bash
# 1. 依赖：Go ≥ 1.26、PostgreSQL 18、Redis 8（single 拓扑可省）、Node ≥ 20（跑门禁）
go version && node --version && psql --version

# 2. 配置与密钥（只做一次）
cp deploy/config.example.yaml config.yaml
export NETYADMIN_DATABASE_PASSWORD=<你的PG密码>
export NETYADMIN_GW_SECRET_KEY=$(openssl rand -base64 32)

# 3. 迁移并启动
go run ./cmd/netyadmin -mode=migrate -migrate-action=up
go run ./cmd/netyadmin
```

**预期输出**：

- 首次迁移会打印一次 `bootstrap admin created:` 与随机密码（只显示一次，记下来；忘记用 `go run ./cmd/netyadmin -mode=reset-password -username=admin`）。
- 启动日志出现 `HTTP listening` 与各模块装配信息；`curl http:/7.0.0.1:8080/health/ready` 返回 `status: ok`。
- 浏览器打开 `http:/7.0.0.1:8080` 能登录（需要管理端产物 `admin/dist` 已构建；源码开发用 `cd admin && pnpm install && pnpm dev`）。没有前端产物不影响本教程——Step 8 用 curl 完成登录与调用。

**失败排查**：见 [下游文档入口 · 排障速查](README.md#排障速查)。

---

## Step 1 生成模块骨架

**目的**：用脚手架生成与基座同构的标准目录，避免手写遗漏。

```bash
node scripts/scaffold/new-module.mjs product --kind=business --owner=my-team
```

**预期输出**：

```text
[scaffold] 已生成 internal/modules/product/
  module.yaml · module.go（迁移/AttachDB 骨架） · api/{keys,rpc,events} · domain · application · adapters · migrations/
生成后待办（缺一按未完成交付）：
  1. contracts/namespaces.yaml 登记：product 的 prefix 与 keys/rpc/events 包路径
  ...
```

生成的目录（与基座模块同构）：

```text
internal/modules/product/
├── module.yaml
├── module.go                 # 装配骨架（Init/AttachDB）
├── api/keys/keys.go          # key 唯一封装 + 快照测试骨架
├── api/rpc/rpc.go            # 内部 RPC 路由常量
├── api/events/events.go      # 事件 topic 常量
├── domain/doc.go
├── application/doc.go
├── adapters/doc.go
└── migrations/.gitkeep
```

**验证**：

```bash
go build ./internal/modules/product/...
```

预期：无输出（编译通过）。说明：脚手架生成的 `AttachDB` 是 fail-closed 占位——未接线时启动会明确失败，这是故意的，Step 7 会实现。

---

## Step 2 Manifest 声明与模块登记

**目的**：让基座认识这个模块——端口、表、配置段、伸缩模式；并在下游登记面留下唯一记录。

### 2.1 module.yaml（完整替换）

```yaml
name: product
kind: business
version: 1.0.0
api_version: 1
contract_version: "2026-11-22"
min_kernel_version: 2.0.0
owner: my-team
data_classification: internal
slo:
  latency_p99_ms: 200
  availability: 0.999
depends_on: []
optional_depends_on: []
optional_features: {}
ports:
  - name: product.Service
    methods:
      - name: Create
        idempotent: false
        timeout: 2s
        retryable: false
        idempotency_key: optional
      - name: Get
        idempotent: true
        timeout: 2s
        retryable: true
        max_retries: 2
      - name: List
        idempotent: true
        timeout: 2s
        retryable: true
        max_retries: 2
      - name: SetStatus
        idempotent: true
        timeout: 2s
        retryable: true
        max_retries: 2
events:
  publishes: []
  subscribes: []
jobs: []
tables: [products]
config_section: product
scaling:
  mode: stateless
  requires: []
deployment:
  colocate_with: any
  expose_internal_rpc: false
```

字段要点：

| 字段 | 说明 |
|------|------|
| `ports[].name` | 必须是 `<module>.<Service>`，全局唯一；消费方经 `module.ResolveAs` 取 |
| `ports[].methods[].retryable` | 为 `true` 时必须 `idempotent: true` 或 `idempotency_key: required`（Manifest 校验强制） |
| `tables` | 本模块独占表；跨模块同名 Build 直接失败 |
| `config_section` | `config.yaml` 必须存在同名段（空段 `{}` 即满足） |
| `scaling.mode` | `stateless` 默认；有状态/选主/分片按实际声明 |
| `slo` | 每模块 SLO，交付/压测基线 |

### 2.2 config.yaml 增加模块段

在 `config.yaml` 的模块段区域追加：

```yaml
product: {}          # product 模块配置段（空段即满足 config_section 存在性校验）
```

> 启动时会出现一条 `config warning: 未知配置段：product`。这是内核静态段名单的有意保守行为：只影响告警，不影响启动与运行。要消除告警，需在内核段名单追加 `product`（一行登记，不改逻辑）——按你的仓库策略决定是否做。
>
> 注意：若你保留基座全量测试（`go test ./...`），`internal/kernel/config` 的「module.yaml `config_section` ↔ 内核段名单」对偶测试会因新段变红。处理方式二选一：①在内核段名单追加 `product`（一行登记；接受后续升级时的合并成本）；②在你的仓库里按项目段名单调整该对偶测试。运行期行为不受影响。

### 2.3 下游登记（namespaces）

在 `contracts/namespaces.local.yaml` 的 `modules:` 下追加（与 note 样例并列）：

```yaml
  product:
    prefix: product
    owner: my-team
    keys: internal/modules/product/api/keys
    rpc: internal/modules/product/api/rpc
    events: internal/modules/product/api/events
```

**验证**：

```bash
node scripts/ci/check-manifests.mjs
node scripts/ci/check-namespaces.mjs
```

预期：两条都是 `[PASS]`。`check-manifests` 校验必填字段/枚举/`scaling.mode`；`check-namespaces` 校验「有 module.yaml 的模块必须已登记」。

---

## Step 3 数据库迁移（号段 + up/down 对称）

**目的**：用下游号段建 `products` 表，并满足「schema/seed 分离、up/down 成对」的迁移铁律。

### 3.1 号段登记

下游项目占一个首位数字；本教程用「项目 2 / 模块 2」：schema `22000-22299`、索引 `22400-22599`、seed `22600-22799`。
在 `contracts/migrations.local.yaml` 的 `modules:` 下追加：

```yaml
      product:
        module_seq: 2
        schema: "22000-22299"
        structure: "22400-22599"
        seed: "22600-22799"
        test_seed: "22800-22899"
```

### 3.2 表结构（schema）

`internal/modules/product/migrations000_product_products_schema.up.sql`：

```sql
-- +goose Up
-- products：商品（示例业务模块；表归属 product，禁止跨模块 JOIN）。
CREATE TABLE products (
    id          text PRIMARY KEY,                                       -- ULID
    name        text NOT NULL,
    price_cents bigint NOT NULL CHECK (price_cents >= 0),               -- 价格（分）
    status      smallint NOT NULL DEFAULT 1 CHECK (status IN (1, 2)),   -- 1=在售 2=下架
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uk_products_name UNIQUE (name)
);
```

`internal/modules/product/migrations000_product_products_schema.down.sql`：

```sql
-- +goose Down
DROP TABLE IF EXISTS products;
```

### 3.3 二级索引（indexes）

`internal/modules/product/migrations400_product_products_indexes.up.sql`：

```sql
-- +goose Up
CREATE INDEX idx_products_status_created ON products (status, created_at DESC);
```

`internal/modules/product/migrations400_product_products_indexes.down.sql`：

```sql
-- +goose Down
DROP INDEX IF EXISTS idx_products_status_created;
```

### 3.4 种子占位（seed，Step 6 填权限数据）

`internal/modules/product/migrations600_product_products_seed.up.sql`：

```sql
-- +goose Up
-- seed：products 表本身无预置业务数据；权限种子在 Step 6 填入本文件。
SELECT 1;
```

`internal/modules/product/migrations600_product_products_seed.down.sql`：

```sql
-- +goose Down
SELECT 1;
```

**验证**（不落库：在事务里正反各跑一遍）：

```bash
psql -h 127.0.0.1 -U <你的PG用户> -d netyadmin_v2 -v ON_ERROR_STOP=1 <<'SQL'
BEGIN;
\i internal/modules/product/migrations000_product_products_schema.up.sql
\i internal/modules/product/migrations400_product_products_indexes.up.sql
\i internal/modules/product/migrations600_product_products_seed.up.sql
\d products
-- 反向：验证 down 对称
\i internal/modules/product/migrations600_product_products_seed.down.sql
\i internal/modules/product/migrations400_product_products_indexes.down.sql
\i internal/modules/product/migrations000_product_products_schema.down.sql
SELECT to_regclass('public.products') AS dropped_table;   -- 期望空（NULL）
ROLLBACK;
SQL
```

预期：`\d products` 列出 `id/name/price_cents/status/created_at/updated_at` 与唯一约束 `uk_products_name`；最后 `dropped_table` 为空；`ROLLBACK` 后数据库无残留。

> 基座自带的 `check-migrations.mjs` 以基座登记文件与号段为准（`contracts/migrations.yaml`、`11xxx-13xxx`）。下游仓库两种做法：①把脚本的登记源改为合并 `contracts/migrations.local.yaml` 并按 `2xxxx` 规则校验（推荐，最接近基座语义）；②在自己的 CI 里实现等价检查。检查项本身不变：号段归属、schema/seed 分离、up/down 成对、无补丁后缀、模块迁移源已登记。

---

## Step 4 领域模型与仓储（domain / application / adapters）

**目的**：把业务规则放在 domain，把用例放 application，把 PG 实现放 adapters；`domain` 不依赖框架。

### 4.1 领域模型

`internal/modules/product/domain/product.go`（替换 `doc.go` 可保留，另建本文件）：

```go
// product.go：product 领域模型（实体/规则/领域错误）。
// 依赖禁令：不 import 框架、errorx、keys、adapters。
package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// 领域错误（调用方映射状态码；repo 负责把基础设施错误转译成这些哨兵）。
var (
	ErrNotFound      = errors.New("product: product not found")
	ErrNameTaken     = errors.New("product: name taken")
	ErrNameInvalid   = errors.New("product: name invalid")
	ErrPriceInvalid  = errors.New("product: price invalid")
	ErrStatusInvalid = errors.New("product: status invalid")
)

// 商品状态（与 DB CHECK 双闸）。
const (
	StatusOnSale  int16 = 1
	StatusOffSale int16 = 2
)

// 名称/价格规则。
const (
	NameMaxLen    = 64
	PriceMaxCents = 100_000_000 // ¥1,000,000.00（示例上限）
)

// Product 是商品实体。
type Product struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	PriceCents int64     `json:"price_cents"`
	Status     int16     `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ValidName 名称规则：去空白后 1-64 字符。
func ValidName(name string) bool {
	n := strings.TrimSpace(name)
	c := utf8.RuneCountInString(n)
	return c >= 1 && c <= NameMaxLen
}

// ValidPrice 价格规则：0 到上限（单位分）。
func ValidPrice(cents int64) bool { return cents >= 0 && cents <= PriceMaxCents }
```

### 4.2 应用服务

`internal/modules/product/application/service.go`：

```go
// service.go：product 用例层（编排/校验/ID 与时间注入；不 import adapters）。
package application

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/product/domain"
)

// 服务层哨兵转出口（adapters 用 errors.Is 映射状态码）。
var (
	ErrNotFound      = domain.ErrNotFound
	ErrNameTaken     = domain.ErrNameTaken
	ErrNameInvalid   = domain.ErrNameInvalid
	ErrPriceInvalid  = domain.ErrPriceInvalid
	ErrStatusInvalid = domain.ErrStatusInvalid
)

// ProductRepo 是持久化契约（adapters 实现；单测用内存假实现）。
type ProductRepo interface {
	Create(ctx context.Context, p *domain.Product) error
	GetByID(ctx context.Context, id string) (*domain.Product, error)
	List(ctx context.Context, offset, limit int) ([]*domain.Product, int, error)
	SetStatus(ctx context.Context, id string, status int16) error
}

// Service 是商品应用服务。
type Service struct {
	repo   ProductRepo
	logger *slog.Logger
	now    func() time.Time
	id     func() string
}

// New 构造服务（logger 可空——默认 slog.Default）。
func New(logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		logger: logger,
		now:    time.Now,
		id:     func() string { return primitives.NewULID(primitives.SystemClock{}) },
	}
}

// Attach 注入仓储（DB 连接后由 module.AttachDB 调用）。
func (s *Service) Attach(repo ProductRepo) { s.repo = repo }

func (s *Service) repoOK() error {
	if s.repo == nil {
		return errors.New("product: repo not attached")
	}
	return nil
}

// Create 创建商品（名称去空白 + 规则校验；唯一冲突由 repo 转译）。
func (s *Service) Create(ctx context.Context, name string, priceCents int64) (*domain.Product, error) {
	if err := s.repoOK(); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if !domain.ValidName(name) {
		return nil, domain.ErrNameInvalid
	}
	if !domain.ValidPrice(priceCents) {
		return nil, domain.ErrPriceInvalid
	}
	p := &domain.Product{
		ID: s.id(), Name: name, PriceCents: priceCents,
		Status: domain.StatusOnSale, CreatedAt: s.now(), UpdatedAt: s.now(),
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Get 商品详情。
func (s *Service) Get(ctx context.Context, id string) (*domain.Product, error) {
	if err := s.repoOK(); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// List 商品分页列表（offset/limit 已由 transport 钳制）。
func (s *Service) List(ctx context.Context, offset, limit int) ([]*domain.Product, int64, error) {
	if err := s.repoOK(); err != nil {
		return nil, 0, err
	}
	list, total, err := s.repo.List(ctx, offset, limit)
	return list, int64(total), err
}

// SetStatus 上架/下架（域校验 + DB CHECK 双闸）。
func (s *Service) SetStatus(ctx context.Context, id string, status int16) error {
	if err := s.repoOK(); err != nil {
		return err
	}
	if status != domain.StatusOnSale && status != domain.StatusOffSale {
		return domain.ErrStatusInvalid
	}
	return s.repo.SetStatus(ctx, id, status)
}
```

### 4.3 PG 仓储

`internal/modules/product/adapters/repo.go`：

```go
// repo.go：product 持久化（PG；唯一冲突/无行精确转译领域哨兵）。
package adapters

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/modules/product/domain"
)

// Repo 实现 application.ProductRepo。
type Repo struct {
	pool *database.Pool
}

// NewRepo 创建。
func NewRepo(pool *database.Pool) *Repo { return &Repo{pool: pool} }

const productCols = `id, name, price_cents, status, created_at, updated_at`

func scanProduct(row interface{ Scan(...any) error }) (*domain.Product, error) {
	var p domain.Product
	if err := row.Scan(&p.ID, &p.Name, &p.PriceCents, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

// notFound 精确分流：只有 pgx.ErrNoRows 才是 NotFound，DB 故障原样上抛。
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func wrap(op string, err error) error { return fmt.Errorf("product: %s: %w", op, err) }

// Create 实现 application.ProductRepo。
func (r *Repo) Create(ctx context.Context, p *domain.Product) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO products
		(id, name, price_cents, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		p.ID, p.Name, p.PriceCents, p.Status, p.CreatedAt, p.UpdatedAt)
	if database.IsUniqueViolation(err) {
		return domain.ErrNameTaken
	}
	if err != nil {
		return wrap("create", err)
	}
	return nil
}

// GetByID 实现 application.ProductRepo。
func (r *Repo) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	p, err := scanProduct(r.pool.QueryRow(ctx,
		`SELECT `+productCols+` FROM products WHERE id=$1`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return p, nil
}

// List 实现 application.ProductRepo。
func (r *Repo) List(ctx context.Context, offset, limit int) ([]*domain.Product, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM products`).Scan(&total); err != nil {
		return nil, 0, wrap("count", err)
	}
	rows, err := r.pool.Query(ctx, `SELECT `+productCols+` FROM products
		ORDER BY created_at DESC OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return nil, 0, wrap("list", err)
	}
	defer rows.Close()
	var out []*domain.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// SetStatus 实现 application.ProductRepo。
func (r *Repo) SetStatus(ctx context.Context, id string, status int16) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE products SET status=$2, updated_at=now() WHERE id=$1`, id, status)
	if err != nil {
		return wrap("set status", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
```

**验证**：

```bash
go build ./internal/modules/product/...
```

预期：无输出。此时还没有 HTTP 层，也没有接数据库——编译通过即可。

---

## Step 5 HTTP 端点（权限码 + 统一信封 + 分页）

**目的**：把用例暴露为管理端 API；权限码走 `transport.Secured`，请求校验走 `transport.Bind`，响应统一信封。

### 5.1 权限码唯一封装

`internal/modules/product/api/keys/perm.go`（与脚手架生成的 `keys.go` 同包）：

```go
package keys

// 管理面权限码（act 与 rbac_apis 种子同值；check-rbac-perms 对偶校验）。
const (
	PermProductsList  = "admin.product.products.list"
	PermProductsWrite = "admin.product.products.write"
)
```

### 5.2 管理端点

`internal/modules/product/adapters/adminapi.go`：

```go
// adminapi.go：product 管理端点（列表/创建/详情/上下架；统一信封 + 分页 + 权限码）。
package adapters

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/product/api/keys"
	"github.com/netyilei/netyadmin/internal/modules/product/application"
)

// AdminDeps 是管理端点依赖（组合根注入）。
type AdminDeps struct {
	Service    *application.Service
	Authorizer authz.Authorizer
	OnDeny     func(*gin.Context, authn.Principal, string, string)
}

// RegisterAdminRoutes 注册 product 管理端点（挂 /admin/v1 组）。
func RegisterAdminRoutes(authed gin.IRouter, deps AdminDeps) {
	authzMW := transport.AuthzMiddleware(deps.Authorizer, transport.PrincipalFrom, deps.OnDeny)
	secured := func(perm string, h gin.HandlerFunc) []gin.HandlerFunc {
		return transport.Secured(authzMW, perm, h) // RequirePerm → AuthZ → handler（顺序敏感）
	}
	g := authed.Group("/products")
	g.GET("", secured(keys.PermProductsList, listProductsHandler(deps))...)
	g.POST("", secured(keys.PermProductsWrite, createProductHandler(deps))...)
	g.GET("/:id", secured(keys.PermProductsList, getProductHandler(deps))...)
	g.PUT("/:id/status", secured(keys.PermProductsWrite, setStatusHandler(deps))...)
}

type createProductRequest struct {
	Name       string `json:"name" validate:"required,min=1,max=64"`
	PriceCents int64  `json:"price_cents" validate:"gte=0"`
}

// @Summary 商品列表
// @Tags product
// @ID admin.product.products.list
// @Param page query int false "页码"
// @Param page_size query int false "页大小"
// @Success 200 {object} errorx.Envelope{data=transport.Page}
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/products [get]
func listProductsHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, size := transport.BindPage(c)
		list, total, err := deps.Service.List(c.Request.Context(), transport.Offset(page, size), size)
		if err != nil {
			transport.FailOf(c, err)
			return
		}
		transport.OK(c, transport.NewPage(list, total, page, size))
	}
}

// @Summary 创建商品
// @Tags product
// @ID admin.product.products.create
// @Accept json
// @Param body body createProductRequest true "商品名称与价格（分）"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /admin/v1/products [post]
func createProductHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createProductRequest
		if !transport.Bind(c, &req) {
			return
		}
		p, err := deps.Service.Create(c.Request.Context(), req.Name, req.PriceCents)
		if err != nil {
			transport.FailOf(c, mapProductError(err))
			return
		}
		transport.OK(c, p)
	}
}

// @Summary 商品详情
// @Tags product
// @ID admin.product.products.get
// @Param id path string true "商品 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/products/{id} [get]
func getProductHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, err := deps.Service.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			transport.FailOf(c, mapProductError(err))
			return
		}
		transport.OK(c, p)
	}
}

type setStatusRequest struct {
	Status int16 `json:"status" validate:"required"`
}

// @Summary 商品上架/下架
// @Tags product
// @ID admin.product.products.status
// @Accept json
// @Param id path string true "商品 ID"
// @Param body body setStatusRequest true "状态（1=在售 2=下架）"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/products/{id}/status [put]
func setStatusHandler(deps AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req setStatusRequest
		if !transport.Bind(c, &req) {
			return
		}
		if err := deps.Service.SetStatus(c.Request.Context(), c.Param("id"), req.Status); err != nil {
			transport.FailOf(c, mapProductError(err))
			return
		}
		transport.OK(c, gin.H{})
	}
}

// mapProductError 领域哨兵 → 业务码（未命中则原样交给 FailOf 归一）。
func mapProductError(err error) error {
	switch {
	case errors.Is(err, application.ErrNotFound):
		return errorx.New(errorx.CodeNotFound)
	case errors.Is(err, application.ErrNameTaken):
		return errorx.New(errorx.CodeConflict)
	case errors.Is(err, application.ErrNameInvalid),
		errors.Is(err, application.ErrPriceInvalid),
		errors.Is(err, application.ErrStatusInvalid):
		return errorx.New(errorx.CodeValidationFailed)
	}
	return err
}
```

**验证**：

```bash
go build ./internal/modules/product/...
go vet ./internal/modules/product/...
```

预期：无输出。

---

## Step 6 权限数据（rbac_apis 种子）

**目的**：路由声明的权限码必须在 `rbac_apis` 有同值登记，否则只有超管通配能通过、精细角色授权失效。

### 6.1 把权限种子写进 seed

`internal/modules/product/migrations600_product_products_seed.up.sql`（覆盖 Step 3 的占位）：

```sql
-- +goose Up
-- 管理面权限种子：act 与 api/keys 权限码同值（对偶义务）。
INSERT INTO rbac_apis (id, method, path, act, summary) VALUES
    ('rbac-api-product-list',   'GET',  '/admin/v1/products',            'admin.product.products.list',  '商品列表'),
    ('rbac-api-product-create', 'POST', '/admin/v1/products',            'admin.product.products.write', '创建商品'),
    ('rbac-api-product-get',    'GET',  '/admin/v1/products/:id',        'admin.product.products.list',  '商品详情'),
    ('rbac-api-product-status', 'PUT',  '/admin/v1/products/:id/status', 'admin.product.products.write', '商品上下架')
ON CONFLICT (id) DO NOTHING;
```

`internal/modules/product/migrations600_product_products_seed.down.sql`：

```sql
-- +goose Down
DELETE FROM rbac_apis WHERE id IN (
    'rbac-api-product-list', 'rbac-api-product-create',
    'rbac-api-product-get', 'rbac-api-product-status'
);
```

说明：

- `rbac_apis.path` 用 Casbin `keyMatch2` 模式（`:id`），必须与 gin 路由模板一致。
- 只登记、不绑角色：super-admin 由通配行（`act='*'`）天然通过；精细角色在管理端「角色权限」页把 API 授给角色。
- `check-rbac-perms` 的规则是**双向**：路由声明了但种子没登记 → 精细授权失效；种子登记了但无路由声明 → 冗余。两边必须同集合。
- 基座脚本的登记面只扫基座 `125xx` 种子文件；下游在自己的仓库把扫描范围改为你的 seed 号段（如 `226xx`），检查项与双向规则不变。

**验证**（同样在事务里跑，不落库）：

```bash
psql -h 127.0.0.1 -U <你的PG用户> -d netyadmin_v2 -v ON_ERROR_STOP=1 <<'SQL'
BEGIN;
\i internal/modules/product/migrations600_product_products_seed.up.sql
SELECT act FROM rbac_apis WHERE act LIKE 'admin.product.%' ORDER BY act;
-- 期望 2 行：admin.product.products.list / admin.product.products.write
ROLLBACK;
SQL
```

---

## Step 7 装配注册（module.go + 三处登记）

**目的**：把模块接进进程：初始化注册迁移源、暴露 Port、DB 连接后注入仓储；在组合根登记模块、迁移源与路由。

### 7.1 module.go（完整替换脚手架骨架）

`internal/modules/product/module.go`：

```go
// Package product 是商品模块装配入口。
// 职责边界：只做依赖装配与注册（Init 注册迁移/Port），禁止 IO 与业务逻辑。
package product

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	klog "github.com/netyilei/netyadmin/internal/kernel/log"
	"github.com/netyilei/netyadmin/internal/kernel/module"
	"github.com/netyilei/netyadmin/internal/modules/product/adapters"
	"github.com/netyilei/netyadmin/internal/modules/product/application"
)

// all: 纳入 .gitkeep——首迁移落地前目录非空保证编译；Runner 只读 *.sql。
//
//go:embed all:migrations
var migrationsFS embed.FS

// Migrations 返回模块迁移源（cmd/netyadmin/migrate.go moduleMigrations 登记）。
func Migrations() fs.FS { return database.MigrationsOf(migrationsFS) }

// Module 实现 kernel/module 契约。
type Module struct {
	service *application.Service
}

// New 构造模块实例（纯构造禁 IO；DB 依赖经 AttachDB 延迟注入）。
func New(logMgr *klog.Manager) *Module {
	return &Module{service: application.New(logMgr.Logger("product", "app"))}
}

// Name 实现 Module。
func (m *Module) Name() string { return "product" }

// DependsOn 实现 Module。
func (m *Module) DependsOn() []string { return nil }

// Init 纯装配：迁移源 + Port Expose（禁 IO/goroutine）。
func (m *Module) Init(_ context.Context, r *module.Registry) error {
	if err := r.Migrations.Register(m.Name(), Migrations()); err != nil {
		return fmt.Errorf("product: register migrations: %w", err)
	}
	if err := r.Services.Expose("product.Service", m.service); err != nil {
		return fmt.Errorf("product: expose service: %w", err)
	}
	return nil
}

// AttachDB 在 DB 连接后由装配器按接口断言自动调用（repo 注入）。
func (m *Module) AttachDB(pool *database.Pool, _ context.Context) error {
	if pool == nil {
		return errors.New("product: nil pool")
	}
	m.service.Attach(adapters.NewRepo(pool))
	return nil
}

// Service 访问器（单测/诊断用）。
func (m *Module) Service() *application.Service { return m.service }
```

### 7.2 assembly.go：登记模块实例

`cmd/netyadmin/assembly.go`：

1. `import` 区追加：

```go
	"github.com/netyilei/netyadmin/internal/modules/product"
```

2. `registerModules` 内、现有模块 `reg(...)` 序列之后追加：

```go
	productMod := product.New(logMgr)
	if err := reg("internal/modules/product", productMod); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
```

### 7.3 migrate.go：登记迁移源

`cmd/netyadmin/migrate.go`：

1. `import` 区追加：

```go
	"github.com/netyilei/netyadmin/internal/modules/product"
```

2. `moduleMigrations()` 的返回切片追加一行：

```go
		{Module: "product", FS: product.Migrations()},
```

### 7.4 main.go：注册管理端路由

`cmd/netyadmin/main.go`：

1. `import` 区追加：

```go
	productadapters "github.com/netyilei/netyadmin/internal/modules/product/adapters"
	productapp "github.com/netyilei/netyadmin/internal/modules/product/application"
```

2. 在 `adminapi-routes` 钩子内（`usersMod` 注册块之后）追加：

```go
		// product 管理端点：跨模块消费经 Registry Resolve（端口名=product.Service）。
		productSvc, rerr := module.ResolveAs[*productapp.Service](registry.Services, "product.Service")
		if rerr != nil {
			return rerr
		}
		productadapters.RegisterAdminRoutes(authedGroup, productadapters.AdminDeps{
			Service: productSvc, Authorizer: authorizer, OnDeny: onDeny,
		})
```

**验证**：

```bash
go build ./...
go run ./cmd/netyadmin -mode=migrate -migrate-action=status
```

预期：

- `go build ./...` 无输出。
- `status` 输出里出现 `product      applied=0`（登记成功，尚未应用；`applied` 数字随环境下发）。
- 启动 `go run ./cmd/netyadmin` 日志出现 `module db attached module=product`（AttachDB 已自动接线）。

---

## Step 8 跑通验证（迁移 → 启动 → 登录 → 调用）

**目的**：用真实命令验证整条链路：迁移建表、服务装配、权限通过、接口返回统一信封。

### 8.1 迁移

```bash
go run ./cmd/netyadmin -mode=migrate -migrate-action=up
```

预期：无错误输出（若这是你第一次迁移，会打印 bootstrap admin 密码；Step 0 已完成过则不会再打印）。核对：

```bash
psql -h 127.0.0.1 -U <你的PG用户> -d netyadmin_v2 -c "\d products"
psql -h 127.0.0.1 -U <你的PG用户> -d netyadmin_v2 -c \
  "SELECT act FROM rbac_apis WHERE act LIKE 'admin.product.%' ORDER BY act"
```

预期：表存在；权限行 2 条。

### 8.2 启动

```bash
go run ./cmd/netyadmin
```

预期日志：`module db attached module=product`；`config warning: 未知配置段：product`（正常告警）；HTTP 监听 8080。

```bash
curl -s http:/7.0.0.1:8080/health/ready
# → {"status":"ok","checks":{"pg":"ok","redis":"ok"}}
```

### 8.3 登录（验证码一次性）

> 下面用 `jq` 解析 JSON（没装可用 Postman / 浏览器开发者工具替代）；验证码 3 分钟有效且一次性，填错要重新获取。

```bash
# ① 取验证码（一次性，3 分钟有效）
curl -s http:/7.0.0.1:8080/admin/v1/auth/captcha > /tmp/captcha.json
CAPTCHA_ID=$(jq -r '.data.captchaId' /tmp/captcha.json)
jq -r '.data.image' /tmp/captcha.json | sed 's#^data:image/png;base64,##' | base64 -d > /tmp/captcha.png
# ② 打开 /tmp/captcha.png，读出 4 位字符（大小写不敏感）
# ③ 登录换 token
LOGIN=$(curl -s -X POST http:/7.0.0.1:8080/admin/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"admin\",\"password\":\"<首启密码>\",\"captchaId\":\"$CAPTCHA_ID\",\"captchaCode\":\"<图中字符>\"}")
echo "$LOGIN" | jq
TOKEN=$(echo "$LOGIN" | jq -r '.data.access_token')
```

预期：

```json
{
  "code": "10000",
  "message_key": "common.success",
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "...",
    "expires_in": 7200,
    "token_type": "Bearer",
    "must_change_password": true,
    "profile": { "...": "..." }
  }
}
```

> 验证码填错即失效，需要重新 `GET /admin/v1/auth/captcha`。`must_change_password=true` 表示首启密码需要修改（管理端会引导；本教程只验证接口，可暂不修改）。

### 8.4 创建商品

```bash
curl -s -X POST http:/7.0.0.1:8080/admin/v1/products \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"演示商品","price_cents":1999}' | jq
```

预期（HTTP 200）：

```json
{
  "code": "10000",
  "message_key": "common.success",
  "data": {
    "id": "01JXXXXXXXXXXXXXXXXXXXXXXX",
    "name": "演示商品",
    "price_cents": 1999,
    "status": 1,
    "created_at": "2026-11-22T10:00:00+08:00",
    "updated_at": "2026-11-22T10:00:00+08:00"
  }
}
```

再发一次同名请求 → HTTP 409，`code=10004`（唯一冲突映射）。

### 8.5 列表 / 详情 / 上下架

```bash
# 列表（分页契约：page 从 1 起，page_size 默认 20 上限 100）
curl -s "http:/7.0.0.1:8080/admin/v1/products?page=1&page_size=20" \
  -H "Authorization: Bearer $TOKEN" | jq
# → data: {list:[...], total:1, page:1, page_size:20}

# 详情（把 <id> 换成创建返回的 id）
curl -s "http:/7.0.0.1:8080/admin/v1/products/<id>" \
  -H "Authorization: Bearer $TOKEN" | jq

# 下架（1=在售 2=下架）
curl -s -X PUT "http:/7.0.0.1:8080/admin/v1/products/<id>/status" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"status":2}' | jq
# → {"code":"10000","message_key":"common.success","data":{}}
```

### 8.6 鉴权与权限验证

```bash
# 不带 token → 401（token 缺失 10101）
curl -s -o /dev/null -w '%{http_code}\n' http:/7.0.0.1:8080/admin/v1/products
# → 401

# 参数校验：空名称 → 400（10002，details.fields 带字段级规则）
curl -s -X POST http:/7.0.0.1:8080/admin/v1/products \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"","price_cents":1999}' | jq
```

预期：空名称返回 `code=10002`，`details.fields.Name` 说明违反的规则；`price_cents` 为负同样 400。

> 权限精细化：super-admin 由通配行天然通过。若要验证「非超管无权限」，创建一个普通角色管理员、不授予 `admin.product.*`，再调用列表 → 403（`code=10106`）。

---

## Step 9 下一步

1. **前端页面**：管理端是 Vue 3 + Vite + TS（Element Plus）。页面/路由/按钮权限的接入方式见 [`admin/README.md`](../../admin/README.md) 与[下游开发指南 · 三 HTTP API 开发](downstream-development.md#三-http-api-开发)；菜单与按钮权限用迁移 seed 登记（参照 `rbac_menus` / `rbac_buttons` 的种子写法）。
2. **单元测试（质量自担保）**：测试纪律见 [`testing.md`](../rules/testing.md)。本模块的最小单测示例（放 `internal/modules/product/application/service_test.go`）：

```go
package application

import (
	"context"
	"errors"
	"testing"

	"github.com/netyilei/netyadmin/internal/modules/product/domain"
)

type fakeRepo struct{ items map[string]*domain.Product }

func (f *fakeRepo) Create(_ context.Context, p *domain.Product) error {
	for _, it := range f.items {
		if it.Name == p.Name {
			return domain.ErrNameTaken
		}
	}
	cp := *p
	f.items[p.ID] = &cp
	return nil
}

func (f *fakeRepo) GetByID(_ context.Context, id string) (*domain.Product, error) {
	p, ok := f.items[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (f *fakeRepo) List(context.Context, int, int) ([]*domain.Product, int, error) {
	out := make([]*domain.Product, 0, len(f.items))
	for _, p := range f.items {
		out = append(out, p)
	}
	return out, len(out), nil
}

func (f *fakeRepo) SetStatus(_ context.Context, id string, status int16) error {
	p, ok := f.items[id]
	if !ok {
		return domain.ErrNotFound
	}
	p.Status = status
	return nil
}

func TestCreateValidatesAndRejectsDuplicateName(t *testing.T) {
	svc := New(nil)
	svc.Attach(&fakeRepo{items: map[string]*domain.Product{}})

	p, err := svc.Create(context.Background(), "  演示商品  ", 1999)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Name != "演示商品" {
		t.Fatalf("name = %q, want 演示商品", p.Name)
	}
	if _, err := svc.Create(context.Background(), "演示商品", 100); !errors.Is(err, domain.ErrNameTaken) {
		t.Fatalf("duplicate err = %v, want ErrNameTaken", err)
	}
	if _, err := svc.Create(context.Background(), " ", 100); !errors.Is(err, domain.ErrNameInvalid) {
		t.Fatalf("empty name err = %v, want ErrNameInvalid", err)
	}
}
```

```bash
go test ./internal/modules/product/...
```

3. **接事件 / 缓存 / 任务**：见[下游开发指南 · 二使用基座功能](downstream-development.md#二-使用基座功能)（Bus 三域、`kernel.Cache` 实体声明、Scheduler Job 双源登记、请求级 Hook 六段）。
4. **开放给第三方**：见[下游开发指南 · 四加入网关](downstream-development.md#四-加入网关把你的-api-开放给第三方)（治理面登记、HMAC、scope 白名单、18xxx 失败码）。
5. **更新 API 契约**：新增端点后按 [`contracts/openapi/README.md`](../../contracts/openapi/README.md) 重新生成三面 spec（swag 注解 → 切分 → 入库）。不重新生成会让 `check-spec-sync` 报「spec 落后于注解」，`check-openapi` 拦破坏性变更。
6. **接入 CI**：把 [下游开发规则 §4.7 / §8](downstream-development-rules.md#47-测试--ci) 的检查项接进你的工程；号段与 `*.local.yaml` 扫描口径按你的项目调整后，`node scripts/ci/run.mjs` 应保持全绿。

---

## 附：本教程新增/修改文件清单

| 文件 | 动作 |
|------|------|
| `internal/modules/product/module.yaml` | 替换（Step 2） |
| `internal/modules/product/module.go` | 替换（Step 7） |
| `internal/modules/product/domain/product.go` | 新增（Step 4） |
| `internal/modules/product/application/service.go` | 新增（Step 4） |
| `internal/modules/product/adapters/repo.go` | 新增（Step 4） |
| `internal/modules/product/adapters/adminapi.go` | 新增（Step 5） |
| `internal/modules/product/api/keys/perm.go` | 新增（Step 5） |
| `internal/modules/product/migrations000_*` `22400_*` `22600_*` | 新增（Step 3/6） |
| `contracts/namespaces.local.yaml` | 追加 product 登记（Step 2） |
| `contracts/migrations.local.yaml` | 追加 product 号段（Step 3） |
| `config.yaml` | 追加 `product: {}`（Step 2） |
| `cmd/netyadmin/assembly.go` `migrate.go` `main.go` | 各加一处登记（Step 7） |

适用版本 v2.0.0
