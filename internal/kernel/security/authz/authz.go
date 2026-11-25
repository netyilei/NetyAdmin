// authz.go：统一授权接口——Authorize(ctx, Principal,
// Resource, Action) → Decision。策略类型（RBAC/Scope/Ownership/Route policy）
// 的判定在接入面/端口层执行；权限数据由 rbac 平台服务提供（RB-01）。
// deny 原因进 Decision.Reason（审计细节面）；错误=系统故障（调用方 fail-closed）。
package authz

import (
	"context"

	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
)

// Decision 是一次授权判定结果。
type Decision struct {
	Allowed bool `json:"allowed"`
	// Reason 是 deny 原因（见 Reason* 常量）；allow 时为空。审计记录用，
	// 不外泄给客户端（客户端只见 10106 permission_denied）。
	Reason string `json:"reason,omitempty"`
}

// deny 原因常量（审计/诊断细节面；ReasonNoMatch=策略未命中）。
const (
	ReasonNoMatch      = "no_match"
	ReasonNoSubject    = "no_subject"
	ReasonNotSupported = "not_supported"
	// ReasonNotWired 是中间件面 authorizer 未接线的 deny 原因（fail-closed
	// 装配缺失——与「策略未命中」区分，审计可定位装配遗漏）。
	ReasonNotWired = "not_wired"
	// ReasonSystemError 是 Authorize 系统故障的 deny 原因（fail-closed 判定）。
	ReasonSystemError = "system_error"
)

// Authorizer 是授权判定统一入口。
// 实现：
//   - CasbinAuthorizer：RBAC/路由策略（admin 主体，本包默认实现）；
//   - Scope 白名单（service/app）：调用方直接判 Principal.Scopes（端口级
//     校验，TR-02/ 编排）——非策略引擎语义，不设第二实现；
//   - Ownership（C 端资源属主）：业务语义随 US-01。
type Authorizer interface {
	Authorize(ctx context.Context, p authn.Principal, resource, action string) (Decision, error)
}
