// Package rpc 是 configcenter 的内部 RPC 路由唯一封装（/internal/v1/rpc/configcenter/...）。
// 当前无 remote 绑定消费方（跨节点配置读取走 DB+Bus reload 同链）；随
// 拆分部署需求登记端口路由常量；禁止他处裸写路由。
package rpc
