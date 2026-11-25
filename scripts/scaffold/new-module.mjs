#!/usr/bin/env node
// 模块脚手架：生成标准模块骨架（CLI 后置，脚本即入口）。
// 用法：node scripts/scaffold/new-module.mjs <name> [--kind platform-service|business|edge] [--owner platform-team]
// 生成物可直接通过 scripts/ci 门禁（manifests/imports/literals）；生成后需按输出提示登记 contracts。
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = path.dirname(path.dirname(path.dirname(fileURLToPath(import.meta.url))));
const MODULES_DIR = path.join(ROOT, "internal", "modules");

const KINDS = new Set(["platform-service", "business", "edge"]);
const NAME_RE = /^[a-z][a-z0-9]*$/; // 模块名：小写字母开头的字母数字单词（不含下划线/连字符），目录=包名
const USAGE = "用法: node scripts/scaffold/new-module.mjs <name> [--kind=platform-service|business|edge] [--owner=platform-team]";

function parseArgs(argv) {
  const args = { kind: "platform-service", owner: "platform-team" };
  const rest = [];
  for (const a of argv) {
    if (a.startsWith("--kind=")) args.kind = a.slice(7);
    else if (a.startsWith("--owner=")) args.owner = a.slice(8);
    else if (a.startsWith("--")) die(`未知参数：${a}（仅支持 --kind= / --owner=）`);
    else rest.push(a);
  }
  args.name = rest[0];
  return args;
}

function die(msg) {
  console.error(`[scaffold] ${msg}`);
  console.error(USAGE);
  process.exit(1);
}

const args = parseArgs(process.argv.slice(2));
const today = new Date().toISOString().slice(0, 10);
if (!args.name || !NAME_RE.test(args.name)) die(`模块名非法：${args.name}（需匹配 ${NAME_RE}）`);
if (!KINDS.has(args.kind)) die(`kind 非法：${args.kind}（${[...KINDS].join("|")}）`);

if (!args.name) die(USAGE);
const target = path.join(MODULES_DIR, args.name);
if (fs.existsSync(target)) die(`模块已存在：internal/modules/${args.name}`);

const w = (rel, content) => {
  const p = path.join(target, rel);
  fs.mkdirSync(path.dirname(p), { recursive: true });
  fs.writeFileSync(p, content);
};

// ---------- module.yaml（字段；必须过 check-manifests） ----------
w("module.yaml", `name: ${args.name}
kind: ${args.kind}               # platform-service | business | edge
version: 0.1.0
api_version: 1
contract_version: "${today}"     # 契约快照（生成日）
min_kernel_version: 2.0.0
owner: ${args.owner}
data_classification: internal    # public | internal | confidential | pii
slo:                             # docs/rules/slo.md；数值可按模块收紧
  latency_p99_ms: 200
  availability: 0.999            # 取值 0-1（与基座模块同口径；99.9% 写作 0.999）
depends_on: []                   # 下游填写：模块依赖（Registry DAG 校验）
optional_depends_on: []          # 未启用时用 no-op 适配器
optional_features: {}            # 特性级开关仅限同生命周期子功能
ports: []                        # 下游填写：端口与方法元数据 idempotent/timeout/retry（架构 §25.5）
events:
  publishes: []                  # 下游填写：domain.${args.name}.<aggregate>.<action>
  subscribes: []
jobs: []
tables: []                       # 本模块独占表；号段：基座 contracts/migrations.yaml / 下游 *.local.yaml；
                                 # 启用软删的表在 Manifest 声明 soft_delete，大表分区策略写此处（migrations.md）
config_section: ${args.name}
scaling:
  mode: stateless                # stateless | leader | sharded | stateful（AGENTS §3）
  requires: []                   # 部署期依赖的基础设施（如 object_store）
deployment:
  colocate_with: any
  expose_internal_rpc: false
`);

// ---------- module.go（可编译标准骨架：embed 迁移/Init 注册/AttachDB 惯例——
// scaffold 收拢 G1-06：audit/ipac/message/users 四处手写模式收进模板，生成即验收） ----------
w("module.go", `// Package ${args.name} 是 ${args.name} 模块的装配入口。
// 职责边界：只做依赖装配与注册（Init 里注册路由/Job/Hook/事件/迁移/配置），
// 禁止 IO 与业务逻辑；业务在 application，实现细节在 adapters。
package ${args.name}

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/module"
)

// all: 纳入 .gitkeep——首迁移落地前目录非空保证编译；Runner 只读 *.sql，空源无害。
//
//go:embed all:migrations
var migrationsFS embed.FS

// Migrations 返回模块迁移源（cmd/netyadmin/migrate.go moduleMigrations 登记——
// CI check-migrations 对偶门禁强制：有迁移文件的模块必须登记）。
func Migrations() fs.FS { return database.MigrationsOf(migrationsFS) }

// Module 实现 kernel/module 契约。
type Module struct {
	// 下游填写：构造产物（application.Service 等）——New 构造 / Init Expose / AttachDB 注入 repo
}

// New 构造模块实例（纯构造禁 IO；DB 依赖经 AttachDB 延迟注入——audit/message 同惯例）。
func New() *Module { return &Module{} }

// Name 实现 Module。
func (m *Module) Name() string { return "${args.name}" }

// DependsOn 实现 Module。
func (m *Module) DependsOn() []string { return nil }

// Init 纯装配：迁移源/Port Expose（禁 IO/goroutine——DB 依赖由 AttachDB 注入）。
func (m *Module) Init(_ context.Context, r *module.Registry) error {
	if err := r.Migrations.Register(m.Name(), Migrations()); err != nil {
		return fmt.Errorf("${args.name}: register migrations: %w", err)
	}
	// 下游填写：r.Services.Expose / r.Jobs.Register / r.Events.Register（按需）
	return nil
}

// AttachDB 在 DB 连接后由装配调用（repo 注入；cmd attachModules 匿名接口断言）。
// fail-closed：未接线即启动失败——repo 注入后替换 return 为注入实现；模块无表时删除本方法。
func (m *Module) AttachDB(pool *database.Pool, _ context.Context) error {
	// 下游填写：m.service.Attach(adapters.NewRepo(pool)) 后 return nil
	return fmt.Errorf("${args.name}: AttachDB not wired——请在 AttachDB 注入 repo 后返回 nil")
}
`);

// ---------- 三层 doc（依赖禁令随包走） ----------
w("domain/doc.go", `// Package domain 是 ${args.name} 领域层：实体、值对象、领域事件、仓储接口、领域错误。
//
// 依赖禁令（CI check-imports 强制）：禁 import gin/gorm/go-redis/slog、
// kernel/errorx、任何 api/keys、任何 adapters。
// 领域错误用 errors.New 定义在本包，由 application 映射到接口状态码（kernel/errorx）。
// 注意：doc 注释行不要用行首空格缩进（gofmt 会重排）。
package domain
`);

w("application/doc.go", `// Package application 是 ${args.name} 应用层：用例/应用服务、事务边界（UnitOfWork）、编排。
//
// 允许：import 本模块 domain、其他模块 api/ 端口与 DTO、kernel/errorx（错误映射用）。
// 禁止：import 任何 adapters、具体 Redis/GORM 客户端、HTTP 类型（CI check-imports 强制）。
package application
`);

w("adapters/doc.go", `// Package adapters 是 ${args.name} 适配器层：domain 仓储接口实现、外部系统适配（SMTP/OSS/CDN/SMS/MQ）。
//
// 允许：实现 domain 仓储接口与 api/ 端口；import 基础设施客户端（GORM/pgx/go-redis）。
// 禁止：发明状态码（外部错误只做转换，映射在 application）；跨模块 import internal。
package adapters
`);

// ---------- api/ 唯一对外 import 面 ----------
w("api/keys/keys.go", `// Package keys 是 ${args.name} 的 Redis/缓存/锁/限流/幂等 key 唯一封装。
//
// 铁律（docs/rules/namespaces.md）：业务代码禁止裸 key 字符串，一律 import 本包。
// 格式：<root>:<env>:[tenant:]${args.name}:<feature>:<id>[:<sub>]；key 必须带 TTL；禁 PII 进 key。
// 数据 key 不跨模块读（走 Port）；共享基础设施 key 须在 module.yaml 声明 shared_infra_keys。
// builder/常量由本模块按需添加；快照测试见 keys_test.go。
package keys
`);

w("api/rpc/rpc.go", `// Package rpc 是 ${args.name} 的内部 RPC 路由与端口常量唯一封装。
//
// 路由格式：/internal/v1/rpc/${args.name}/{port}/{method}。
// 方法幂等/超时/重试元数据在 module.yaml ports 声明，客户端按元数据执行。
// 骨架示例：首个对外端口时按本包格式添加路由常量。
package rpc
`);

w("api/events/events.go", `// Package events 是 ${args.name} 的领域/平台事件 Topic 常量唯一封装。
//
// 命名：domain.${args.name}.<aggregate>.<action>；SchemaVer 只加字段；
// 事务内发布用 PublishAfterCommit（Outbox）；消费方必须幂等。
// 骨架示例：首个领域事件时按本包格式添加 Topic 常量。
package events
`);

// ---------- migrations 目录（首迁移时再建文件；CI check-migrations 会校验） ----------
w("migrations/.gitkeep", "");

// ---------- keys 快照测试骨架（namespaces.md §3） ----------
w("api/keys/keys_test.go", `package keys

import "testing"

// key 格式快照测试（namespaces.md §3）：新增 builder 后在此登记期望值，
// 防止 key 格式被无声变更。格式：<root>:<env>:${args.name}:<feature>:<id>[:<sub>]
func TestKeysSnapshot(t *testing.T) {
	// snapshot := map[string]string{ ... }
	// for k, want := range snapshot { if got := builder(k); got != want { t.Fatalf(...) } }
	_ = t
}
`);

console.log(`[scaffold] 已生成 internal/modules/${args.name}/`);
console.log(`  module.yaml · module.go（迁移/AttachDB 骨架） · api/{keys,rpc,events} · domain · application · adapters · migrations/`);
console.log(`
生成后待办（缺一按未完成交付）：
  1. contracts/namespaces.yaml 登记：${args.name} 的 prefix 与 keys/rpc/events 包路径
  2. contracts/migrations.yaml 登记：${args.name} 的表序号段（向 owner 申请未占用段）
  3. cmd/netyadmin 登记：assembly.go registerModules 模块实例 + migrate.go moduleMigrations
     迁移源（首个迁移文件落地后必须；check-migrations 对偶门禁双向强制）
  4. module.yaml 填 depends_on / ports / events / tables；scaling.mode 按实际状态选
  5. 跑门禁确认绿：node scripts/ci/run.mjs
  6. 规则文件若变更（contracts），重新 ACK：node scripts/rules/ack.mjs <新hash>`);
