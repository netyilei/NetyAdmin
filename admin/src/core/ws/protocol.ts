// WS 协议常量（前端镜像）。后端唯一源 internal/modules/realtime/api/protocol
// （types.go/envelope.go）+ contracts/status-codes.yaml 22xxx 段；本文档为其 TS
// 投影——契约变更须同步，禁止手写字符串/裸关闭码（AGENTS.md §2/§6）。
// 契约：docs/rules/ws-protocol.md §3 信封 / §4 帧类型 / §5 关闭码。

/** 信封协议版本（protocol.SchemaVersion 镜像——只加字段不破坏）。 */
export const WS_PROTOCOL_VERSION = 1;

/** 单帧负载上限（ws-protocol §6；服务端超限 100904）。 */
export const WS_MAX_PAYLOAD_BYTES = 64 * 1024;

/** 频道名长度上限（ws-protocol §2）。 */
export const WS_CHANNEL_MAX_LEN = 128;

/** 心跳间隔默认值（ms；ws-protocol §6 规范基线——实际以 hello 帧 heartbeat_interval 为准）。 */
export const WS_HEARTBEAT_INTERVAL_MS = 30_000;

/** 帧类型（ws-protocol §4——types.go 逐字镜像；收发共用同一常量表）。 */
export const WS_FRAME = {
  // 客户端 → 服务端
  SUBSCRIBE: "subscribe",
  UNSUBSCRIBE: "unsubscribe",
  MESSAGE: "message",
  PING: "ping",
  ACK: "ack",
  RESUME: "resume",
  // 服务端 → 客户端
  HELLO: "hello",
  SUBSCRIBED: "subscribed",
  UNSUBSCRIBED: "unsubscribed",
  PONG: "pong",
  ERROR: "error",
  CLOSE: "close"
} as const;

/** 关闭码（ws-protocol §5——protocol/close.go 逐字镜像）。 */
export const WS_CLOSE_CODE = {
  NORMAL: 1000, // 客户端主动正常关闭
  GOING_AWAY: 1001, // 服务端优雅关闭/滚动升级/空闲超时
  POLICY_VIOLATION: 1008, // 鉴权失败/频道越权
  TRY_AGAIN_LATER: 1013, // 过载/背压/限流（慢消费者）
  AUTH_EXPIRED: 4001, // ticket/JWT 过期
  CHANNEL_DENIED: 4002, // 频道 ACL 拒绝
  RATE_LIMITED: 4003, // 消息速率超限
  PAYLOAD_TOO_LARGE: 4004, // 单帧超限
  DUPLICATE_CONN: 4005, // 同身份连接数超限
  PROTOCOL_VIOLATION: 4006, // 非法帧/未知类型
  SESSION_REVOKED: 4007, // 令牌吊销/账号禁用（终态——不重连）
  SERVER_ERROR: 4008 // 服务端内部错误
} as const;

/** admin 面 WS error 帧业务码（contracts/status-codes.yaml surface=admin domain=ws）。 */
export const WS_ADMIN_ERROR_CODE = {
  CHANNEL_DENIED: "22001",
  INVALID_CHANNEL: "22002",
  PROTOCOL_VIOLATION: "22003",
  CHANNEL_LIMIT: "22004",
  RESUME_UNAVAILABLE: "22005",
  NOT_SUBSCRIBED: "22006"
} as const;

/** WS 帧信封（ws-protocol §3——envelope.go 镜像；未知字段必须忽略，向前兼容）。 */
export interface WsEnvelope {
  id: string; // 消息唯一 ID（幂等/去重）
  type: string; // WS_FRAME 常量
  channel?: string; // 订阅/消息类必填
  payload?: unknown; // 业务负载 ≤64KB
  ts: number; // 服务端毫秒时间戳
  request_id?: string;
  trace_id?: string;
  schema_version: number;
  seq?: number; // 频道内序号（排序/续传）
  code?: string; // error 帧：5 位业务码
  message_key?: string; // error 帧：i18n 键
  close_code?: number; // close 帧：§5 关闭码
  reason?: string; // close 帧：原因
}

/** hello 帧负载（session.go：conn_id/protocol_version/heartbeat_interval 毫秒）。 */
export interface WsHelloPayload {
  conn_id: string;
  protocol_version: number;
  heartbeat_interval: number;
}

/** ticket 签发响应（session.go TicketHandler：transport.OK 信封 data）。 */
export interface WsTicketData {
  ticket: string;
  expires_in: number;
}
