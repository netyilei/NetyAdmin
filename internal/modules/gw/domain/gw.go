// gw.go：gateway 治理领域模型与存储契约（V1 open_platform 对齐——
// §18 governance/edge 两上下文的数据面归属本模块，edge HMAC 面消费
// 本模块 Port）。secret 加密面：AES-256-GCM（primitives；key=Secret 注入），
// 明文只在创建/重置的响应通道一次性出现。
package domain

import (
	"context"
	"errors"
	"time"
)

// Status 是 app/scope 组/API 状态位镜像。
type Status int8

const (
	StatusEnabled  Status = 1
	StatusDisabled Status = 2
)

// QuotaConfig 是 App 限流桶配置（jsonb；滑窗语义——/面级中间件消费；
// Rate<=0=不限流（显式无限）。V1 的 capacity 字段未随 V2 滑窗消费——不再下发）。
type QuotaConfig struct {
	Rate int `json:"rate"` // 每秒请求数
}

// App 是开放平台应用实体。IP 管控唯一面=IPGroupIDs 组集（——组集空=
// 不做 IP 判定，非空=引擎严格合成；无独立开关/策略字段——单一事实源）。
type App struct {
	ID               string
	AppKey           string
	AppSecretEnc     string // AES-GCM 密文（永不出 domain 明文面）
	Name             string
	Status           Status
	RateLimitEnabled bool
	Quota            QuotaConfig
	Remark           string
	IPGroupIDs       []string
	StorageRef       string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ScopeGroup 是 scope 组定义（出口 camelCase json——契约与线格式单一源；
// 接口权限页/应用表单同口径）。
type ScopeGroup struct {
	ID          string    `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Api 是网关 API 判定登记（出口 camelCase json）。
type Api struct {
	ID          string    `json:"id"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Grp         string    `json:"grp"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// AppView 是对外投影（脱敏——secret 密文/明文都不出；scopes/ipGroupIds 聚合展示）。
type AppView struct {
	ID               string      `json:"id"`
	AppKey           string      `json:"appKey"`
	Name             string      `json:"name"`
	Status           Status      `json:"status"`
	RateLimitEnabled bool        `json:"rateLimitEnabled"`
	Quota            QuotaConfig `json:"quota"`
	Remark           string      `json:"remark"`
	StorageRef       string      `json:"storageRef"`
	IPGroupIDs       []string    `json:"ipGroupIds"`
	Scopes           []string    `json:"scopes"`
	CreatedAt        time.Time   `json:"createdAt"`
	UpdatedAt        time.Time   `json:"updatedAt"`
}

// 领域错误。
var (
	ErrNotFound        = errors.New("gw: not found")
	ErrAppKeyTaken     = errors.New("gw: app key taken")
	ErrScopeNotGroup   = errors.New("gw: scope not in groups") // app 授权 scope 必须是已定义组
	ErrDisabled        = errors.New("gw: app disabled")
	ErrScopeNotFound   = errors.New("gw: scope group not found")
	ErrScopeCodeTaken  = errors.New("gw: scope code taken")
	ErrScopeGroupInUse = errors.New("gw: scope group in use by apps")
)

// Repo 是治理存储契约（application 定义、adapters 实现）。
type Repo interface {
	// —— Apps ——
	ListApps(ctx context.Context, offset, limit int) ([]App, int64, error)
	// ListAllForSnapshot 全量应用行（edge 判定快照装载面——含禁用行；secret
	// 密文原样返回，解密在装载侧）。与 ListApps 共享行扫描口径（J-1 单点）。
	ListAllForSnapshot(ctx context.Context) ([]App, error)
	GetApp(ctx context.Context, id string) (App, error)
	GetAppByKey(ctx context.Context, appKey string) (App, error)
	// InsertAppWithScopes 创建 App+初始 scopes 单事务（安-F7：两步写非原子，
	// scopes 写失败遗留无授权 App；app_key 唯一约束冲突 → ErrAppKeyTaken——
	// 预检窗口外并发重名 409 而非 500）。
	InsertAppWithScopes(ctx context.Context, a App, scopes []string) error
	UpdateApp(ctx context.Context, a App) error
	// UpdateAppStatus 状态位独立写（/status 端点——UpdateApp 不含 status，
	// 更新面不可复活禁用 App）。
	UpdateAppStatus(ctx context.Context, id string, status Status) error
	UpdateAppSecretEnc(ctx context.Context, id, secretEnc string) error
	DeleteApp(ctx context.Context, id string) error
	// AppScopes 聚合读（展示投影）。
	AppScopes(ctx context.Context, appID string) ([]string, error)
	// UpdateAppWithScopes 基础字段+scopes 全量替换单事务（应用表单单一保存
	// 入口——两步写非原子：scopes 写失败遗留半更新状态）。
	UpdateAppWithScopes(ctx context.Context, a App, scopes []string) error

	// —— Scope groups ——
	ListScopeGroups(ctx context.Context) ([]ScopeGroup, error)
	GetScopeGroup(ctx context.Context, code string) (ScopeGroup, error)
	GetScopeGroupByID(ctx context.Context, id string) (ScopeGroup, error)
	InsertScopeGroup(ctx context.Context, g ScopeGroup) error
	UpdateScopeGroup(ctx context.Context, g ScopeGroup) error
	// DeleteScopeGroup 删除（占用守卫事务化：条件删除内联引用判定——
	// gw_app_scopes 按 code / gw_scope_apis 按 id 任一引用 → ErrScopeGroupInUse；
	// 组不存在或并发已删 → ErrScopeNotFound；判定与删除同语句原子）。
	DeleteScopeGroup(ctx context.Context, id string) error

	// —— APIs ——
	ListApis(ctx context.Context) ([]Api, error)
	GetApi(ctx context.Context, id string) (Api, error)
	InsertApi(ctx context.Context, a Api) error
	UpdateApi(ctx context.Context, a Api) error
	DeleteApi(ctx context.Context, id string) error

	// —— Scope→API 白名单 ——
	ScopeApiIDs(ctx context.Context, scopeID string) ([]string, error)
	SetScopeApis(ctx context.Context, scopeID string, apiIDs []string) error
}

// Validate 校验 quota 合法性（rate/capacity 非负）。
func (q QuotaConfig) Validate() error {
	if q.Rate < 0 {
		return errors.New("gw: quota negative")
	}
	return nil
}
