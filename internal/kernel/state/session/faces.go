// faces.go：三个面的操作——吊销检查/标记、会话存取、刷新轮换（一次性）。
package session

import (
	"context"
	"fmt"
	"time"
)

// Revoke 吊销 token 的 jti（TTL = token 剩余寿命；过期自动滚出；幂等）。
// 注意：写路径无 DB 回退（contracts read_fallback 是读向语义）——Redis 不可用
// 时吊销标记失败按错误返回，调用方（AuthN）决定是否阻断操作。
func (s *Store) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	if _, err := s.client.SetNX(ctx, s.prefixRevoked+jti, 1, ttl); err != nil {
		return fmt.Errorf("session: revoke %s: %w", jti, err)
	}
	return nil
}

// IsRevoked 检查 jti 是否已吊销。
// 语义（contracts）：Redis 失败 → single 有 DB 回调则回退、multi fail-closed。
// 已知权衡：Redis 在线但标记丢失（如重启未持久化）时返回 false——
// 标记 TTL 与 token 寿命对齐，正常运维下窗口为空；严格面可配 DB 强校验。
func (s *Store) IsRevoked(ctx context.Context, jti string) (bool, error) {
	revoked, err := s.client.Exists(ctx, s.prefixRevoked+jti)
	if err == nil {
		return revoked, nil
	}
	if s.topo == TopologySingle && s.revDB != nil {
		if s.metrics != nil {
			s.metrics.SessionDBFallbacks.WithLabelValues("revocation").Inc()
		}
		dbRevoked, dbErr := s.revDB(ctx, jti)
		if dbErr != nil {
			// 回退失败=系统不可用（fail-closed）——并入 ErrUnavailable 供调用方
			// errors.Is 区分系统面（10010）与 token 面（101xx）
			return false, fmt.Errorf("%w: revocation db fallback: %w", ErrUnavailable, dbErr)
		}
		return dbRevoked, nil
	}
	if s.metrics != nil {
		s.metrics.SessionFailClosed.WithLabelValues("revocation").Inc()
	}
	return false, fmt.Errorf("%w: is_revoked %s: %w", ErrUnavailable, jti, err)
}

// —— 会话存储（policy: session_store；Redis + DB 读回退） ——

// PutSession 存会话负载（登录/轮换时写）。
// 写路径无 DB 回退（read_fallback 是读向语义；写失败按错误返回）。
func (s *Store) PutSession(ctx context.Context, sessionID string, payload []byte, ttl time.Duration) error {
	if err := s.client.Set(ctx, s.prefixStore+sessionID, payload, ttl); err != nil {
		return fmt.Errorf("session: put %s: %w", sessionID, err)
	}
	return nil
}

// GetSession 读会话负载：Redis → 未命中或不可用时按策略走 DB 回退
// （read_fallback: db 覆盖两个场景：宕机 + Redis 过期但 DB 仍有）。
func (s *Store) GetSession(ctx context.Context, sessionID string) ([]byte, bool, error) {
	payload, ok, err := s.client.Get(ctx, s.prefixStore+sessionID)
	if err == nil && ok {
		return payload, true, nil
	}
	// Redis 未命中（ok=false, err=nil）或不可用（err≠nil）：
	// 未命中不需要策略裁决（read_fallback: db 主场景）；不可用需按拓扑裁决
	if err == nil || s.dbFallbackAllowed("session_store") {
		if s.sessionDB != nil {
			if err != nil && s.metrics != nil {
				s.metrics.SessionDBFallbacks.WithLabelValues("session_store").Inc() // 宕机回退（未命中不算降级）
			}
			payload, found, dbErr := s.sessionDB(ctx, sessionID)
			if dbErr != nil {
				return nil, false, fmt.Errorf("%w: session db fallback: %w", ErrUnavailable, dbErr)
			}
			return payload, found, nil
		}
		if err == nil {
			return nil, false, nil // 未命中且无 DB 回调
		}
	}
	if s.metrics != nil {
		s.metrics.SessionFailClosed.WithLabelValues("session_store").Inc()
	}
	return nil, false, fmt.Errorf("session: get %s: %w", sessionID, err)
}

// DropSession 删除会话（登出/吊销联动）。
func (s *Store) DropSession(ctx context.Context, sessionID string) error {
	if err := s.client.Del(ctx, s.prefixStore+sessionID); err != nil {
		return fmt.Errorf("session: drop %s: %w", sessionID, err)
	}
	return nil
}

// —— 刷新轮换（policy: refresh_rotation；fail_closed 双拓扑） ——

// rotateLua 刷新轮换原子操作：旧 token 属于该会话 → 删旧、写新（一次性语义）。
const rotateLua = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	redis.call("DEL", KEYS[1])
	redis.call("SET", KEYS[2], ARGV[2], "PX", ARGV[3])
	return 1
end
return 0
`

// RotateRefresh 刷新令牌轮换：校验旧 refresh 属于该会话 → 原子作废旧、签发位。
// newToken 由调用方（AuthN）生成传入；此处只管理 Redis 侧的一次性占用。
// 任一失败返回 ErrDenied/错误——fail_closed 双拓扑（contracts refresh_rotation）。
func (s *Store) RotateRefresh(ctx context.Context, oldToken, newToken, sessionID string, ttl time.Duration) error {
	v, err := s.client.Eval(ctx, rotateLua,
		[]string{s.prefixRefresh + oldToken, s.prefixRefresh + newToken},
		sessionID, sessionID, ttl.Milliseconds())
	if err != nil {
		return fmt.Errorf("session: rotate %s: %w", sessionID, err)
	}
	if n, _ := v.(int64); n == 0 {
		return ErrDenied
	}
	return nil
}

// BindRefresh 登录时绑定首个 refresh token → 会话。
func (s *Store) BindRefresh(ctx context.Context, token, sessionID string, ttl time.Duration) error {
	ok, err := s.client.SetNX(ctx, s.prefixRefresh+token, sessionID, ttl)
	if err != nil {
		return fmt.Errorf("session: bind refresh: %w", err)
	}
	if !ok {
		return ErrDenied // token 冲突（重新生成即可）
	}
	return nil
}
