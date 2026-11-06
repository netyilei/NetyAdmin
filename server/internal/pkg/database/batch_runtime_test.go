package database

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestDeleteBatchBeforeRuntime 真库验证：终止条件（恰好多批+余数批）、只删旧行、幂等。
// 自包含：连接维护库（默认 root@localhost/123456，可用 NETYADMIN_TEST_PG_DSN 覆盖），
// 自建 PID 命名临时库并造数，结束后整体删除；PG 不可达自动 Skip。
// 临时库 setup 与 repository/user 的真库测试同一套安全门：
// DSN 替换支持 keyword/URL 两种形式，建表前断言 current_database，
// 绝不允许 DSN 替换失败时把测试 DDL 打进真实库。
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

	dbName := fmt.Sprintf("netyadmin_rt_batch_%d", os.Getpid())
	maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + dbName)
	if err := maint.WithContext(ctx).Exec("CREATE DATABASE " + dbName).Error; err != nil {
		t.Fatalf("create test db: %v", err)
	}
	// 建库成功立即注册兜底清理：后续任何一步失败也不泄漏临时库
	// （LIFO：下方"关连接+删库"先执行，这里幂等兜底）。
	t.Cleanup(func() { maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + dbName) })

	testDSN, err := replaceTestDBName(dsn, dbName)
	if err != nil {
		t.Fatalf("replace dsn: %v", err)
	}
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	// 安全门：DSN 替换必须生效，绝不能在真实库上建表/写数据。
	var current string
	if err := db.WithContext(ctx).Raw("SELECT current_database()").Scan(&current).Error; err != nil {
		t.Fatalf("select current_database: %v", err)
	}
	if current != dbName {
		t.Fatalf("DSN 替换失败：连到了 %q，拒绝在非测试库执行 DDL", current)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
		maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + dbName)
	})

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

// replaceTestDBName 同时支持 keyword DSN（dbname=...）与 URL DSN（postgres://.../dbname）。
// 与 repository/user/user_token_repo_runtime_test.go 的 replaceDBName 逻辑一致
// （跨包无法共享 _test.go，改动时两处同步）。
func replaceTestDBName(dsn, dbName string) (string, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", err
		}
		u.Path = "/" + dbName
		return u.String(), nil
	}
	re := regexp.MustCompile(`(^|\s)dbname=\S+`)
	if !re.MatchString(dsn) {
		return "", fmt.Errorf("DSN 缺少 dbname: %s", dsn)
	}
	return re.ReplaceAllString(dsn, "${1}dbname="+dbName), nil
}
