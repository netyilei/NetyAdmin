// rpcport.go：rbac 模块的 RPC 服务端注册——Authorizer.Authorize 进
// 内部 RPC 宿主（拆分部署下 adminapi 进程经 remote 绑定调用；同进程 binding
// local 不经此面）。DTO/handler 同源在 api/rpc（跨模块契约面）；模块只委托。
package rbac

import (
	"github.com/netyilei/netyadmin/internal/kernel/transport/rpc"
	portrpc "github.com/netyilei/netyadmin/internal/modules/rbac/api/rpc"
)

// RegisterRPC 把 Authorizer.Authorize 注册到内部 RPC 宿主（cmd 装配调用；
// module/port/method/meta 与 api/rpc 契约常量同源）。
func (m *Module) RegisterRPC(server *rpc.Server) error {
	return server.Register("rbac", portrpc.PortName, portrpc.MethodName,
		portrpc.NewAuthorizeHandler(m.service.Authorizer()), rpc.MethodMeta{
			Timeout:    portrpc.MethodTimeout,
			ContractV:  portrpc.ContractV,
			Idempotent: true,
		})
}
