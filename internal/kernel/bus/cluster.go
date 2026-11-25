// cluster.go：Cluster 域——Redis Streams 消费组（per-topic 流 + 全集群同名组，
//）。XADD 后全集群恰好一个消费者执行：handler 成功 XAck；
// 失败留 PEL 由回收循环重投（at-least-once）；累计投递达到 MaxAttempts
// XAck+丢弃计数（重试上限红线；DLQ/重放随）。流按 MAXLEN 近似裁剪。
// 消费轮询为非阻塞 XREADGROUP（clusterPoll 间隔）：不依赖阻塞命令的读超时
// 放宽，新 topic 拾取延迟 ≤ 一个轮询周期。
package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

type clusterTransport struct {
	face         redisFace
	keys         keySet
	logger       *slog.Logger
	nodeID       string
	maxLen       int64
	maxAttempts  int64
	minIdle      time.Duration // 回收最小空闲 = HandlerTimeout + margin（防在途双处理）
	reclaimEvery time.Duration
	topics       func() []string                                                                         // 当前有订阅的 topic 快照（Bus.clusterTopics）
	enqueue      func(msg Message, ack *pendingAck)                                                      // 入本地队列（含丢弃计数）
	ack          func(stream, entryID string) error                                                      // 条目确认（Bus.ackEntry，毒消息/回收共用）
	storeDead    func(ctx context.Context, stream, entryID, cause string, attempts int, raw string) bool // 死信入库（Bus.storeDeadEntry；true=已入库）
	discarded    func(topic, entryID string)                                                             // 重试上限丢弃（指标+日志）
}

// publish XADD 到 topic 流（裁剪 MAXLEN）。
func (t *clusterTransport) publish(ctx context.Context, msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("bus: marshal cluster envelope: %w", err)
	}
	if _, err := t.face.XAdd(ctx, t.keys.streamKey(msg.Topic), t.maxLen, map[string]any{streamField: string(data)}); err != nil {
		return fmt.Errorf("bus: cluster publish: %w", err)
	}
	return nil
}

// run 消费循环：每轮取订阅 topic 快照 → 非阻塞 XREADGROUP 竞争取新消息 → 入队。
// 组由 Subscribe 时建（MKSTREAM），仅本节点订阅的 topic 会被读取。
func (t *clusterTransport) run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if topics := t.topics(); len(topics) > 0 {
			streams := make([]string, len(topics))
			for i, tp := range topics {
				streams[i] = t.keys.streamKey(tp) // topic → 流键（XREADGROUP 按 key 读）
			}
			entries, err := t.face.XReadGroup(ctx, t.keys.consumerGroup, t.nodeID, streams)
			if err != nil && ctx.Err() == nil {
				t.logger.Warn("bus cluster read failed", "err", err)
			}
			for _, e := range entries {
				t.handleEntry(ctx, e)
			}
		}
		if !sleepCtx(ctx, clusterPoll) {
			return
		}
	}
}

// reclaim 回收循环：扫描各 topic 的 PEL——达到重试上限的 XAck 丢弃；
// 空闲超过 minIdle 的认领（XClaim）后按新消息同路径重投。
func (t *clusterTransport) reclaim(ctx context.Context) {
	for {
		if !sleepCtx(ctx, t.reclaimEvery) {
			return
		}
		for _, topic := range t.topics() {
			t.reclaimTopic(ctx, topic)
		}
	}
}

func (t *clusterTransport) reclaimTopic(ctx context.Context, topic string) {
	stream := t.keys.streamKey(topic)
	pending, err := t.face.XPending(ctx, stream, t.keys.consumerGroup, t.minIdle, pendingScanLimit)
	if err != nil {
		if ctx.Err() == nil {
			t.logger.Warn("bus cluster pending scan failed", "topic", topic, "err", err)
		}
		return
	}
	var claimIDs []string
	for _, p := range pending {
		// Attempts = 已完成投递次数（首投=1，XCLAIM 续投递增）；达到上限不再续投
		if p.Attempts >= t.maxAttempts {
			// 重试上限红线：必弃。先尽力入 DLQ 留存（无 DB/写失败则仅计数+告警，
			// 行为——不可无限重投毒消息）
			if !t.storeStreamDead(ctx, stream, p.ID, "max attempts exceeded", int(p.Attempts)) {
				t.logger.Warn("bus cluster message discarded without dlq (db unavailable)", "topic", topic, "id", p.ID)
			}
			if err := t.face.XAck(ctx, stream, t.keys.consumerGroup, p.ID); err != nil {
				t.logger.Warn("bus cluster discard ack failed", "topic", topic, "id", p.ID, "err", err)
				continue
			}
			t.discarded(topic, p.ID)
			continue
		}
		claimIDs = append(claimIDs, p.ID)
	}
	if len(claimIDs) == 0 {
		return
	}
	entries, err := t.face.XClaim(ctx, stream, t.keys.consumerGroup, t.nodeID, t.minIdle, claimIDs)
	if err != nil {
		if ctx.Err() == nil {
			t.logger.Warn("bus cluster claim failed", "topic", topic, "err", err)
		}
		return
	}
	for _, e := range entries {
		t.handleEntry(ctx, e)
	}
}

// storeStreamDead 取条目原文并交 Bus 入 DLQ（达到重试上限/毒消息）；成功返回
// true。XRange 取不到原文 = 条目已被 MAXLEN 近似裁剪：入**合成死信行**（元
// 数据留存、内容不可恢复标记，审计 A12-2）。入库失败（无 DB/写失败）返回
// false，调用方按计数路径弃置。
func (t *clusterTransport) storeStreamDead(ctx context.Context, stream, entryID, cause string, attempts int) bool {
	if t.storeDead == nil {
		return false
	}
	entries, err := t.face.XRange(ctx, stream, entryID, entryID, 1)
	if err != nil {
		if ctx.Err() == nil {
			t.logger.Warn("bus dlq fetch failed", "stream", stream, "id", entryID, "err", err)
		}
		return false
	}
	if len(entries) == 0 {
		// 被裁剪：原文不可恢复，合成行保留元数据（topic/entryID/原因），
		// kind 空=不可重放标记（与毒消息同径）
		return t.storeDead(ctx, stream, entryID,
			cause+" (envelope trimmed by MAXLEN before discard; content unrecoverable)", attempts, "")
	}
	return t.storeDead(ctx, stream, entryID, cause, attempts, entries[0].Fields[streamField])
}

// handleEntry 解码条目并入队（ack 联动）。信封损坏为毒消息：原文交 Bus 入 DLQ
// 留存后 XAck 移出 PEL（不可重试内容，重投无意义；DLQ 不可用时仅计数+告警）。
func (t *clusterTransport) handleEntry(ctx context.Context, e streamEntry) {
	data := e.Fields[streamField]
	var msg Message
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		if t.storeDead != nil && t.storeDead(ctx, e.Stream, e.ID, "unmarshal: "+err.Error(), 1, data) {
			t.logger.Warn("bus cluster poison message moved to dlq", "stream", e.Stream, "id", e.ID)
		} else {
			t.logger.Warn("bus poison message discarded without dlq (db unavailable)", "stream", e.Stream, "id", e.ID)
		}
		if ackErr := t.ack(e.Stream, e.ID); ackErr != nil {
			t.logger.Warn("bus poison ack failed", "stream", e.Stream, "id", e.ID, "err", ackErr)
			return // 留 PEL：下轮按重试上限路径处理
		}
		t.discarded(t.keys.streamTopic(e.Stream), e.ID)
		return
	}
	t.enqueue(msg, &pendingAck{stream: e.Stream, entryID: e.ID})
}
