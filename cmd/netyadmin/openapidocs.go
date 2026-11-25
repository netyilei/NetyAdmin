//lint:file-ignore U1000 swag 注解载体（生成器消费，运行时零引用——见下方锚点原因）

// openapidocs.go：client/gateway 面 swagger 注解锚点（仅文档面——无运行时逻辑）。
//
// 为什么要锚点：这两面的处理器是共享实现——WS ticket/upgrade 是 realtime
// session.Surface 的方法（同一 handler 分别挂 admin/client/gateway 三组），
// 网关业务转发是 access.go 单一分派器（一个闭包内三分支、method 全通配）。
// 注解必须按「挂载面+实际路径+HTTP method」落在组合根侧，且每个 method 需唯一
// operationId；swag 要求注解紧贴函数声明且一块注解只出一个 ID，故以空函数承载
// 逐 method 锚点（同 operations.go envelopeSwaggerAlias 的 import 锚点惯例，
// 「注解贴着路由代码」在此处的唯一可行落点）。
package main

import "github.com/netyilei/netyadmin/internal/kernel/errorx"

// openAPIEnvelope 是 swagger 注解的 import 锚点（本文件无运行时 errorx 消费）。
type openAPIEnvelope = errorx.Envelope

// wsTicketDocRequest 是 ticket 签发请求体的文档 DTO（与 session.TicketHandler
// 内部匿名结构同形——swag 无法引用函数内局部类型）。
type wsTicketDocRequest struct {
	Channels []string `json:"channels"`
}

// @Summary 客户端 WebSocket 升级
// @ID client.ws.upgrade
// @Description ticket 一次性凭证换连接（ws-protocol §1；realtime 禁用时 503 fail-closed）
// @Tags ws-client
// @Produce json
// @Param ticket query string true "POST /client/v1/ws/tickets 签发的一次性 ticket"
// @Success 101 "Switching Protocols（WS 帧协议见 ws-protocol §3-§7）"
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /client/v1/ws [get]
func clientWSUpgradeSwaggerDoc() {}

// @Summary 客户端 WebSocket ticket 签发
// @ID client.ws.ticket
// @Description User JWT 换取一次性 ticket（绑定身份+允许频道集合；空 channels=订阅决策留给服务端 ACL）
// @Tags ws-client
// @Accept json
// @Produce json
// @Param body body wsTicketDocRequest false "订阅意图（可省略）"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Router /client/v1/ws/tickets [post]
func clientWSTicketSwaggerDoc() {}

// @Summary 网关 WebSocket 升级
// @ID gateway.ws.upgrade
// @Description ticket 一次性凭证换连接（edge HMAC 管线签发的 ticket；realtime 禁用时 503 fail-closed）
// @Tags gateway
// @Produce json
// @Param ticket query string true "POST /gateway/v1/ws/tickets 签发的一次性 ticket"
// @Success 101 "Switching Protocols（WS 帧协议见 ws-protocol §3-§7）"
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /gateway/v1/ws [get]
func gatewayWSUpgradeSwaggerDoc() {}

// @Summary 网关 WebSocket ticket 签发
// @ID gateway.ws.ticket
// @Description App 身份（edge HMAC）换取一次性 ticket；channels 受 App ws.channel.* scope 约束
// @Tags gateway
// @Accept json
// @Produce json
// @Param X-NA-App-Id header string true "AppKey（应用标识）"
// @Param X-NA-Timestamp header string true "Unix 秒级时间戳"
// @Param X-NA-Nonce header string true "一次性随机串"
// @Param X-NA-Signature header string true "HMAC-SHA256 签名（canonical 见 downstream-development §4）"
// @Param body body wsTicketDocRequest false "订阅意图（可省略）"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /gateway/v1/ws/tickets [post]
func gatewayWSTicketSwaggerDoc() {}

// —— 网关业务转发（5 method 各一 operationId；实际=gatewayDispatcher 的 default 分支）——

// @Summary 网关业务 API 转发（GET）
// @ID gateway.api.forward.get
// @Description 完整网关 path 经验签+scope 白名单后原样转发到进程内 client 路由；内部 path 须先在 gw_apis 登记
// @Tags gateway
// @Produce json
// @Param path path string true "内部路径（如 client/v1/echo——剥 /gateway/v1 前缀后的形态）"
// @Param X-NA-App-Id header string true "AppKey（应用标识）"
// @Param X-NA-Timestamp header string true "Unix 秒级时间戳"
// @Param X-NA-Nonce header string true "一次性随机串（防重放窗内唯一）"
// @Param X-NA-Signature header string true "HMAC-SHA256 签名（canonical 见 downstream-development §4）"
// @Success 200 {object} errorx.Envelope "下游端点原始响应（信封形态随目标端点）"
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 429 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /gateway/v1/{path} [get]
func gatewayAPIForwardGetSwaggerDoc() {}

// @Summary 网关业务 API 转发（POST）
// @ID gateway.api.forward.post
// @Description 签名 payload=body SHA256 hex；其余同 GET
// @Tags gateway
// @Accept json
// @Produce json
// @Param path path string true "内部路径（如 client/v1/users/login）"
// @Param X-NA-App-Id header string true "AppKey（应用标识）"
// @Param X-NA-Timestamp header string true "Unix 秒级时间戳"
// @Param X-NA-Nonce header string true "一次性随机串（防重放窗内唯一）"
// @Param X-NA-Signature header string true "HMAC-SHA256 签名（canonical 见 downstream-development §4）"
// @Success 200 {object} errorx.Envelope "下游端点原始响应（信封形态随目标端点）"
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 429 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /gateway/v1/{path} [post]
func gatewayAPIForwardPostSwaggerDoc() {}

// @Summary 网关业务 API 转发（PUT）
// @ID gateway.api.forward.put
// @Description 签名 payload=body SHA256 hex；其余同 GET
// @Tags gateway
// @Accept json
// @Produce json
// @Param path path string true "内部路径（如 client/v1/users/phone）"
// @Param X-NA-App-Id header string true "AppKey（应用标识）"
// @Param X-NA-Timestamp header string true "Unix 秒级时间戳"
// @Param X-NA-Nonce header string true "一次性随机串（防重放窗内唯一）"
// @Param X-NA-Signature header string true "HMAC-SHA256 签名（canonical 见 downstream-development §4）"
// @Success 200 {object} errorx.Envelope "下游端点原始响应（信封形态随目标端点）"
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 429 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /gateway/v1/{path} [put]
func gatewayAPIForwardPutSwaggerDoc() {}

// @Summary 网关业务 API 转发（DELETE）
// @ID gateway.api.forward.delete
// @Description 其余同 GET
// @Tags gateway
// @Produce json
// @Param path path string true "内部路径"
// @Param X-NA-App-Id header string true "AppKey（应用标识）"
// @Param X-NA-Timestamp header string true "Unix 秒级时间戳"
// @Param X-NA-Nonce header string true "一次性随机串（防重放窗内唯一）"
// @Param X-NA-Signature header string true "HMAC-SHA256 签名（canonical 见 downstream-development §4）"
// @Success 200 {object} errorx.Envelope "下游端点原始响应（信封形态随目标端点）"
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 429 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /gateway/v1/{path} [delete]
func gatewayAPIForwardDeleteSwaggerDoc() {}

// @Summary 网关业务 API 转发（PATCH）
// @ID gateway.api.forward.patch
// @Description 签名 payload=body SHA256 hex；其余同 GET
// @Tags gateway
// @Accept json
// @Produce json
// @Param path path string true "内部路径"
// @Param X-NA-App-Id header string true "AppKey（应用标识）"
// @Param X-NA-Timestamp header string true "Unix 秒级时间戳"
// @Param X-NA-Nonce header string true "一次性随机串（防重放窗内唯一）"
// @Param X-NA-Signature header string true "HMAC-SHA256 签名（canonical 见 downstream-development §4）"
// @Success 200 {object} errorx.Envelope "下游端点原始响应（信封形态随目标端点）"
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 429 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /gateway/v1/{path} [patch]
func gatewayAPIForwardPatchSwaggerDoc() {}
