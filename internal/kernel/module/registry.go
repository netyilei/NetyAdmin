// registry.go：Module 契约、Registry 装配与校验（可拆卸装配模型）。
package module

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/netyilei/netyadmin/internal/kernel/bootstrap"
	"github.com/netyilei/netyadmin/internal/kernel/config"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"gopkg.in/yaml.v3"
)

// Module 是模块契约：Init 纯装配，不做 IO/启动 goroutine。
type Module interface {
	Name() string
	DependsOn() []string
	Init(ctx context.Context, r *Registry) error
}

// —— Services（跨模块调用经 Expose/Resolve，替代 edge→业务模块编译依赖） ——

// Services 是端口注册表：模块 Expose 自己的 Port 实现，消费方 Resolve（泛型帮手 ResolveAs）。
type Services struct {
	mu       sync.RWMutex
	services map[string]any
}

func newServices() *Services { return &Services{services: map[string]any{}} }

// Expose 注册端口实现；同名重复注册 fail（唯一归属）。
func (s *Services) Expose(name string, port any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.services[name]; ok {
		return fmt.Errorf("module: port %q already exposed", name)
	}
	s.services[name] = port
	return nil
}

// Resolve 取端口实现；未注册返回错误并列出已注册名（排障）。
func (s *Services) Resolve(name string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.services[name]
	if !ok {
		return nil, fmt.Errorf("module: port %q not registered (registered: %v)", name, s.namesLocked())
	}
	return p, nil
}

func (s *Services) namesLocked() []string {
	names := make([]string, 0, len(s.services))
	for n := range s.services {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Snapshot 返回已 Expose 端口名快照（诊断数据面 bindings 用）。
func (s *Services) Snapshot() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.namesLocked()
}

// ResolveAs 泛型帮手：Resolve + 类型断言（的推荐消费方式）。
func ResolveAs[T any](s *Services, name string) (T, error) {
	var zero T
	p, err := s.Resolve(name)
	if err != nil {
		return zero, err
	}
	t, ok := p.(T)
	if !ok {
		return zero, fmt.Errorf("module: port %q type assertion failed (want %T, got %T)", name, zero, p)
	}
	return t, nil
}

// —— Config（闭环：模块 config_section 绑定） ——

// ConfigBinder 把模块声明的配置段绑定到模块私有配置结构（原始段来自 kernel/config 快照）。
type ConfigBinder struct {
	cfg *config.Config
}

// Bind 将 <section> 原始段绑定到 target（yaml 语义）；段不存在 fail（模块声明必须被配置满足）。
func (c *ConfigBinder) Bind(section string, target any) error {
	raw, ok := c.cfg.Section(section)
	if !ok {
		return fmt.Errorf("module: config section %q not present (config_section not satisfied)", section)
	}
	b, err := yamlMarshal(raw)
	if err != nil {
		return fmt.Errorf("module: config section %q marshal failed: %w", section, err)
	}
	return yamlUnmarshal(b, target)
}

// Exists 段是否存在（可选配置段场景）。
func (c *ConfigBinder) Exists(section string) bool { return c.cfg.HasSection(section) }

// —— 其余 registrar：最小收集器（G1-15：仅记录供诊断/校验，真实现随依赖里程碑替换——
// Routes→、Jobs→/、Events→ BUS、Migrations→、Security→） ——

// RouteEntry 是已注册路由记录（handler 为 any， transport 落地后强类型化）。
type RouteEntry struct {
	Surface string // admin | client | gateway
	Method  string
	Path    string
	Module  string
	Handler any
}

// RouteCollector 收集各模块自注册的路由（edges 不 import 业务模块）。
type RouteCollector struct {
	mu      sync.Mutex
	entries []RouteEntry
}

// Register 记录一条路由（同 surface+method+path 重复 fail）。
func (c *RouteCollector) Register(module, surface, method, path string, handler any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.entries {
		if e.Surface == surface && e.Method == method && e.Path == path {
			return fmt.Errorf("module: duplicate route %s %s (%s vs %s)", method, path, e.Module, module)
		}
	}
	c.entries = append(c.entries, RouteEntry{Surface: surface, Method: method, Path: path, Module: module, Handler: handler})
	return nil
}

// Entries 返回全部路由记录（诊断）。
func (c *RouteCollector) Entries() []RouteEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]RouteEntry(nil), c.entries...)
}

// JobEntry 是已声明 Job 记录（spec 为 any， scheduler 落地后强类型化）。
type JobEntry struct {
	Name   string
	Module string
	Spec   any
}

// JobCollector 收集模块 Job 声明（同名 fail）。
type JobCollector struct {
	mu   sync.Mutex
	jobs []JobEntry
}

// Register 记录 Job。
func (c *JobCollector) Register(module, name string, spec any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, j := range c.jobs {
		if j.Name == name {
			return fmt.Errorf("module: duplicate job %q (%s vs %s)", name, j.Module, module)
		}
	}
	c.jobs = append(c.jobs, JobEntry{Name: name, Module: module, Spec: spec})
	return nil
}

// Entries 返回全部 Job 记录。
func (c *JobCollector) Entries() []JobEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]JobEntry(nil), c.jobs...)
}

// EventKind 区分发布/订阅。
type EventKind string

const (
	EventPublish   EventKind = "publish"
	EventSubscribe EventKind = "subscribe"
)

// EventEntry 是事件目录条目（统一 module+topic+kind 形状）。
type EventEntry struct {
	Module string
	Topic  string
	Kind   EventKind
}

// EventCollector 收集事件发布/订阅声明（事件目录数据源； BUS 消费）。
type EventCollector struct {
	mu      sync.Mutex
	entries []EventEntry
}

// Register 记录事件声明（同 module+topic+kind 重复忽略，多模块订阅同 topic 合法）。
func (c *EventCollector) Register(module string, kind EventKind, topics []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, t := range topics {
		entry := EventEntry{Module: module, Topic: t, Kind: kind}
		dup := false
		for _, e := range c.entries {
			if e == entry {
				dup = true
				break
			}
		}
		if !dup {
			c.entries = append(c.entries, entry)
		}
	}
}

// Snapshot 返回事件目录快照（诊断/§13.1 引用存在性校验）。
func (c *EventCollector) Snapshot() []EventEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]EventEntry(nil), c.entries...)
}

// MigrationCollector 收集模块迁移源（runner 消费；src 为 embed.FS 或等价源）。
type MigrationCollector struct {
	mu   sync.Mutex
	mods []string
	srcs []any
}

// Register 记录模块迁移源（同模块重复 fail）。
func (c *MigrationCollector) Register(module string, src any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.mods {
		if m == module {
			return fmt.Errorf("module: duplicate migration source %q", module)
		}
	}
	c.mods = append(c.mods, module)
	c.srcs = append(c.srcs, src)
	return nil
}

// SecurityEntry 一次 AuthN Provider 登记（模块自注册 → 接入面装配 Chain 消费）。
type SecurityEntry struct {
	Module   string
	Name     string
	Provider authn.Provider
}

// SecurityCollector 收集模块自注册的 AuthN Provider（强类型化；
// AuthZ/RateLimit 强类型随/ 落地追加）。
type SecurityCollector struct {
	mu      sync.Mutex
	entries []SecurityEntry
}

// RegisterAuthN 记录模块声明的 AuthN Provider（name 全局唯一，重复 fail）。
func (c *SecurityCollector) RegisterAuthN(module, name string, p authn.Provider) error {
	if p == nil {
		return fmt.Errorf("module: nil authn provider %q", name)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.entries {
		if e.Name == name {
			return fmt.Errorf("module: duplicate security provider %q", name)
		}
	}
	c.entries = append(c.entries, SecurityEntry{Module: module, Name: name, Provider: p})
	return nil
}

// AuthN 返回已登记 Provider 快照（接入面装配用）。
func (c *SecurityCollector) AuthN() []SecurityEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]SecurityEntry(nil), c.entries...)
}

// —— Registry ——

// Registry 是模块装配与校验中枢（§13.1）。
// Build 按依赖拓扑序执行各模块 Init；失败 fail-fast（进程退出语义，不可重入）。
type Registry struct {
	Services   *Services
	Config     *ConfigBinder
	Hooks      *bootstrap.App // 模块经 Hooks.On(stage,...) 注册生命周期钩子（转发）
	Routes     *RouteCollector
	Jobs       *JobCollector
	Events     *EventCollector
	Migrations *MigrationCollector
	Security   *SecurityCollector

	cfg *config.Config

	mu        sync.Mutex
	modules   map[string]*registered
	order     []string // Init 完成顺序（诊断）
	attempted bool     // Build 已尝试（fail-fast：失败不可重入）
	started   bool
}

type registered struct {
	module Module
	mf     *Manifest
}

// NewRegistry 创建（cfg/app 由 bootstrap 注入；nil 允许——对应 registrar 降级）。
func NewRegistry(cfg *config.Config, app *bootstrap.App) *Registry {
	r := &Registry{
		Services: newServices(),
		cfg:      cfg,
		modules:  map[string]*registered{},
	}
	if cfg != nil {
		r.Config = &ConfigBinder{cfg: cfg}
	}
	if app != nil {
		r.Hooks = app
	}
	r.Routes = &RouteCollector{}
	r.Jobs = &JobCollector{}
	r.Events = &EventCollector{}
	r.Migrations = &MigrationCollector{}
	r.Security = &SecurityCollector{}
	return r
}

// Register 登记模块及其 Manifest（未启用的模块不注册——部署期禁用，§25.3）。
// Build 尝试后调用返回错误（fail-fast 语义：启动失败即进程退出，无重入）。
func (r *Registry) Register(m Module, mf *Manifest) error {
	if m == nil || mf == nil {
		return errors.New("module: module/manifest must not be nil")
	}
	if m.Name() != mf.Name {
		return fmt.Errorf("module: Module.Name()=%q inconsistent with manifest.name=%q", m.Name(), mf.Name)
	}
	if err := mf.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.attempted {
		return errors.New("module: registry build already attempted (fail-fast, no re-entry)")
	}
	if _, ok := r.modules[mf.Name]; ok {
		return fmt.Errorf("module: %q already registered", mf.Name)
	}
	r.modules[mf.Name] = &registered{module: m, mf: mf}
	return nil
}

// OptionalMissing 返回未注册的可选依赖（no-op 适配器语义，诊断用）。
func (r *Registry) OptionalMissing() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for n, reg := range r.modules {
		for _, d := range reg.mf.OptionalDependsOn {
			if _, ok := r.modules[d]; !ok {
				out = append(out, n+" -> "+d)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Build 校验跨模块约束（§13.1：依赖存在/DAG 无环/表归属唯一/端口全局唯一/
// config_section 满足/事件引用存在）并按拓扑序执行 Init。
// 锁内仅快照，校验与 Init 全部锁外执行；失败 fail-fast 不可重入。
func (r *Registry) Build(ctx context.Context) error {
	r.mu.Lock()
	if r.attempted || r.started {
		r.mu.Unlock()
		return errors.New("module: registry build already attempted (fail-fast, no re-entry)")
	}
	r.attempted = true
	snapshot := make(map[string]*registered, len(r.modules))
	for n, reg := range r.modules {
		snapshot[n] = reg
	}
	names := make([]string, 0, len(snapshot))
	for n := range snapshot {
		names = append(names, n)
	}
	sort.Strings(names)
	r.mu.Unlock()

	// ① Module/Manifest 双源一致性 + 依赖存在性 + config_section 满足性
	for _, n := range names {
		reg := snapshot[n]
		mf := reg.mf
		modDeps := reg.module.DependsOn()
		if len(modDeps) != len(mf.DependsOn) {
			return fmt.Errorf("module: %s Module.DependsOn() inconsistent with manifest.depends_on (%v vs %v)", n, modDeps, mf.DependsOn)
		}
		mfSet := map[string]bool{}
		for _, d := range mf.DependsOn {
			mfSet[d] = true
		}
		for _, d := range modDeps {
			if !mfSet[d] {
				return fmt.Errorf("module: %s Module.DependsOn() inconsistent with manifest.depends_on (%v vs %v)", n, modDeps, mf.DependsOn)
			}
			if _, ok := snapshot[d]; !ok {
				return fmt.Errorf("module: %s dependency %q not registered (fail-fast)", n, d)
			}
		}
		if r.cfg != nil && !r.cfg.HasSection(mf.ConfigSection) {
			return fmt.Errorf("module: %s config_section %q not satisfied by configuration", n, mf.ConfigSection)
		}
	}

	// ② 表归属唯一 / 端口全局唯一
	tables := map[string]string{}
	ports := map[string]string{}
	for _, n := range names {
		mf := snapshot[n].mf
		for _, t := range mf.Tables {
			if owner, ok := tables[t]; ok {
				return fmt.Errorf("module: table %q ownership conflict (%s vs %s)", t, owner, n)
			}
			tables[t] = n
		}
		for _, p := range mf.Ports {
			if owner, ok := ports[p.Name]; ok {
				return fmt.Errorf("module: port %q global conflict (%s vs %s)", p.Name, owner, n)
			}
			ports[p.Name] = n
		}
	}

	// ③ DAG 无环（DFS 三色）
	color := map[string]int{}
	var visit func(n string) error
	visit = func(n string) error {
		color[n] = 1 // gray
		for _, dep := range snapshot[n].mf.DependsOn {
			switch color[dep] {
			case 1:
				return fmt.Errorf("module: dependency cycle: reached %s back via %s", n, dep)
			case 0:
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		color[n] = 2 // black
		return nil
	}
	for _, n := range names {
		if color[n] == 0 {
			if err := visit(n); err != nil {
				return err
			}
		}
	}

	// ④ 事件引用存在性（§13.1）：订阅的 topic 必须有人发布或是平台内建事件
	published := map[string]bool{}
	for _, n := range names {
		for _, t := range snapshot[n].mf.Events.Publishes {
			published[t] = true
		}
	}
	for _, n := range names {
		for _, t := range snapshot[n].mf.Events.Subscribes {
			if !published[t] && !strings.HasPrefix(t, "platform.") {
				return fmt.Errorf("module: %s subscribes unknown topic %q (no publisher registered)", n, t)
			}
		}
	}

	// ⑤ 拓扑序 Init（依赖先于被依赖）
	var topo []string
	visited := map[string]bool{}
	var walk func(n string)
	walk = func(n string) {
		if visited[n] {
			return
		}
		visited[n] = true
		for _, dep := range snapshot[n].mf.DependsOn {
			walk(dep)
		}
		topo = append(topo, n)
	}
	for _, n := range names {
		walk(n)
	}

	for _, n := range topo {
		reg := snapshot[n]
		if err := reg.module.Init(ctx, r); err != nil {
			return fmt.Errorf("module: %s Init failed: %w", n, err)
		}
		r.mu.Lock()
		r.order = append(r.order, n)
		r.mu.Unlock()
	}

	// ⑤.1 Job 双源校验：Register 发生于模块 Init，校验必须在 Init 之后才能
	// 双向成立（触发点兑现：Manifest jobs 声明 vs 注册实例——目录
	// 与实例漂移=部署错误，双向 fail-fast）。名字单一源：JobCollector 跨模块
	// 同名即拒，jd.Name 与 job.Name() 同字面（含模块前缀）。
	jobDeclared := map[string]bool{}
	jobRegistered := map[string]bool{}
	for _, n := range names {
		for _, jd := range snapshot[n].mf.Jobs {
			jobDeclared[jd.Name] = true
		}
	}
	for _, je := range r.Jobs.Entries() {
		jobRegistered[je.Name] = true
	}
	for key := range jobDeclared {
		if !jobRegistered[key] {
			return fmt.Errorf("module: job %q declared in manifest but not registered (dual-source)", key)
		}
	}
	for key := range jobRegistered {
		if !jobDeclared[key] {
			return fmt.Errorf("module: job %q registered but not declared in manifest (dual-source)", key)
		}
	}

	r.mu.Lock()
	r.started = true
	r.mu.Unlock()
	return nil
}

// Started 已启动模块名（Init 完成序）。
func (r *Registry) Started() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.order...)
}

// Module 按名取已登记模块实例（装配层 AttachDB/模块级生命周期编排用；
// 未登记返回 false）。
func (r *Registry) Module(name string) (Module, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	reg, ok := r.modules[name]
	if !ok {
		return nil, false
	}
	return reg.module, true
}

// EventsSnapshot 返回事件目录快照（模块 Manifest publishes/subscribes 声明，
// §5.5；诊断面 events 数据源）。
func (r *Registry) EventsSnapshot() []EventEntry {
	return r.Events.Snapshot()
}

// yamlMarshal/yamlUnmarshal 供 ConfigBinder 的 yaml 语义绑定（单点封装）。
func yamlMarshal(v any) ([]byte, error)   { return yaml.Marshal(v) }
func yamlUnmarshal(b []byte, t any) error { return yaml.Unmarshal(b, t) }
