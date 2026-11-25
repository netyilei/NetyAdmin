// broadcast.go：Broadcast 域远端段——Redis Pub/Sub 单频道 relay。
// 发布 = PUBLISH 信封；其他节点 relay 循环收到后本地异步投递。发布实例自环
// （From==instID）跳过：本地段已投递，避免双执行。Pub/Sub 无持久化：
// at-most-once，掉线期间消息丢失（广播类语义可接受，§5.4 可丢弃并计数）。
// 接收走 Channel()（go-redis 内建重连；通道随 PubSub.Close 关闭 → 循环退出），
// 不用 ReceiveMessage：其阻塞读无读期限，ctx 取消不中断（Close 死锁隐患）。
package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

// wireEnvelope 跨节点线格式（From 为发布实例标识，防自环；Seq 为发布方
// 单调序列——接收侧断档检测 = 断连/重启窗口丢失计数，§5.4 计数半项）。
type wireEnvelope struct {
	Msg  Message `json:"msg"`
	From string  `json:"from"`
	Seq  uint64  `json:"seq"`
}

type broadcastTransport struct {
	face    redisFace
	ps      pubsubFace
	ch      string // relay 频道（keySet.broadcastChannel）
	instID  string
	logger  *slog.Logger
	seq     *atomic.Uint64                // 发布序列（per-instance 单调；重启回 0 由接收侧重置判定）
	enqueue func(msg Message)             // 本地段入队（Bus.enqueueBroadcast，含丢弃计数）
	lost    func(source string, n uint64) // 缺口丢失计数（Bus.recordBroadcastLost）
}

// relay 发布到远端频道（seq 自增后装入信封）。
func (t *broadcastTransport) relay(ctx context.Context, msg Message) error {
	data, err := json.Marshal(wireEnvelope{Msg: msg, From: t.instID, Seq: t.seq.Add(1)})
	if err != nil {
		return fmt.Errorf("bus: marshal broadcast envelope: %w", err)
	}
	if err := t.face.Publish(ctx, t.ch, data); err != nil {
		return fmt.Errorf("bus: broadcast relay: %w", err)
	}
	return nil
}

// run relay 接收循环：ctx 取消（Close）或通道关闭（ps.Close）退出。
func (t *broadcastTransport) run(ctx context.Context) {
	msgs := t.ps.Channel()
	last := map[string]uint64{} // per-source 最后收到序列
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-msgs:
			if !ok {
				return // ps 已关闭（Bus.Close）
			}
			var env wireEnvelope
			if err := json.Unmarshal([]byte(data), &env); err != nil {
				t.logger.Error("bus broadcast malformed envelope", "err", err)
				continue
			}
			if env.From == t.instID {
				continue // 自环：本地段已投递
			}
			// 缺口检测（§5.4 计数半项；发布方重启 seq 回 0 → 重置追踪不计）
			if prev, tracked := last[env.From]; tracked && env.Seq > prev+1 {
				lost := env.Seq - prev - 1
				t.lost(env.From, lost)
				t.logger.Warn("bus broadcast gap detected", "source", env.From, "lost", lost)
			}
			last[env.From] = env.Seq
			t.enqueue(env.Msg)
		}
	}
}

// sleepCtx 可被 ctx 取消的退避；返回 false 表示 ctx 已取消。
func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
