// inbox.go：消费去重原语（Inbox 行："表或 Redis；TTL；
// 与业务幂等键配合"）。Redis SET NX EX 实现：首见返回 true（处理），
// 重复返回 false（跳过）。调用方流程：Acquire→true 才执行业务；崩溃后
// TTL 内重投会被跳过（延迟重处理）——消费方幂等仍是最终兜底。
// Redis 故障返回错误：调用方默认跳过处理（消息留 PEL 重投）即保持
// at-least-once；不建本地回退（无 Redis 单机无重投语义，G1-15）。
package bus

import (
	"context"
	"fmt"
	"time"
)

// Inbox 消费去重句柄（经 Bus.Inbox() 获取；local-only 模式为 nil）。
type Inbox struct {
	face redisFace
	keys keySet
}

// Inbox 取消费去重句柄；local-only 模式返回 nil（Acquire 不可用）。
func (b *Bus) Inbox() *Inbox {
	if b.redis == nil {
		return nil
	}
	return &Inbox{face: b.redis, keys: b.keys}
}

// Acquire 标记业务键首次出现：true=首见（调用方处理），false=重复（跳过）。
// key 为业务幂等键或消息 ID；ttl 到期后键消失（同 key 可再次处理）。
// Redis 错误原样返回——调用方跳过处理即保持 at-least-once。
func (i *Inbox) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if i == nil {
		return false, ErrNoRedis // local-only：无重投语义，去重不可用
	}
	if key == "" || len(key) > inboxKeyMaxLen {
		return false, fmt.Errorf("%w: length must be 1..%d", ErrInvalidKey, inboxKeyMaxLen)
	}
	if ttl <= 0 {
		return false, fmt.Errorf("%w: ttl must be positive", ErrInvalidTTL)
	}
	first, err := i.face.SetNX(ctx, i.keys.inboxKey(key), ttl)
	if err != nil {
		return false, fmt.Errorf("bus: inbox acquire: %w", err)
	}
	return first, nil
}
