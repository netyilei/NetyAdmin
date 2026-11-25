// Package api 是 rbac 模块的跨模块端口（授权判定面）。
// 接口定义在 kernel 单一源（security/authz），本包只做模块级消费入口别名——
// 下游 import 本包即可注入 Authorizer 依赖。
package api

import "github.com/netyilei/netyadmin/internal/kernel/security/authz"

// Authorizer 是授权判定端口（注册名 `rbac.Authorizer`）：
//
//	authzr, err := module.ResolveAs[rbacapi.Authorizer](registry.Services, "rbac.Authorizer")
//	err = authzr.Authorize(ctx, principal, "order:create")
//
// 策略经管理端维护并热更广播；下游只做判定。
type Authorizer = authz.Authorizer
