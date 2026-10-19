//go:build runtime

package database

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestDeleteBatchBeforeRuntime 真库验证（build tag runtime 隔离）：
// 终止条件（恰好多批+余数批）、只删旧行、返回累计数。
// 前置：localhost 可达的 PG 中已建 sys_task_logs 并造数 2500 旧 + 300 新。
func TestDeleteBatchBeforeRuntime(t *testing.T) {
	db, err := gorm.Open(postgres.Open("host=localhost user=root dbname=netyadmin_sqltest sslmode=disable"), &gorm.Config{})
	if err != nil {
		t.Skipf("pg 不可达: %v", err)
	}
	ctx := context.Background()

	var before int64
	db.WithContext(ctx).Raw("SELECT count(*) FROM sys_task_logs").Scan(&before)
	if before != 2800 {
		t.Skipf("数据未就绪 count=%d", before)
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
	db.WithContext(ctx).Exec("TRUNCATE sys_task_logs")
}
