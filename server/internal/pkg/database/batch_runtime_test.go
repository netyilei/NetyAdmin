package database

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestDeleteBatchBeforeRuntime 真库验证：终止条件（恰好多批+余数批）、只删旧行、幂等。
// 自包含：连接维护库（默认 root@localhost/123456，可用 NETYADMIN_TEST_PG_DSN 覆盖），
// 自建临时库 netyadmin_rt_test 并造数，结束后整体删除；PG 不可达自动 Skip。
func TestDeleteBatchBeforeRuntime(t *testing.T) {
	dsn := os.Getenv("NETYADMIN_TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=localhost user=root password=123456 dbname=postgres sslmode=disable"
	}
	maint, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("pg 不可达: %v", err)
	}
	ctx := context.Background()

	const testDB = "netyadmin_rt_test"
	maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + testDB)
	if err := maint.WithContext(ctx).Exec("CREATE DATABASE " + testDB).Error; err != nil {
		t.Fatalf("create test db: %v", err)
	}
	defer maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + testDB)

	db, err := gorm.Open(postgres.Open(strings.Replace(dsn, "dbname=postgres", "dbname="+testDB, 1)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	// 自造数：2500 旧（10 天前）+ 300 新（当前）
	if err := db.WithContext(ctx).Exec(`CREATE TABLE sys_task_logs (id BIGSERIAL PRIMARY KEY, name VARCHAR(50), created_at TIMESTAMPTZ DEFAULT now())`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := db.WithContext(ctx).Exec(`INSERT INTO sys_task_logs (name, created_at)
SELECT 'old', now() - interval '10 days' FROM generate_series(1, 2500)
UNION ALL SELECT 'new', now() FROM generate_series(1, 300)`).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	deleted, err := DeleteBatchBefore(ctx, db, "sys_task_logs", time.Now().Add(-5*24*time.Hour), 1000)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if deleted != 2500 {
		t.Errorf("删除=%d 期望2500（多批+余数批终止）", deleted)
	}
	var after int64
	db.WithContext(ctx).Raw("SELECT count(*) FROM sys_task_logs").Scan(&after)
	if after != 300 {
		t.Errorf("剩余=%d 期望300（新行不受影响）", after)
	}
	// 二次调用应删 0（幂等）
	deleted2, _ := DeleteBatchBefore(ctx, db, "sys_task_logs", time.Now().Add(-5*24*time.Hour), 1000)
	if deleted2 != 0 {
		t.Errorf("二次删除=%d 期望0", deleted2)
	}
}
