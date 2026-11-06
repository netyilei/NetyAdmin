package user

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	userEntity "NetyAdmin/internal/domain/entity/user"
)

// TestUpdateHashesRequiresRowRuntime 真库验证 token hash 回写的契约：
// UPDATE 命中 0 行必须显式报错（gorm.ErrRecordNotFound），不能静默成功；
// 先 UpsertAndIncrement 建行后同一调用必须成功落库。
// 自建临时库，PG 不可达自动 Skip。
func TestUpdateHashesRequiresRowRuntime(t *testing.T) {
	dsn := os.Getenv("NETYADMIN_TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=localhost user=root password=123456 dbname=postgres sslmode=disable"
	}
	maint, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("pg 不可达: %v", err)
	}
	ctx := context.Background()

	const testDB = "netyadmin_rt_user_token_test"
	maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + testDB)
	if err := maint.WithContext(ctx).Exec("CREATE DATABASE " + testDB).Error; err != nil {
		t.Fatalf("create test db: %v", err)
	}

	db, err := gorm.Open(postgres.Open(strings.Replace(dsn, "dbname=postgres", "dbname="+testDB, 1)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer func() {
		sqlDB.Close()
		maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + testDB)
	}()

	if err := db.WithContext(ctx).Exec(`
CREATE TABLE user_tokens (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            VARCHAR(26) NOT NULL,
    platform           VARCHAR(50) NOT NULL,
    token_version      BIGINT NOT NULL DEFAULT 0,
    access_hash        VARCHAR(64) NOT NULL DEFAULT '',
    refresh_hash       VARCHAR(64) NOT NULL DEFAULT '',
    access_expires_at  TIMESTAMP WITH TIME ZONE,
    refresh_expires_at TIMESTAMP WITH TIME ZONE,
    created_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, platform)
)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}

	repo := NewUserTokenRepository(db)
	now := time.Now().Truncate(time.Second)
	exp := now.Add(time.Hour)

	// 1. 行不存在：两个 hash 回写方法都必须报 ErrRecordNotFound。
	err = repo.UpdateHashes(ctx, "u1", "web", "a1", "r1", exp, exp)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("UpdateHashes missing row: err = %v, want gorm.ErrRecordNotFound", err)
	}
	err = repo.UpdateAccessHash(ctx, "u1", "web", "a1", exp)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("UpdateAccessHash missing row: err = %v, want gorm.ErrRecordNotFound", err)
	}

	// 2. 先建行（Login 的真实顺序），随后回写必须成功。
	if _, err := repo.UpsertAndIncrement(ctx, &userEntity.UserToken{UserID: "u1", Platform: "web"}); err != nil {
		t.Fatalf("UpsertAndIncrement: %v", err)
	}
	if err := repo.UpdateHashes(ctx, "u1", "web", "a1", "r1", exp, exp); err != nil {
		t.Fatalf("UpdateHashes existing row: %v", err)
	}
	got, err := repo.GetByPlatform(ctx, "u1", "web")
	if err != nil {
		t.Fatalf("GetByPlatform: %v", err)
	}
	if got.AccessHash != "a1" || got.RefreshHash != "r1" || got.TokenVersion != 1 {
		t.Fatalf("row = %+v, want hash a1/r1 version 1", got)
	}
}
