// Package authz 实现授权（AuthZ）：统一 Authorizer（RBAC / Scope / Ownership / 路由策略）。
// 策略引擎 Casbin/v3 SyncedEnforcer；策略变更发布 platform.rbac.policy.changed
// 广播 reload；权限数据由 rbac 平台服务提供，判定在接入面/端口层执行。
package authz
