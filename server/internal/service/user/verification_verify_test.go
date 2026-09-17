package user

import (
	"context"
	"testing"
	"time"

	"NetyAdmin/internal/pkg/cache"
)

// TestVerifyAndClearCode 覆盖验证码原子消费核心语义（修复后实现）：
// 正确码一次性消费、错误码保留可重试、5 次上限作废、已消费/不存在失败。
func TestVerifyAndClearCode(t *testing.T) {
	ctx := context.Background()

	newSvc := func() (*verificationService, *mockUserCacheMgr) {
		mgr := newMockUserCacheMgr()
		return &verificationService{cacheSlow: mgr}, mgr
	}
	codeKey := func(scene, target string) string { return cache.KeyVerificationCode(scene, target) }

	t.Run("correct code consumed once", func(t *testing.T) {
		svc, mgr := newSvc()
		_ = mgr.Set(ctx, codeKey("s", "t"), "123456", time.Minute)
		ok, err := svc.VerifyAndClearCode(ctx, "s", "t", "123456")
		if err != nil || !ok {
			t.Fatalf("first verify: ok=%v err=%v, want true/nil", ok, err)
		}
		ok, err = svc.VerifyAndClearCode(ctx, "s", "t", "123456")
		if err != nil || ok {
			t.Fatalf("second verify (replay): ok=%v err=%v, want false/nil", ok, err)
		}
	})

	t.Run("wrong attempt keeps code and counts", func(t *testing.T) {
		svc, mgr := newSvc()
		_ = mgr.Set(ctx, codeKey("s", "t"), "123456", time.Minute)
		if ok, _ := svc.VerifyAndClearCode(ctx, "s", "t", "000000"); ok {
			t.Fatal("wrong code should fail")
		}
		// 错误尝试后正确码仍可用（回写维持尝试窗口）
		ok, err := svc.VerifyAndClearCode(ctx, "s", "t", "123456")
		if err != nil || !ok {
			t.Fatalf("correct code after wrong attempt: ok=%v err=%v", ok, err)
		}
		// 尝试计数已 +1
		var attempt string
		_ = mgr.Get(ctx, cache.KeyVerifyCodeAttempt("s", "t"), &attempt)
		if attempt != "1" {
			t.Fatalf("attempt count = %q, want 1", attempt)
		}
	})

	t.Run("five wrong attempts void the code", func(t *testing.T) {
		svc, mgr := newSvc()
		_ = mgr.Set(ctx, codeKey("s", "t"), "123456", time.Minute)
		for i := 0; i < 5; i++ {
			if ok, _ := svc.VerifyAndClearCode(ctx, "s", "t", "000000"); ok {
				t.Fatalf("attempt %d should fail", i+1)
			}
		}
		ok, err := svc.VerifyAndClearCode(ctx, "s", "t", "123456")
		if err != nil || ok {
			t.Fatalf("correct code after 5 wrong attempts: ok=%v err=%v, want false/nil (voided)", ok, err)
		}
	})

	t.Run("missing code fails without error", func(t *testing.T) {
		svc, _ := newSvc()
		ok, err := svc.VerifyAndClearCode(ctx, "s", "never", "123456")
		if err != nil {
			t.Fatalf("missing code should be (false, nil), got err=%v", err)
		}
		if ok {
			t.Fatal("missing code should not verify")
		}
	})
}
