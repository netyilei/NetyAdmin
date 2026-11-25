// authorizer.go：rbac.Authorizer 的内部 RPC 契约面——请求/响应 DTO
// 与远程客户端实现（kernel authz.Authorizer 同接口——local/remote
// 同接口）。调用方（拆分部署下的 adminapi 进程）经 kernel/binding.Bind 拿到此
// client，与 local 实现同签名消费。判定主体=admin ID（角色不进 token——RB-01）。
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/transport/rpc"
)

// 契约常量（server 侧注册与 client 调用同源——单一语义）。
const (
	PortName   = "Authorizer"
	MethodName = "Authorize"
	// ContractV 是端口契约主版本（N-1；破坏式变更升 2 并双版本窗口）。
	ContractV = 1
	// MethodTimeout 是 Authorize 判定上限（Manifest rbac.Authorizer.Authorize 10ms 同源）。
	MethodTimeout = 10 * time.Millisecond
)

// principalDTO 是判定主体的跨进程投影（判定面所需字段；json 显式 tag——
// 跨进程契约不依赖 struct 默认字段名）。
type principalDTO struct {
	Type   string   `json:"type"`
	ID     string   `json:"id"`
	Scopes []string `json:"scopes,omitempty"`
	Act    string   `json:"act,omitempty"`
}

func toDTO(p authn.Principal) principalDTO {
	return principalDTO{Type: string(p.Type), ID: p.ID, Scopes: p.Scopes, Act: p.Act}
}

func (d principalDTO) toPrincipal() authn.Principal {
	return authn.Principal{
		Type: authn.PrincipalType(d.Type), ID: d.ID, Scopes: d.Scopes, Act: d.Act,
	}
}

// authorizeRequest/Response 是 Authorize 的线格式。
type authorizeRequest struct {
	Principal principalDTO `json:"principal"`
	Resource  string       `json:"resource"`
	Action    string       `json:"action"`
}

type authorizeResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// RemoteAuthorizer 是 Authorizer 的远程绑定实现（local/remote 同接口——
// kernel authz.Authorizer）。
type RemoteAuthorizer struct {
	client *rpc.Client
}

// NewRemoteAuthorizer 构造（client 的 BaseURL 指向承载 rbac 模块的服务实例）。
func NewRemoteAuthorizer(client *rpc.Client) *RemoteAuthorizer {
	return &RemoteAuthorizer{client: client}
}

// Authorize 实现 authz.Authorizer（remote）。
func (r *RemoteAuthorizer) Authorize(ctx context.Context, p authn.Principal, resource, action string) (authz.Decision, error) {
	req := authorizeRequest{Principal: toDTO(p), Resource: resource, Action: action}
	var resp authorizeResponse
	// 幂等判定（true）——重试语义与 local 判定一致。
	if err := r.client.Invoke(ctx, "rbac", PortName, MethodName, ContractV, true, req, &resp); err != nil {
		return authz.Decision{}, fmt.Errorf("rbac: remote authorize: %w", err)
	}
	return authz.Decision{Allowed: resp.Allowed, Reason: resp.Reason}, nil
}

// EncodeAuthorizeRequest 是集成测试的请求编码辅助（client 之外构造原始请求）。
func EncodeAuthorizeRequest(p authn.Principal, resource, action string) (json.RawMessage, error) {
	return json.Marshal(authorizeRequest{Principal: toDTO(p), Resource: resource, Action: action})
}

// NewAuthorizeHandler 构造 server 侧方法实现（模块 RegisterRPC 用——DTO/解码/
// 判定分派同源；authorizer 参数=本模块判定面）。
func NewAuthorizeHandler(authorizer authz.Authorizer) rpc.Handler {
	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		var req authorizeRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("rbac: rpc decode request: %w", err)
		}
		return authorizer.Authorize(ctx, req.Principal.toPrincipal(), req.Resource, req.Action)
	}
}
