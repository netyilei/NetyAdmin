// Package binding 是模块端口的 local/remote 绑定层（——
// 「local：同进程绑定实现；remote：内部 RPC 客户端实现同一接口」）。
// 装配期决策：cmd 组合根按部署拓扑（config.binding 段）为每个跨进程端口选择
// local/remote；调用方只 import 接口（Port），不感知绑定形态。
// 契约测试对 local/remote 跑同一套用例（§13.3；testing.md 契约测试纪律）。
package binding

import (
	"fmt"

	"github.com/netyilei/netyadmin/internal/kernel/module"
)

// Target 是一个端口的绑定声明（config.binding 段条目镜像）。
type Target struct {
	Port     string   // Registry 端口名（如 rbac.Authorizer）
	BaseURLs []string // remote 目标实例清单
}

// Bind 按声明解析端口引用：local=Registry.Resolve 直调（类型断言 T）；
// remote=remoteFactory 构造同一接口的 RPC 适配器（实现在各模块 api/ 包——
// Port 的远程引用面）。
func Bind[T any](services *module.Services, t Target, remoteFactory func(baseURLs []string) (T, error)) (T, error) {
	var zero T
	if len(t.BaseURLs) == 0 {
		v, err := services.Resolve(t.Port)
		if err != nil {
			return zero, fmt.Errorf("binding: local %s: %w", t.Port, err)
		}
		p, ok := v.(T)
		if !ok {
			return zero, fmt.Errorf("binding: local %s: port is %T, want %T", t.Port, v, zero)
		}
		return p, nil
	}
	if remoteFactory == nil {
		return zero, fmt.Errorf("binding: remote %s requires remote factory", t.Port)
	}
	p, err := remoteFactory(t.BaseURLs)
	if err != nil {
		return zero, fmt.Errorf("binding: remote %s: %w", t.Port, err)
	}
	return p, nil
}
