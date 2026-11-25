// config.go：configcenter 领域模型与存储契约（——动态配置层）。
// 键语义：group+key 唯一；value 按 value_type 语义解析（application 层校验）；
// is_system 键禁删（系统保留面）；is_public 键经公开端点免鉴权读（前端登录页等）。
// 变更面：事务内落 history（变更前整表快照 + 全局 version）→ commit 后广播
// platform.config.changed（G2：缓存失效在 commit 之后——广播必须在事务外）。
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"time"
)

// ValueType 是配置值类型（sys_configs.value_type smallint 镜像）。
type ValueType int8

const (
	TypeString ValueType = 1
	TypeInt    ValueType = 2
	TypeBool   ValueType = 3
	TypeJSON   ValueType = 4
)

// 领域错误。
var (
	// ErrNotFound：配置键不存在。
	ErrNotFound = errors.New("configcenter: config not found")
	// ErrKeyExists：group+key 已存在（创建面）。
	ErrKeyExists = errors.New("configcenter: config key exists")
	// ErrSystemProtected：系统键禁止删除（is_system）。
	ErrSystemProtected = errors.New("configcenter: system config protected")
	// ErrInvalidValue：值与 value_type 不匹配（int/bool/json 解析失败）。
	ErrInvalidValue = errors.New("configcenter: value invalid for type")
	// ErrInvalidKey：键名含 URL 语义字符（key 进 DELETE /configs/:group/:key
	// 路径参数——'/' 断路由、'?#' 截断/进 query， 结构性消除：写面
	// 拒绝而非删除面失败后需手工 DB 清理）。
	ErrInvalidKey = errors.New("configcenter: config key contains invalid characters")
	// ErrReservedGroup：命中保留组（AGENTS §4 不热更面——kernel 进程/拓扑键）。
	ErrReservedGroup = errors.New("configcenter: reserved group")
	// ErrTooManyKeys：键集超量级上限（——写面超限拒绝/读面超限
	// 报错不截断；超限属部署异常，人工介入而非静默扩容）。
	ErrTooManyKeys = errors.New("configcenter: too many config keys")
)

// MaxTotalKeys 是键集量级上限（运维键量级防护——ApplyChange
// 变更后投影超限拒绝，ListAll 读面 LIMIT 超限报错；语义边界见各自 doc）。
const MaxTotalKeys = 10000

// Config 是一条动态配置（表 sys_configs 镜像；json 面向响应——api-style §2）。
type Config struct {
	ID          string    `json:"id"`
	GroupName   string    `json:"group"`
	ConfigKey   string    `json:"key"`
	ConfigValue string    `json:"value"`
	ValueType   ValueType `json:"valueType"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"isSystem"`
	IsPublic    bool      `json:"isPublic"`
	UpdatedBy   string    `json:"updatedBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// HistoryEntry 是一条变更历史（表 config_history 镜像；snapshot=变更前整表）。
type HistoryEntry struct {
	ID        string         `json:"id"`
	Version   int64          `json:"version"`
	Snapshot  []SnapshotItem `json:"snapshot"`
	Reason    string         `json:"reason"`
	CreatedBy string         `json:"createdBy"`
	CreatedAt time.Time      `json:"createdAt"`
}

// SnapshotItem 是快照里的一条配置（jsonb 序列化契约；description 入快照——
// 回滚恢复原描述，评审复项）。
type SnapshotItem struct {
	GroupName   string `json:"group"`
	ConfigKey   string `json:"key"`
	ConfigValue string `json:"value"`
	ValueType   int8   `json:"valueType"`
	Description string `json:"description"`
	IsSystem    bool   `json:"isSystem"`
	IsPublic    bool   `json:"isPublic"`
}

// ReservedGroups 是热更边界保留组（AGENTS §4：监听地址/DSN/Redis 地址/模块
// 启停**不热更**——这些面的动态化被显式禁止；创建/更新键时命中即拒）。
// 当前保留组即本清单；扩展随具名禁改面出现。
var ReservedGroups = map[string]bool{
	"kernel": true, // 进程/内核面（监听地址/拓扑/Redis—— 分层 env-only 或启动期）
}

// ChangeKind 是变更操作类型（ApplyChange 单事务原语的写面）。
type ChangeKind int8

const (
	ChangeCreate   ChangeKind = 1
	ChangeUpdate   ChangeKind = 2
	ChangeDelete   ChangeKind = 3
	ChangeRollback ChangeKind = 4
)

// Change 是一次变更请求（ApplyChange 输入；Items 支持——批量 upsert 整批一版）。
type Change struct {
	Kind       ChangeKind
	Configs    []Config // create/update/upsert 的键集（rollback/delete 为空）
	Group      string   // delete 目标
	Key        string   // delete 目标
	RollbackTo int64    // rollback 目标版本（repo 内读快照写回）
	Reason     string
	By         string
}

// Repo 是存储契约（application 定义、adapters 实现——依赖向内）。
// 写面收拢为 ApplyChange 单事务原语（G1-16 根修：版本分配+快照落史+写操作
// 同事务——apply 失败整体回滚零孤儿；advisory lock 串行化版本分配）。
type Repo interface {
	// ListHistory 最近变更历史。
	ListHistory(ctx context.Context, limit int) ([]HistoryEntry, error)
	// ListAll 全量配置（watcher 快照装载）。
	ListAll(ctx context.Context) ([]Config, error)
	// ListPublic 公开键集（免鉴权读面）。
	ListPublic(ctx context.Context) ([]Config, error)
	// Get 单键（group+key 唯一定位）。
	Get(ctx context.Context, group, key string) (Config, error)

	// ApplyChange 单事务变更：advisory lock 串行化 → 变更前快照落 history
	// （version=事务内 max+1）→ 写操作 → commit；返回新版本。
	// 业务冲突（键存在/不存在/系统保护）在事务内判定并整体回滚。
	ApplyChange(ctx context.Context, ch Change) (int64, error)

	// CurrentVersion 当前全局版本（无历史=0）。
	CurrentVersion(ctx context.Context) (int64, error)
	// GetHistory 按版本取历史（回滚读源/审计查询随消费者）。
	GetHistory(ctx context.Context, version int64) (HistoryEntry, error)
}

// keyPattern 键字符集白名单（字母数字与 .:_-，长度 1-100 对齐 bind tag——
// 现有键形如 log:module.transport；URL 语义字符 /?#%& 禁入）。
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,100}$`)

// Validate 校验键字符集与值/类型匹配（string 恒过；int/bool/json 解析验证）。
func (c Config) Validate() error {
	if !keyPattern.MatchString(c.ConfigKey) {
		return ErrInvalidKey
	}
	switch c.ValueType {
	case TypeString, TypeJSON:
		if c.ValueType == TypeJSON && !json.Valid([]byte(c.ConfigValue)) {
			return ErrInvalidValue
		}
		return nil
	case TypeInt:
		if _, err := json.Number(c.ConfigValue).Int64(); err != nil {
			return ErrInvalidValue
		}
		return nil
	case TypeBool:
		if c.ConfigValue != "true" && c.ConfigValue != "false" {
			return ErrInvalidValue
		}
		return nil
	default:
		return ErrInvalidValue
	}
}
