// ipac.go：IP 访问控制领域模型（—— ipac 模块收拢 gw；配置组模型：
// 规则归策略组，应用引用组集；「禁止 > 开放」组集严格合成归引擎）。
package domain

import (
	"errors"
	"time"
)

// IPAction 是规则动作枚举（gw_ip_rules.action smallint 镜像，11257 迁移）。
// allow=白名单（组内命中放行）/deny=黑名单（命中拒绝）；deny 跨组恒优先。
type IPAction int8

const (
	IPActionAllow IPAction = 1
	IPActionDeny  IPAction = 2
)

// Valid 校验枚举（写面校验+DB CHECK 双闸）。
func (a IPAction) Valid() bool { return a == IPActionAllow || a == IPActionDeny }

// IPDecision 是 IP 判定结论（fail-closed：引擎未装载/组集无命中 → deny）。
type IPDecision struct {
	Allowed bool
	// Reason 说明判定来源（组内规则 id / fail_closed / no_group=组集空不限 /
	// no_match=组集非空无命中）。
	Reason string
}

// IPGroup 是 IP 策略组（gw_ip_groups 镜像；应用引用组集复用同一策略）。
type IPGroup struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// IPRule 是 IP 规则（gw_ip_rules 镜像；归组）。
type IPRule struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"groupId"`
	CIDRs     string    `json:"cidrs"`
	Action    IPAction  `json:"action"`
	Priority  int       `json:"priority"`
	Enabled   bool      `json:"enabled"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// 领域错误（管理面写路径）。
var (
	ErrIPActionInvalid  = errors.New("gw: ip action invalid")
	ErrIPCIDRInvalid    = errors.New("gw: ip cidr invalid")
	ErrIPGroupNotFound  = errors.New("gw: ip group not found")
	ErrIPRuleNotFound   = errors.New("gw: ip rule not found")
	ErrIPGroupNameTaken = errors.New("gw: ip group name taken")
	ErrIPGroupInUse     = errors.New("gw: ip group in use by apps")
)
