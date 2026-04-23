package captcha

import (
	"context"
	"strconv"
	"time"

	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/configsync"

	"gorm.io/gorm"
)

// captchaToken 验证码数据库模型
type captchaToken struct {
	ID        uint      `gorm:"primaryKey"`
	CaptchaID string    `gorm:"column:captcha_id;uniqueIndex;not null"`
	Answer    string    `gorm:"column:answer;not null"`
	ExpireAt  time.Time `gorm:"column:expire_at;index;not null"`
}

func (captchaToken) TableName() string {
	return "captcha_tokens"
}

// dualStore 实现 base64Captcha.Store 接口，支持缓存和数据库双轨存储
type dualStore struct {
	cache   cache.LazyCacheManager
	watcher configsync.ConfigWatcher
	db      *gorm.DB
}

func NewDualStore(cache cache.LazyCacheManager, watcher configsync.ConfigWatcher, db *gorm.DB) *dualStore {
	return &dualStore{
		cache:   cache,
		watcher: watcher,
		db:      db,
	}
}

func (s *dualStore) isCacheEnabled() bool {
	return s.watcher.IsCacheEnabled("captcha")
}

func (s *dualStore) getTTL() time.Duration {
	expireStr, exists := s.watcher.GetConfig("captcha_config", "captcha_expire")
	if !exists {
		return 10 * time.Minute
	}

	seconds, _ := strconv.Atoi(expireStr)
	if seconds <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(seconds) * time.Second
}

func (s *dualStore) Set(id string, value string) error {
	ttl := s.getTTL()

	if s.isCacheEnabled() {
		return s.cache.Set(context.Background(), cache.KeyCaptchaToken(id), value, ttl)
	}

	// 数据库模式
	token := &captchaToken{
		CaptchaID: id,
		Answer:    value,
		ExpireAt:  time.Now().Add(ttl),
	}
	return s.db.Create(token).Error
}

func (s *dualStore) Get(id string, clear bool) string {
	ctx := context.Background()
	var answer string

	if s.isCacheEnabled() {
		err := s.cache.Get(ctx, cache.KeyCaptchaToken(id), &answer)
		if err != nil {
			return ""
		}
		if clear {
			_ = s.cache.Delete(ctx, cache.KeyCaptchaToken(id))
		}
		return answer
	}

	// 数据库模式
	var token captchaToken
	err := s.db.Where("captcha_id = ? AND expire_at > ?", id, time.Now()).First(&token).Error
	if err != nil {
		return ""
	}

	answer = token.Answer
	if clear {
		s.db.Delete(&token)
	}

	return answer
}

func (s *dualStore) Verify(id, answer string, clear bool) bool {
	v := s.Get(id, clear)
	return v == answer
}
