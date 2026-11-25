// Package rpc 是 note 的内部 RPC 路由与端口常量唯一封装。
//
// 路由格式：/internal/v1/rpc/note/{port}/{method}。
// 方法幂等/超时/重试元数据在 module.yaml ports 声明，客户端按元数据执行。
// 骨架示例：首个对外端口时按本包格式添加路由常量。
package rpc
