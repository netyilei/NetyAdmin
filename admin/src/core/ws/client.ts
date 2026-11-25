// WS 客户端（admin surface—— 对接）。
// 连接时序：POST /admin/v1/ws/tickets（JWT，Bearer 头由 axios 拦截器注入）→
// GET /admin/v1/ws?ticket=…（同源路径——开发经 vite ws 代理，生产同源挂载）→
// 服务端 hello 帧（协议无客户端 hello——客户端 hello 属未知类型，服务端 4006 关连，
// ws-protocol §4.1 客户端帧白名单不含 hello）→ 恢复订阅。
// 心跳：以 hello.heartbeat_interval 为周期发 ping，一个周期内无 pong 判死重连；
// 对服务端 ping 即时回 pong（服务端 90s 空闲检测，ws-protocol §7）。
// 重连：指数退避 1s→2s→4s…封顶 30s 带 jitter（§7）；hello 成功即归零。
// 终态不重连：ticket 鉴权失败（100051xx）与主动 close()——重试无意义；
// 4007 令牌吊销在下次 ticket 签发时以鉴权码收敛为终态。
// 机制面与 Vue/Pinia 解耦（纯类——消费方见 hooks/useWebsocket.ts）。
import http from "@/api";
import { StatusCode } from "@/constants/status-codes";

import {
  WS_ADMIN_ERROR_CODE,
  WS_CHANNEL_MAX_LEN,
  WS_CLOSE_CODE,
  WS_FRAME,
  WS_HEARTBEAT_INTERVAL_MS,
  WS_PROTOCOL_VERSION,
  type WsEnvelope,
  type WsHelloPayload,
  type WsTicketData
} from "./protocol";

/** ticket 签发端点（admin JWT 组内——cmd/netyadmin/main.go 挂载）。 */
const TICKET_URL = "/admin/v1/ws/tickets";
/** upgrade 端点（公开组；ticket=一次性凭证，query 传递——禁止 JWT 入 URL）。 */
const UPGRADE_PATH = "/admin/v1/ws";
/** hello 等待超时（ms）——TCP 已建但握手帧未达即判失败重试。 */
const HELLO_TIMEOUT_MS = 10_000;
/** 重连退避基值/封顶（ws-protocol §7：1s→2s→4s…上限 30s 带 jitter）。 */
const BACKOFF_BASE_MS = 1_000;
const BACKOFF_MAX_MS = 30_000;
/** 退避 jitter 上界（+0~30%）。 */
const BACKOFF_JITTER_RATIO = 0.3;

/** 连接状态机：idle 未启动 / connecting 首连 / connected 在线 / reconnecting 退避中 / closed 主动或终态关闭。 */
export type WsStatus = "idle" | "connecting" | "connected" | "reconnecting" | "closed";

/** 错误信息（kind 区分来源：frame=服务端 error 帧 / auth=ticket 鉴权终态 / closed=连接层关闭）。 */
export interface WsErrorInfo {
  kind: "frame" | "auth" | "closed";
  code?: string;
  messageKey?: string;
  channel?: string;
  closeCode?: number;
  reason?: string;
}

export type WsMessageHandler = (frame: WsEnvelope) => void;
export type WsErrorHandler = (err: WsErrorInfo) => void;
export type WsStatusHandler = (status: WsStatus) => void;

/** 订阅/退订回执等待者（kind 决定 error 帧语义：sub 拒绝 / unsub 收敛成功）。 */
interface ChannelWaiter {
  kind: "sub" | "unsub";
  resolve: (seq?: number) => void;
  reject: (err: WsErrorInfo) => void;
}

/** ticket 失败的信封形状（axios 拦截器拒绝值为响应信封）。 */
interface EnvelopeLike {
  code?: unknown;
  message_key?: unknown;
}

/** ticket 鉴权类失败码——重连无意义（信封 5 位码，@/constants/status-codes 单一源）。 */
const TICKET_AUTH_CODES: ReadonlySet<string> = new Set([
  StatusCode.CodeUnauthorized,
  StatusCode.CodeTokenMissing,
  StatusCode.CodeTokenInvalid,
  StatusCode.CodeTokenExpired,
  StatusCode.CodeTokenRevoked,
  StatusCode.CodeAccountDisabled,
  StatusCode.CodePermissionDenied
]);

function envelopeField(err: unknown, key: keyof EnvelopeLike): string | undefined {
  if (err && typeof err === "object" && key in err) {
    const v = (err as EnvelopeLike)[key];
    if (typeof v === "string") return v;
  }
  return undefined;
}

/** 帧唯一 ID（secure context 无 randomUUID 时降级时间+随机段——仅幂等记账用途）。 */
function newFrameId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") return crypto.randomUUID();
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
}

export class WsClient {
  private ws: WebSocket | null = null;
  private status: WsStatus = "idle";
  private connectPromise: Promise<void> | null = null;
  private intentionalClose = false;

  private attempt = 0;
  private everConnected = false; // 是否握手成功过——状态语义区分首连/重连
  private heartbeatMs = WS_HEARTBEAT_INTERVAL_MS;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private awaitingPong = false;

  private connId = "";
  private protocolVersion = WS_PROTOCOL_VERSION;

  /** 期望订阅集（跨重连恢复——hello 后统一重发 subscribe）。 */
  private wanted = new Set<string>();
  /** 订阅/退订回执等待者（按频道分桶；断线期间挂起，回执或重连恢复后收敛）。 */
  private waiters = new Map<string, ChannelWaiter[]>();

  private messageHandlers = new Set<WsMessageHandler>();
  private errorHandlers = new Set<WsErrorHandler>();
  private statusHandlers = new Set<WsStatusHandler>();

  // —— 连接生命周期 ——

  /**
   * 建立连接（幂等：进行中返回同一 Promise；已在线直接 resolve）。
   * hello 到达才 resolve；鉴权终态 reject；瞬态失败转入后台退避重试（Promise 保持 pending）。
   */
  connect(): Promise<void> {
    if (this.connectPromise) return this.connectPromise;
    if (this.status === "connected") return Promise.resolve();
    this.intentionalClose = false;
    this.connectPromise = this.run();
    return this.connectPromise;
  }

  /**
   * 主动关闭（WebSocket close 握手 1000——协议客户端帧白名单无应用层 close 帧，
   * 发送属未知类型将触发服务端 4006 关连）；不再重连。
   */
  close(): void {
    this.intentionalClose = true;
    this.stopHeartbeat();
    this.cancelSocket();
    this.settleAllWaiters({ kind: "closed", reason: "client closed" });
    this.attempt = 0;
    this.everConnected = false;
    this.connId = "";
    this.setStatus("closed");
  }

  // —— 订阅 API ——

  /**
   * 订阅频道（subscribed 回执 resolve，携带频道 seq；断线期间调用自动在重连后收敛）。
   * 频道 ACL 服务端两级校验（admin 权限码 ws:channel:<name>、私有频道仅本人）；
   * 被拒频道移出期望集（防重连恢复循环），错误经 reject + onError 双通道透出。
   */
  subscribe(channel: string): Promise<number | undefined> {
    if (!channel || channel.length > WS_CHANNEL_MAX_LEN) {
      const info: WsErrorInfo = {
        kind: "frame",
        code: WS_ADMIN_ERROR_CODE.INVALID_CHANNEL,
        channel,
        messageKey: "admin.ws_invalid_channel"
      };
      return Promise.reject(info);
    }
    this.wanted.add(channel);
    return new Promise((resolve, reject) => {
      this.pushWaiter(channel, { kind: "sub", resolve, reject });
      if (this.isOpen()) this.sendFrame(WS_FRAME.SUBSCRIBE, { channel });
    });
  }

  /** 退订频道（幂等收敛——目标态"未订阅"即成功；服务端 22006 亦 resolve 并经 onError 透出）。 */
  unsubscribe(channel: string): Promise<void> {
    this.wanted.delete(channel);
    const pending = this.takeWaiters(channel);
    if (!this.isOpen()) {
      // 未在线：无回执可达——期望集已删即收敛
      pending.forEach(w => this.settleWaiter(w));
      return Promise.resolve();
    }
    return new Promise(resolve => {
      pending.forEach(w => this.settleWaiter(w));
      this.pushWaiter(channel, { kind: "unsub", resolve: () => resolve(), reject: () => resolve() });
      this.sendFrame(WS_FRAME.UNSUBSCRIBE, { channel });
    });
  }

  // —— 监听注册（返回解除函数） ——

  /** 业务消息（message 帧）监听——入参为完整信封（channel/payload/seq/id 由消费方取用）。 */
  onMessage(handler: WsMessageHandler): () => void {
    this.messageHandlers.add(handler);
    return () => this.messageHandlers.delete(handler);
  }

  /** 错误监听（error 帧/ticket 鉴权终态/连接层关闭）。 */
  onError(handler: WsErrorHandler): () => void {
    this.errorHandlers.add(handler);
    return () => this.errorHandlers.delete(handler);
  }

  /** 连接状态变化监听。 */
  onStatus(handler: WsStatusHandler): () => void {
    this.statusHandlers.add(handler);
    return () => this.statusHandlers.delete(handler);
  }

  // —— 状态读数 ——

  getStatus(): WsStatus {
    return this.status;
  }
  getConnId(): string {
    return this.connId;
  }
  getHeartbeatInterval(): number {
    return this.heartbeatMs;
  }
  isConnected(): boolean {
    return this.status === "connected";
  }

  // —— 内部：重试主循环 ——

  private async run(): Promise<void> {
    try {
      for (;;) {
        if (this.intentionalClose) return;
        this.setStatus(this.attempt === 0 && !this.everConnected ? "connecting" : "reconnecting");
        let ticket: string;
        try {
          ticket = await this.requestTicket();
        } catch (err) {
          if (this.intentionalClose) return;
          const code = envelopeField(err, "code");
          if (code && TICKET_AUTH_CODES.has(code)) {
            // 鉴权终态：token 失效/权限缺失——重试无意义，停止并双通道透出
            const info: WsErrorInfo = { kind: "auth", code, messageKey: envelopeField(err, "message_key") };
            this.emitError(info);
            this.setStatus("closed");
            throw info;
          }
          await this.backoffSleep(); // 网络/服务不可用——瞬态重试
          continue;
        }
        try {
          await this.openSocket(ticket);
          return; // hello 到达——连接就绪，循环退出
        } catch {
          if (this.intentionalClose) return;
          await this.backoffSleep();
        }
      }
    } finally {
      // run 生命周期结束（成功/终态）即释放占用——重连触发路径可重新 connect()
      this.connectPromise = null;
    }
  }

  private async requestTicket(): Promise<string> {
    // channels 传空：ticket 不绑定频道集，订阅决策留给服务端两级 ACL（客户端订阅动态可达）
    const res = await http.post<WsTicketData>(TICKET_URL, { channels: [] }, { loading: false, cancel: false });
    const ticket = res.data?.ticket;
    if (!ticket) throw new Error("ws: empty ticket");
    return ticket;
  }

  /** 建立 upgrade 连接并等待 hello（resolve=握手完成；reject=升级/握手失败或中途断开）。 */
  private openSocket(ticket: string): Promise<void> {
    return new Promise((resolve, reject) => {
      const proto = location.protocol === "https:" ? "wss:" : "ws:";
      const url = `${proto}//${location.host}${UPGRADE_PATH}?ticket=${encodeURIComponent(ticket)}`;
      let settled = false;
      let helloTimer: ReturnType<typeof setTimeout> | null = null;
      const socket = new WebSocket(url);
      this.ws = socket;

      const settle = (ok: boolean, err?: WsErrorInfo) => {
        if (settled) return;
        settled = true;
        if (helloTimer) clearTimeout(helloTimer);
        if (ok) resolve();
        else reject(err);
      };

      socket.onopen = () => {
        helloTimer = setTimeout(() => {
          settle(false, { kind: "closed", reason: "hello timeout" });
          this.cancelSocket();
        }, HELLO_TIMEOUT_MS);
      };
      socket.onmessage = ev => {
        const frame = this.parseFrame(ev.data);
        if (!frame) return; // 非 JSON 帧——服务端产出恒为信封，忽略
        if (frame.type === WS_FRAME.HELLO) {
          const ok = this.onHello(frame);
          settle(ok, ok ? undefined : { kind: "closed", reason: "bad hello payload" });
          if (!ok) this.cancelSocket();
          return;
        }
        if (!settled) return; // hello 前不消费业务帧（协议时序保证 hello 先行）
        this.handleFrame(frame);
      };
      socket.onclose = ev => {
        const wasConnected = settled;
        settle(false, { kind: "closed", closeCode: ev.code, reason: ev.reason });
        this.handleDisconnect(wasConnected);
      };
      socket.onerror = () => {
        /* close 事件随后必达——错误经 onclose/退避路径收敛 */
      };
    });
  }

  private onHello(frame: WsEnvelope): boolean {
    const payload = frame.payload as WsHelloPayload | undefined;
    if (!payload || typeof payload.conn_id !== "string") return false;
    this.connId = payload.conn_id;
    if (typeof payload.protocol_version === "number") this.protocolVersion = payload.protocol_version;
    this.heartbeatMs =
      typeof payload.heartbeat_interval === "number" && payload.heartbeat_interval > 0
        ? payload.heartbeat_interval
        : WS_HEARTBEAT_INTERVAL_MS;
    this.attempt = 0; // 握手成功——退避归零
    this.everConnected = true;
    this.setStatus("connected");
    this.startHeartbeat();
    this.restoreSubscriptions();
    return true;
  }

  /** hello 后恢复期望订阅（重连路径——无挂起 waiter 的频道静默恢复，有 waiter 待回执收敛）。 */
  private restoreSubscriptions(): void {
    for (const channel of this.wanted) {
      if (!this.waiters.has(channel)) {
        this.pushWaiter(channel, {
          kind: "sub",
          resolve: () => undefined, // 静默恢复——回执无需通知（订阅态由 wanted 集承载）
          reject: () => this.wanted.delete(channel)
        });
      }
    }
    for (const channel of this.wanted) this.sendFrame(WS_FRAME.SUBSCRIBE, { channel });
  }

  /** 连接断开清理（幂等；在线中断开时触发重连——run 循环已退出的场景）。 */
  private handleDisconnect(wasConnected: boolean): void {
    this.stopHeartbeat();
    this.awaitingPong = false;
    this.ws = null;
    this.connId = "";
    if (!wasConnected) return; // 握手失败——run() 循环自行退避重试
    if (this.intentionalClose) {
      this.setStatus("closed");
      return;
    }
    this.setStatus("reconnecting");
    // 终态拒绝已双通道透出（onError），此处防未处理 rejection
    void this.connect().catch(() => undefined);
  }

  // —— 内部：帧处理 ——

  private handleFrame(frame: WsEnvelope): void {
    switch (frame.type) {
      case WS_FRAME.PING:
        // 服务端心跳——即时回 pong（服务端空闲检测按 pong/ping 记活）
        this.sendFrame(WS_FRAME.PONG, { ts: frame.ts });
        return;
      case WS_FRAME.PONG:
        this.awaitingPong = false;
        return;
      case WS_FRAME.SUBSCRIBED: {
        if (frame.channel) {
          this.takeWaiters(frame.channel).forEach(w => this.settleWaiter(w, { seq: frame.seq }));
        }
        return;
      }
      case WS_FRAME.UNSUBSCRIBED: {
        if (frame.channel) this.takeWaiters(frame.channel).forEach(w => this.settleWaiter(w));
        return;
      }
      case WS_FRAME.MESSAGE:
        this.messageHandlers.forEach(h => h(frame));
        return;
      case WS_FRAME.ERROR: {
        const info: WsErrorInfo = {
          kind: "frame",
          code: frame.code,
          messageKey: frame.message_key,
          channel: frame.channel
        };
        if (frame.channel) {
          const taken = this.takeWaiters(frame.channel);
          const subDenied = taken.some(w => w.kind === "sub");
          taken.forEach(w => this.settleWaiter(w, { rejectWith: info }));
          if (subDenied) this.wanted.delete(frame.channel); // 订阅被拒——移出期望集防重连恢复循环
        }
        this.emitError(info);
        return;
      }
      case WS_FRAME.CLOSE:
        // 服务端关闭通知——close_code 随 WebSocket Close 帧到达，经 onclose/退避收敛
        this.emitError({ kind: "closed", closeCode: frame.close_code, reason: frame.reason });
        return;
      default:
        return; // 未知类型忽略（向前兼容——ws-protocol §4）
    }
  }

  // —— 内部：心跳 ——

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.awaitingPong = false;
    this.heartbeatTimer = setInterval(() => {
      if (!this.isOpen()) return;
      if (this.awaitingPong) {
        // 一个周期未获 pong——链路判死，主动断开触发重连（服务端 90s 空闲关闭前自愈）
        this.ws?.close(WS_CLOSE_CODE.NORMAL, "pong timeout");
        return;
      }
      this.awaitingPong = true;
      this.sendFrame(WS_FRAME.PING, { ts: Date.now() });
    }, this.heartbeatMs);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  // —— 内部：帧收发 ——

  private isOpen(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  private sendFrame(type: string, opts?: { channel?: string; ts?: number }): void {
    if (!this.isOpen() || !this.ws) return;
    const frame: WsEnvelope = {
      id: newFrameId(),
      type,
      ts: opts?.ts ?? Date.now(),
      schema_version: this.protocolVersion
    };
    if (opts?.channel !== undefined) frame.channel = opts.channel;
    this.ws.send(JSON.stringify(frame));
  }

  private parseFrame(raw: unknown): WsEnvelope | null {
    try {
      const frame = JSON.parse(String(raw)) as WsEnvelope;
      if (!frame || typeof frame.type !== "string") return null;
      return frame;
    } catch {
      return null;
    }
  }

  private cancelSocket(): void {
    if (!this.ws) return;
    try {
      this.ws.close(WS_CLOSE_CODE.NORMAL, "client closed");
    } catch {
      /* 已关连——幂等 */
    }
    this.ws = null;
  }

  // —— 内部：退避 ——

  private backoffSleep(): Promise<void> {
    const base = Math.min(BACKOFF_MAX_MS, BACKOFF_BASE_MS * 2 ** this.attempt);
    const delay = Math.round(base * (1 + Math.random() * BACKOFF_JITTER_RATIO));
    this.attempt++;
    return new Promise(resolve => setTimeout(resolve, delay));
  }

  // —— 内部：waiter 收敛 ——

  private pushWaiter(channel: string, waiter: ChannelWaiter): void {
    const list = this.waiters.get(channel) ?? [];
    list.push(waiter);
    this.waiters.set(channel, list);
  }

  private takeWaiters(channel: string): ChannelWaiter[] {
    const list = this.waiters.get(channel) ?? [];
    this.waiters.delete(channel);
    return list;
  }

  /** waiter 收敛：rejectWith 仅作用于 sub（unsub 目标态"未订阅"恒成功）。 */
  private settleWaiter(waiter: ChannelWaiter, outcome?: { rejectWith?: WsErrorInfo; seq?: number }): void {
    if (outcome?.rejectWith && waiter.kind === "sub") waiter.reject(outcome.rejectWith);
    else waiter.resolve(outcome?.seq);
  }

  private settleAllWaiters(err: WsErrorInfo): void {
    for (const [channel, list] of this.waiters) {
      list.forEach(w => this.settleWaiter(w, { rejectWith: err }));
      this.waiters.delete(channel);
    }
  }

  // —— 内部：状态与错误透出 ——

  private setStatus(next: WsStatus): void {
    if (this.status === next) return;
    this.status = next;
    this.statusHandlers.forEach(h => h(next));
  }

  private emitError(err: WsErrorInfo): void {
    this.errorHandlers.forEach(h => h(err));
  }
}

/** 共享单例（admin 推送面单一连接——多页共用，避免 4005 同身份上限自我挤占）。 */
let singleton: WsClient | null = null;

export function getWsClient(): WsClient {
  if (!singleton) singleton = new WsClient();
  return singleton;
}
