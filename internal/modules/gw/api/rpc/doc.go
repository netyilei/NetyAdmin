// Package rpc 是 gw 的内部 RPC 路由唯一封装（/internal/v1/rpc/gw/...）。
// edge 面经 module Port 直调（同进程绑定）——remote 绑定随拆分部署需求登记；
// 禁止他处裸写路由。
package rpc
