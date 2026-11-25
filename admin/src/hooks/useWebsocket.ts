// WS 组合式封装（src/core/ws/client.ts 共享单例之上的 Vue 消费面）。
// 单例共享连接（4005 同身份上限约束下不各自建连）；监听随组件作用域自动解除；
// 连接生命周期由调用方驱动（autoConnect 可选）——WS 推送面后续按需接入。
import { getCurrentScope, onScopeDispose, ref, type Ref } from "vue";

import { getWsClient, type WsErrorInfo, type WsStatus } from "@/core/ws/client";
import type { WsEnvelope } from "@/core/ws/protocol";

export interface UseWebsocketOptions {
  /** 挂载即建连（默认 false——由调用方在合适时机 connect）。 */
  autoConnect?: boolean;
  /** 业务消息回调（message 帧——随作用域自动解除）。 */
  onMessage?: (frame: WsEnvelope) => void;
  /** 错误回调（error 帧/鉴权终态/连接层关闭——随作用域自动解除）。 */
  onError?: (err: WsErrorInfo) => void;
}

export interface UseWebsocketReturn {
  /** 连接状态（响应式）。 */
  status: Ref<WsStatus>;
  /** 服务端分配的连接 ID（hello 帧 conn_id；离线为空串）。 */
  connId: Ref<string>;
  /** 心跳周期（ms；hello 帧 heartbeat_interval，未握手为规范基线 30s）。 */
  heartbeatMs: Ref<number>;
  connect: () => Promise<void>;
  close: () => void;
  subscribe: (channel: string) => Promise<number | undefined>;
  unsubscribe: (channel: string) => Promise<void>;
  /** 追加业务消息监听（随作用域自动解除）。 */
  onMessage: (handler: (frame: WsEnvelope) => void) => void;
}

/**
 * WS 客户端组合式封装——多消费方共享同一连接。
 * @example
 * const { status, connId, connect, subscribe, onMessage } = useWebsocket();
 * await connect();
 * await subscribe(`admin:${userId}`);
 * onMessage(frame => { ... });
 */
export function useWebsocket(options: UseWebsocketOptions = {}): UseWebsocketReturn {
  const client = getWsClient();

  const status = ref<WsStatus>(client.getStatus());
  const connId = ref<string>(client.getConnId());
  const heartbeatMs = ref<number>(client.getHeartbeatInterval());

  // 作用域自动清理：状态镜像 + options 回调
  const disposers: Array<() => void> = [
    client.onStatus(next => {
      status.value = next;
      connId.value = next === "connected" ? client.getConnId() : "";
    }),
    client.onError(err => options.onError?.(err))
  ];
  if (options.onMessage) disposers.push(client.onMessage(options.onMessage));

  const scoped = getCurrentScope() != null;
  if (scoped) {
    onScopeDispose(() => disposers.forEach(off => off()));
  }

  /** 追加消息监听（同样随作用域自动解除）。 */
  const onMessage = (handler: (frame: WsEnvelope) => void) => {
    const off = client.onMessage(handler);
    disposers.push(off);
  };

  if (options.autoConnect) void client.connect().catch(() => undefined); // 终态拒绝已经 onError 透出

  return {
    status,
    connId,
    heartbeatMs,
    connect: () => client.connect(),
    close: () => client.close(),
    subscribe: (channel: string) => client.subscribe(channel),
    unsubscribe: (channel: string) => client.unsubscribe(channel),
    onMessage
  };
}
