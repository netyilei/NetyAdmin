// outbox.go：Outbox 事务性发布 + 中继 + DLQ。
//
// Outbox：PublishToOutbox 要求 ctx 处于 database.WithTx 事务内，信封与业务写
// 同事务 INSERT（回滚即丢弃）；Relay 循环以 FOR UPDATE SKIP LOCKED 批量 claim
// （ORDER BY id 保序；next_attempt_at 未到期的退避行跳过）后逐行以 Cluster 域
// 发布，成功标记 published，保留窗口后清理（"清理已完成"）。
//
// 多 relay 语义：
//   - 分区 claim：SKIP LOCKED 行锁粒度——多实例并行 claim 无全局锁/全表串行点；
//   - 行级隔离：单行失败只推进自身 attempts/退避（outboxBackoff），达
//     MaxAttempts 入 bus_dlq 并置 dead——毒行不阻塞后续行（原批级回滚=一行
//     坏数据永久卡住整批）。同节点内仍按 id 序发布；多节点 relay 可能交叠，
//     per-aggregate 全序限制登记。
//
// DLQ：Cluster 域消息投递达到 MaxAttempts 或毒消息（信封损坏）时入库留存并
// XAck 移出 PEL；Outbox 毒行同表留存（同一重放面）。ReplayDLQ 重建信封重发布。
// DLQ 依赖 DB——无 DB 时退回的丢弃计数路径（登记）。
package bus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/netyilei/netyadmin/internal/kernel/database"
)

// Outbox/DLQ 具名常量（G1-15：无具名消费者，不设 Options 项）。
const (
	outboxBatch     = 100                    // 单轮 claim 条数
	outboxPoll      = 500 * time.Millisecond // relay 轮询间隔
	outboxRetention = time.Hour              // published 行保留窗口
	inboxKeyMaxLen  = 512                    // Inbox 业务键长度上限

	// outboxRetryBase/outboxRetryMax 是**行级**失败退避（attempts 1→2s、2→4s…
	// 上限 30s）：单行失败只推迟自身，不整批回滚——毒行不阻塞后续行
	//
	outboxRetryBase = 2 * time.Second
	outboxRetryMax  = 30 * time.Second
)

// 错误哨兵。
var (
	ErrNoTx       = errors.New("bus: outbox publish requires transaction (database.WithTx)")
	ErrNoDB       = errors.New("bus: database not attached")
	ErrNotFound   = errors.New("bus: not found")
	ErrInvalidKey = errors.New("bus: invalid inbox key")
	ErrInvalidTTL = errors.New("bus: invalid inbox ttl")
	ErrPoisonRow  = errors.New("bus: poison dlq row cannot be replayed")
)

// outbox 行状态（Go 侧具名；SQL 文本经参数引用，禁裸数字——G1-08）。
const (
	outboxPending   = 0 // 待发布（含退避窗口内的重试行）
	outboxPublished = 1 // 已发布（保留窗口后清理）
	outboxDead      = 2 // 毒行出列（达 MaxAttempts——payload 已进 bus_dlq 留存）
)

// AttachDB 延迟注入数据库池（Outbox/DLQ 依赖；须在 Start 前调用，幂等）。
// 已有池时仅接受同一实例（重复注入不同池 fail-closed，防装配错配）。
// cmd 装配顺序：db-connect hook（建池 + 本方法）→ bus-start hook（Start 起 relay）。
func (b *Bus) AttachDB(pool *database.Pool) error {
	if b.closed.Load() {
		return ErrClosed
	}
	if b.started.Load() {
		return ErrAlreadyStart
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.db == nil {
		b.db = pool
		return nil
	}
	if b.db != pool {
		return fmt.Errorf("bus: attach db mismatch (different pool already attached)")
	}
	return nil
}

// WithAggregate 指定聚合键（Outbox 行记录；同聚合事件按写入序发布，
// per-aggregate 分区 claim 随排序敏感消费方接入，登记）。
func WithAggregate(id string) PublishOption {
	return func(c *publishConfig) { c.aggregateID = id }
}

// PublishToOutbox 将消息写入 outbox 表，与 ctx 事务同提交（§5.4 事务内发布）。
// ctx 必须在 database.WithTx 内（业务同事务）；事务回滚即丢弃。
// Relay 异步以 Cluster 域发布（outbox 语义 = at-least-once，域固定不可覆盖）——
// 调用方不承担发布即时性。
func (b *Bus) PublishToOutbox(ctx context.Context, msg Message, opts ...PublishOption) error {
	if b.closed.Load() {
		return ErrClosed
	}
	if b.db == nil {
		return ErrNoDB
	}
	if database.TxFrom(ctx) == nil {
		return ErrNoTx
	}
	cfg := publishConfig{domain: DomainCluster, kind: KindDomain, schemaVer: schemaVerDefault}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.domain != DomainCluster {
		// outbox 恒 cluster（relay at-least-once）；语义冲突 fail-closed 拒绝
		return fmt.Errorf("%w: outbox domain is fixed to cluster", ErrInvalidOption)
	}
	if err := cfg.validate(); err != nil {
		return err
	}
	prepared, err := b.prepare(ctx, msg, cfg)
	if err != nil {
		return err
	}
	headers, err := json.Marshal(prepared.Headers)
	if err != nil {
		return fmt.Errorf("bus: marshal outbox headers: %w", err)
	}
	if _, err := database.TxFrom(ctx).Exec(ctx, `INSERT INTO bus_outbox
		(message_id, topic, kind, schema_ver, aggregate_id, payload, headers, occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		prepared.ID, prepared.Topic, string(prepared.Kind), prepared.SchemaVer,
		cfg.aggregateID, string(prepared.Payload), string(headers), prepared.OccurredAt); err != nil {
		return fmt.Errorf("bus: outbox insert: %w", err)
	}
	return nil
}

// PublishAfterCommit 类型化 Outbox 发布（§5.2 语义名）：ev 序列化为 Payload。
func PublishAfterCommit[E Event](b *Bus, ctx context.Context, ev E, opts ...PublishOption) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("bus: marshal event %T: %w", ev, err)
	}
	return b.PublishToOutbox(ctx, Message{Topic: ev.Topic(), Payload: json.RawMessage(data)}, opts...)
}

// Publish 类型化即时发布（§5.2 泛型形态；转发 PublishEvent，即时域经 opts 声明）。
func Publish[E Event](b *Bus, ctx context.Context, ev E, opts ...PublishOption) error {
	return PublishEvent(b, ctx, ev, opts...)
}

// runRelay 中继循环：批量 claim→发布→标记；published 过保留窗口清理；
// DLQ 深度校准。失败退避续跑，ctx 取消退出。
func (b *Bus) runRelay(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := b.relayBatch(ctx); err != nil {
			if ctx.Err() == nil {
				b.logger.Warn("bus outbox relay failed", "err", err)
			}
		}
		if err := b.cleanupOutbox(ctx); err != nil && ctx.Err() == nil {
			b.logger.Warn("bus outbox cleanup failed", "err", err)
		}
		b.syncDLQDepth(ctx)
		b.syncOutboxPending(ctx)
		if !sleepCtx(ctx, outboxPoll) {
			return
		}
	}
}

// relayBatch 单事务批量中继（FOR UPDATE SKIP LOCKED）：
// claim pending（按 id 序；next_attempt_at 未到期的行跳过）→ 逐行发布/失败记账
// → commit。**行级隔离**：单行失败只推进自身 attempts+退避（达 MaxAttempts
// 转 DLQ 并置 dead），不整批回滚——毒行不阻塞后续行；多 relay 实例靠 SKIP
// LOCKED 天然并行 claim（行锁粒度，无全局锁/全表串行点）。
func (b *Bus) relayBatch(ctx context.Context) error {
	return database.WithinTx(ctx, b.db, func(ctx context.Context) error {
		tx := database.TxFrom(ctx)
		rows, err := tx.Query(ctx, `SELECT id, message_id, topic, kind, schema_ver,
			payload, headers, occurred_at, attempts
			FROM bus_outbox
			WHERE status = $2 AND (next_attempt_at IS NULL OR next_attempt_at <= now())
			ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED`,
			outboxBatch, outboxPending)
		if err != nil {
			return fmt.Errorf("bus: outbox claim: %w", err)
		}
		type claim struct {
			id        int64
			attempts  int
			msg       Message
			decodeErr error
		}
		var batch []claim
		for rows.Next() {
			var c claim
			var payload, headers string
			if err := rows.Scan(&c.id, &c.msg.ID, &c.msg.Topic, &c.msg.Kind, &c.msg.SchemaVer,
				&payload, &headers, &c.msg.OccurredAt, &c.attempts); err != nil {
				rows.Close()
				return fmt.Errorf("bus: outbox scan: %w", err)
			}
			c.msg.Payload = json.RawMessage(payload)
			if uerr := json.Unmarshal([]byte(headers), &c.msg.Headers); uerr != nil {
				// 信封头损坏=该行毒数据：不中断整批（行级失败记账延后到 rows 关闭后
				// ——pgx 单连接不允许 Rows 未关闭时再执行查询）
				c.msg.Headers = nil
				c.decodeErr = fmt.Errorf("headers decode: %w", uerr)
			}
			batch = append(batch, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("bus: outbox rows: %w", err)
		}
		for _, c := range batch {
			if c.decodeErr != nil {
				b.failOutboxRow(ctx, tx, c.id, c.attempts, c.msg, c.decodeErr)
				continue
			}
			if err := b.Publish(ctx, c.msg, WithDomain(DomainCluster)); err != nil {
				b.failOutboxRow(ctx, tx, c.id, c.attempts, c.msg, err)
				continue
			}
			if _, err := tx.Exec(ctx, `UPDATE bus_outbox
				SET status = $2, last_error = '', next_attempt_at = NULL,
				    updated_at = now(), published_at = now() WHERE id = $1`,
				c.id, outboxPublished); err != nil {
				return fmt.Errorf("bus: outbox mark: %w", err)
			}
		}
		return nil
	})
}

// failOutboxRow 行级失败记账（relayBatch 内调用，同一事务）：
//   - attempts+1 后未达 MaxAttempts → 保持 pending，写 last_error 与行级退避
//     窗口（next_attempt_at）——其他行不受影响，下轮继续；
//   - 达到 MaxAttempts → payload 入 bus_dlq（与 Cluster 域同表同重放面），
//     行置 dead 出列——毒行不再占用 claim 面。
//
// 记账失败返回错误由外层事务回滚（DB 不可用属基础设施故障，非行级毒数据）。
func (b *Bus) failOutboxRow(ctx context.Context, tx pgx.Tx, id int64, attempts int,
	msg Message, cause error) {

	next := attempts + 1
	causeMsg := truncateError(cause.Error())
	if next >= b.cfg.MaxAttempts {
		headers, _ := json.Marshal(msg.Headers)
		if _, err := tx.Exec(ctx, `INSERT INTO bus_dlq
			(topic, message_id, kind, schema_ver, payload, headers, occurred_at, error, attempts)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			msg.Topic, msg.ID, string(msg.Kind), msg.SchemaVer,
			string(msg.Payload), string(headers), msg.OccurredAt, causeMsg, next); err != nil {
			b.logger.Warn("bus outbox dlq store failed (row stays pending, retried next round)",
				"id", id, "topic", msg.Topic, "err", err)
			return
		}
		if _, err := tx.Exec(ctx, `UPDATE bus_outbox
			SET status = $2, attempts = $3, last_error = $4, updated_at = now() WHERE id = $1`,
			id, outboxDead, next, causeMsg); err != nil {
			b.logger.Warn("bus outbox dead mark failed (row stays pending, dlq row may duplicate on retry)",
				"id", id, "err", err)
			return
		}
		b.metrics.BusDLQDepth.Inc()
		b.logger.Warn("bus outbox poison row moved to dlq", "id", id, "topic", msg.Topic,
			"attempts", next, "err", causeMsg)
		return
	}
	nextAt := b.clock.Now().Add(b.outboxBackoff(next))
	if _, err := tx.Exec(ctx, `UPDATE bus_outbox
		SET attempts = $2, last_error = $3, next_attempt_at = $4, updated_at = now() WHERE id = $1`,
		id, next, causeMsg, nextAt); err != nil {
		b.logger.Warn("bus outbox retry bookkeeping failed", "id", id, "err", err)
		return
	}
	b.logger.Warn("bus outbox row failed (row-level backoff)", "id", id, "topic", msg.Topic,
		"attempts", next, "next_attempt_in", b.outboxBackoff(next).String(), "err", causeMsg)
}

// outboxBackoff 行级指数退避（第 n 次失败 → base*2^(n-1)，上限 outboxRetryMax）。
func (b *Bus) outboxBackoff(attempts int) time.Duration {
	base := outboxRetryBase
	if b.cfg.outboxRetryBase > 0 { // 测试注入面
		base = b.cfg.outboxRetryBase
	}
	d := base
	for i := 1; i < attempts; i++ {
		d *= 2
		if d >= outboxRetryMax {
			return outboxRetryMax
		}
	}
	return d
}

// truncateError 限制失败原因长度（DB TEXT 无界；防模板化超长错误拖大行）。
func truncateError(s string) string {
	const maxLen = 1024
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// cleanupOutbox 清理已发布且超保留窗口的行（清理已完成）；dead 行
// （毒行已进 DLQ）同窗口清理——DLQ 表才是留存/重放面。
// 截止时间经注入 Clock 计算（可测），不取 DB now()。
func (b *Bus) cleanupOutbox(ctx context.Context) error {
	cutoff := b.clock.Now().Add(-outboxRetention)
	if _, err := b.db.Exec(ctx,
		`DELETE FROM bus_outbox WHERE status = $1 AND published_at < $2`,
		outboxPublished, cutoff); err != nil {
		return fmt.Errorf("bus: outbox cleanup: %w", err)
	}
	if _, err := b.db.Exec(ctx,
		`DELETE FROM bus_outbox WHERE status = $1 AND updated_at < $2`,
		outboxDead, cutoff); err != nil {
		return fmt.Errorf("bus: outbox cleanup dead: %w", err)
	}
	return nil
}

// storeDLQ 消息入库死信（达到 MaxAttempts / 毒消息）。无 DB 返回 ErrNoDB
// （调用方退回计数路径）。入库后 DLQ 深度 +1。
func (b *Bus) storeDLQ(ctx context.Context, msg Message, cause string, attempts int) error {
	if b.db == nil {
		return ErrNoDB
	}
	headers, _ := json.Marshal(msg.Headers)
	if _, err := b.db.Exec(ctx, `INSERT INTO bus_dlq
		(topic, message_id, kind, schema_ver, payload, headers, occurred_at, error, attempts)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		msg.Topic, msg.ID, string(msg.Kind), msg.SchemaVer,
		string(msg.Payload), string(headers), msg.OccurredAt, cause, attempts); err != nil {
		return fmt.Errorf("bus: dlq insert: %w", err)
	}
	b.metrics.BusDLQDepth.Inc()
	return nil
}

// ReplayDLQ 重放死信：重建信封以 Cluster 域重发布，成功后删除该行。
// 重放后再次失败会经正常投递链重新进入 DLQ。
func (b *Bus) ReplayDLQ(ctx context.Context, id int64) error {
	if b.db == nil {
		return ErrNoDB
	}
	var msg Message
	var payload, headers string
	err := b.db.QueryRow(ctx, `SELECT topic, message_id, kind, schema_ver,
		payload, headers, occurred_at FROM bus_dlq WHERE id = $1`, id).Scan(
		&msg.Topic, &msg.ID, &msg.Kind, &msg.SchemaVer, &payload, &headers, &msg.OccurredAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: dlq id %d", ErrNotFound, id)
		}
		return fmt.Errorf("bus: dlq scan: %w", err)
	}
	msg.Payload = json.RawMessage(payload)
	if msg.Kind == "" {
		// 毒行（kind 空 = 不可重放标记）：payload 是整条原文，重放无意义，
		// 拒绝并提示人工取数（certify 后可手改 kind 走重放）
		return fmt.Errorf("%w: dlq id %d (raw retained for inspection)", ErrPoisonRow, id)
	}
	if err := json.Unmarshal([]byte(headers), &msg.Headers); err != nil {
		return fmt.Errorf("bus: dlq headers decode: %w", err)
	}
	if err := b.Publish(ctx, msg, WithDomain(DomainCluster)); err != nil {
		return fmt.Errorf("bus: dlq replay publish: %w", err)
	}
	if _, err := b.db.Exec(ctx, `DELETE FROM bus_dlq WHERE id = $1`, id); err != nil {
		return fmt.Errorf("bus: dlq replay delete: %w", err)
	}
	b.metrics.BusDLQDepth.Dec()
	return nil
}

// storeDeadEntry 死信入库的完整路径：从流条目重建信封。
// 信封可解析 → 存原始字段（ReplayDLQ 可无损重放）；损坏（毒消息）→ 整条
// 原文存 payload、kind 置空标记，ReplayDLQ 拒放（人工取数）。
func (b *Bus) storeDeadEntry(ctx context.Context, stream, entryID, cause string, attempts int, raw string) bool {
	msg := Message{
		ID:         entryID,
		Topic:      b.keys.streamTopic(stream),
		Payload:    json.RawMessage(raw),
		OccurredAt: b.clock.Now(),
	}
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		// 毒消息：原文留存；Kind 保持空 = 不可重放标记
		return b.storeDLQEntry(ctx, msg, "poison envelope: "+cause, attempts)
	}
	// 信封完好：字段来自信封本身（storeDLQ 序列化 headers；Kind/SchemaVer 已解出）
	return b.storeDLQEntry(ctx, msg, cause, attempts)
}
func (b *Bus) storeDLQEntry(ctx context.Context, msg Message, cause string, attempts int) bool {
	if err := b.storeDLQ(ctx, msg, cause, attempts); err != nil {
		if ctx.Err() == nil && !errors.Is(err, ErrNoDB) {
			b.logger.Warn("bus dlq store failed", "topic", msg.Topic, "id", msg.ID, "err", err)
		}
		return false
	}
	return true
}

// syncOutboxPending 校准 outbox 待发布积压 gauge（relay 跟不上/Redis 不可用可见）。
func (b *Bus) syncOutboxPending(ctx context.Context) {
	var n int
	if err := b.db.QueryRow(ctx, `SELECT count(*) FROM bus_outbox WHERE status = $1`, outboxPending).Scan(&n); err != nil {
		return // 校准失败不告警（下轮重试）
	}
	b.metrics.BusOutboxPending.Set(float64(n))
}

// syncDLQDepth 用表行数校准 DLQ 深度 gauge（防增减漂移）。
func (b *Bus) syncDLQDepth(ctx context.Context) {
	if b.db == nil {
		return
	}
	var n int
	if err := b.db.QueryRow(ctx, `SELECT count(*) FROM bus_dlq`).Scan(&n); err != nil {
		return // 校准失败不告警（下轮重试）
	}
	b.metrics.BusDLQDepth.Set(float64(n))
}
