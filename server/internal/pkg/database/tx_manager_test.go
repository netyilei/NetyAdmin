package database

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testEntity 仅用于事务测试的临时表实体，不参与业务，也不做软删除。
type testEntity struct {
	ID   uint   `gorm:"primaryKey"`
	Name string
}

// setupTestDB 构造一个内存级 SQLite 库并完成 AutoMigrate，
// 每个用例独立一份，避免用例间数据污染。
//
// 注意：SQLite ":memory:" 是 per-connection 的（每个连接独享一份内存库），
// 若不限制连接池大小，AutoMigrate 在 conn1 建表后，后续查询可能落到 conn2
// 而报 "no such table"。这里强制 SetMaxOpenConns(1) 让所有操作复用同一连接，
// 既保证表结构可见，也使事务提交/回滚后的可见性断言稳定可靠。
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NotNil(t, db)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(&testEntity{}))
	return db
}

// TestTransactionManager_BeginCommit 验证：提交后写入的数据在外部连接可见。
func TestTransactionManager_BeginCommit(t *testing.T) {
	db := setupTestDB(t)
	tm := NewTransactionManager(db)
	ctx := context.Background()

	txCtx, tx := tm.Begin(ctx)
	require.NotNil(t, tx)
	require.NoError(t, tx.DB.Error)

	// 在事务内插入一条记录。
	require.NoError(t, tx.DB.Create(&testEntity{Name: "alice"}).Error)

	require.NoError(t, tm.Commit(tx))

	// 提交后：fallback 连接可见。
	var count int64
	require.NoError(t, db.Model(&testEntity{}).Where("name = ?", "alice").Count(&count).Error)
	assert.Equal(t, int64(1), count)

	// 防止 unused 警告：txCtx 用于下游传播，这里仅占位。
	_ = txCtx
}

// TestTransactionManager_BeginRollback 验证：回滚后写入的数据对外部不可见。
func TestTransactionManager_BeginRollback(t *testing.T) {
	db := setupTestDB(t)
	tm := NewTransactionManager(db)
	ctx := context.Background()

	txCtx, tx := tm.Begin(ctx)
	require.NotNil(t, tx)
	require.NoError(t, tx.DB.Error)

	require.NoError(t, tx.DB.Create(&testEntity{Name: "bob"}).Error)

	// 回滚应静默成功，不抛错。
	tm.Rollback(tx)

	var count int64
	require.NoError(t, db.Model(&testEntity{}).Where("name = ?", "bob").Count(&count).Error)
	assert.Equal(t, int64(0), count, "回滚后记录不应存在")

	_ = txCtx
}

// TestWithTransaction_Success 验证：fn 返回 nil 时事务正常提交，写入数据持久化可见。
func TestWithTransaction_Success(t *testing.T) {
	db := setupTestDB(t)
	tm := NewTransactionManager(db)
	ctx := context.Background()

	err := tm.WithTransaction(ctx, func(txCtx context.Context) error {
		// 通过 GetDB 取事务 DB 写入，模拟 Repository 视角。
		txDB := GetDB(txCtx, db)
		require.NotNil(t, txDB)
		return txDB.Create(&testEntity{Name: "alice"}).Error
	})
	require.NoError(t, err)

	// 提交后：fallback 连接可见
	var count int64
	require.NoError(t, db.Model(&testEntity{}).Where("name = ?", "alice").Count(&count).Error)
	assert.Equal(t, int64(1), count, "提交后记录应可见")
}

// TestWithTransaction_Error 验证：fn 返回 error 时自动 Rollback，错误向上传播，写入数据不持久化。
func TestWithTransaction_Error(t *testing.T) {
	db := setupTestDB(t)
	tm := NewTransactionManager(db)
	ctx := context.Background()

	wantErr := errors.New("boom")
	err := tm.WithTransaction(ctx, func(txCtx context.Context) error {
		txDB := GetDB(txCtx, db)
		require.NotNil(t, txDB)
		require.NoError(t, txDB.Create(&testEntity{Name: "bob"}).Error)
		return wantErr // 模拟业务错误
	})
	assert.ErrorIs(t, err, wantErr, "应返回 fn 抛出的错误")

	// 回滚后：数据不可见
	var count int64
	require.NoError(t, db.Model(&testEntity{}).Where("name = ?", "bob").Count(&count).Error)
	assert.Equal(t, int64(0), count, "fn 返回 error 后事务应回滚，记录不应存在")
}

// TestWithTransaction_Panic 验证：fn panic 时自动 Rollback，panic 向上传播，写入数据不持久化。
// panic 重抛是设计要求：让上层 recovery 中间件捕获 + Sentry 上报。
func TestWithTransaction_Panic(t *testing.T) {
	db := setupTestDB(t)
	tm := NewTransactionManager(db)
	ctx := context.Background()

	var gotPanic interface{}
	func() {
		defer func() {
			gotPanic = recover()
		}()
		_ = tm.WithTransaction(ctx, func(txCtx context.Context) error {
			txDB := GetDB(txCtx, db)
			require.NotNil(t, txDB)
			require.NoError(t, txDB.Create(&testEntity{Name: "carol"}).Error)
			panic("kaboom")
		})
	}()
	assert.Equal(t, "kaboom", gotPanic, "panic 应被重抛向上传播")

	// 回滚后：数据不可见
	var count int64
	require.NoError(t, db.Model(&testEntity{}).Where("name = ?", "carol").Count(&count).Error)
	assert.Equal(t, int64(0), count, "panic 后事务应回滚，记录不应存在")
}

// TestGetDB_WithTx_Commits 验证：通过 GetDB 取到的事务 DB 写入后可被 Commit 持久化。
func TestGetDB_WithTx_Commits(t *testing.T) {
	db := setupTestDB(t)
	tm := NewTransactionManager(db)
	ctx := context.Background()

	txCtx, tx := tm.Begin(ctx)

	// Repository 视角：通过 GetDB 从 context 取事务 DB。
	txDB := GetDB(txCtx, db)
	require.NotNil(t, txDB)
	require.NoError(t, txDB.Create(&testEntity{Name: "dave"}).Error)

	require.NoError(t, tm.Commit(tx))

	var count int64
	require.NoError(t, db.Model(&testEntity{}).Where("name = ?", "dave").Count(&count).Error)
	assert.Equal(t, int64(1), count, "GetDB 写入应随事务提交持久化")
}

// TestGetDB_WithTx_RollsBack 验证：通过 GetDB 取到的事务 DB 写入可被 Rollback 撤销。
func TestGetDB_WithTx_RollsBack(t *testing.T) {
	db := setupTestDB(t)
	tm := NewTransactionManager(db)
	ctx := context.Background()

	txCtx, tx := tm.Begin(ctx)

	txDB := GetDB(txCtx, db)
	require.NotNil(t, txDB)
	require.NoError(t, txDB.Create(&testEntity{Name: "carol"}).Error)

	tm.Rollback(tx)

	var count int64
	require.NoError(t, db.Model(&testEntity{}).Where("name = ?", "carol").Count(&count).Error)
	assert.Equal(t, int64(0), count, "GetDB 写入应随事务回滚被撤销")
}

// TestGetDB_WithoutTx 验证：context 中无事务时，GetDB 返回 fallback 并可立即生效。
func TestGetDB_WithoutTx(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	got := GetDB(ctx, db)
	require.NotNil(t, got)
	require.NoError(t, got.Create(&testEntity{Name: "eve"}).Error)

	var count int64
	require.NoError(t, db.Model(&testEntity{}).Where("name = ?", "eve").Count(&count).Error)
	assert.Equal(t, int64(1), count, "无事务时应直连 fallback 并立即写入")
}

// TestContext_TxRetrieval 验证：Begin 返回的 context 可通过 TxKey 取回同一 *Tx。
func TestContext_TxRetrieval(t *testing.T) {
	db := setupTestDB(t)
	tm := NewTransactionManager(db)
	ctx := context.Background()

	txCtx, tx := tm.Begin(ctx)

	got, ok := txCtx.Value(TxKey).(*Tx)
	assert.True(t, ok, "context 中应能取出 *Tx")
	assert.Same(t, tx, got, "取出的 *Tx 应与 Begin 返回的是同一指针")
	assert.NotNil(t, got.DB, "Tx.DB 应非空")
}
