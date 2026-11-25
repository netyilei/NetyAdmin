// Package rpc 实现内部 RPC：/internal/v1/rpc/{module}/{port}/{method}，HTTP/JSON 起步
// （gRPC/Connect 作为可选适配）；服务 JWT（sub/scp/act/exp≤60s/trace_id）、
// 端口级 AuthZ、超时/重试/熔断（按 Manifest 端口方法元数据）、N-1 兼容。
// 实现：TR-02（服务宿主/发现/绑定：..）。
package rpc
