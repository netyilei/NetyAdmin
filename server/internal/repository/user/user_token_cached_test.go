package user

import (
	"encoding/json"
	"testing"
	"time"

	userEntity "NetyAdmin/internal/domain/entity/user"
)

// TestUserTokenCacheEntryRoundtrip 验证缓存 entry 经 JSON 序列化往返后
// 两个 hash 不丢失——这是 user_tokens 缓存丢哈希 BUG（json:"-" 地雷）的回归测试。
func TestUserTokenCacheEntryRoundtrip(t *testing.T) {
	now := time.Now().Truncate(time.Second) // JSON 时间精度对齐
	src := &userEntity.UserToken{
		ID: 7, UserID: "01HTEST", Platform: "web",
		TokenVersion:     3,
		AccessHash:       "aabbccddeeff00112233",
		RefreshHash:      "11223344556677889900",
		AccessExpiresAt:  &now,
		RefreshExpiresAt: &now,
	}

	data, err := json.Marshal(newUserTokenCacheEntry(src))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var entry userTokenCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := entry.toUserToken()

	if got.AccessHash != src.AccessHash || got.RefreshHash != src.RefreshHash {
		t.Fatalf("hashes lost in round-trip: access=%q refresh=%q, want %q/%q",
			got.AccessHash, got.RefreshHash, src.AccessHash, src.RefreshHash)
	}
	if got.TokenVersion != src.TokenVersion || got.Platform != src.Platform || got.UserID != src.UserID {
		t.Fatalf("field mismatch: %+v vs src", got)
	}
}
