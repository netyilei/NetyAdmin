// Package database 提供数据库相关基础设施：健康检查、事务管理器等。
//
// 本文件实现基于 context 传播的事务管理器：
//   - TransactionManager 封装 GORM 事务的 Begin/Commit/Rollback 语义；
//   - 通过 context.WithValue 在 Service ↔ Repository 之间隐式传递事务句柄；
//   - 提供 WithTransaction 闭包 API，自动处理 panic / error 路径的 Rollback；
//   - 维护活跃事务计数器（atomic），优雅关闭时可检测未提交事务并告警；
//   - TransactionManager 自身可作为单例在 DI 中复用，并发安全。
package database

import (
	"context"
	"log/slog"
	"sync/atomic"

	"gorm.io/gorm"
)

// txKeyType 是 context.Value 的 key 类型，避免与其他包冲突。
// 使用私有空结构体类型，外部无法伪造同类型 key。
type txKeyType struct{}

// TxKey 是事务句柄在 context 中的存取 key。
var TxKey txKeyType

// Tx 封装一次数据库事务句柄。
//
// 字段仅在同包内可见：Service 层通过 TransactionManager 的方法操作 *Tx，
// 不直接读写其字段；Repository 层通过 GetDB(ctx, fallback) 间接取到 *gorm.DB。
//
// NOT safe for concurrent use. Expected to be used by a single goroutine per transaction.
type Tx struct {
	// DB 是 GORM 事务句柄（由 tm.db.WithContext(ctx).Begin() 得到）。
	DB *gorm.DB
}

// TransactionManager 事务管理器：可作为单例在应用生命周期内复用。
//
// activeTxCount 维护当前活跃事务数（Begin 增 / Commit 或 Rollback 减），
// 用于优雅关闭时检测是否有未提交事务（若有则 slog.Error 告警，避免数据丢失）。
// 使用 atomic.Int64 保证并发安全。
type TransactionManager struct {
	db            *gorm.DB
	activeTxCount atomic.Int64
}

// TxManager 是事务管理器的抽象接口，支持 Mock 测试。
//
// 所有 Service 应依赖此接口而非 *TransactionManager 具体类型，
// 便于在服务层单元测试中用 mock 替代真实事务管理器，无需数据库连接。
type TxManager interface {
	// Begin 开启一个新事务，并将其写入返回的 context 中。
	Begin(ctx context.Context) (context.Context, *Tx)
	// Commit 提交事务。
	Commit(tx *Tx) error
	// Rollback 回滚事务。
	Rollback(tx *Tx)
	// WithTransaction 执行闭包内的事务，自动处理 panic 与 error 时的 Rollback。
	WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
	// ActiveTransactions 返回当前活跃事务数。
	ActiveTransactions() int64
}

// compile-time check: *TransactionManager 满足 TxManager 接口
var _ TxManager = (*TransactionManager)(nil)

// NewTransactionManager 构造事务管理器实例。
//
// 传入的 db 通常是应用启动时初始化好的 *gorm.DB 连接池，
// 后续每次 Begin 都会基于它派生新事务。
func NewTransactionManager(db *gorm.DB) *TransactionManager {
	return &TransactionManager{db: db}
}

// Begin 开启一个新事务，并将其写入返回的 context 中。
//
// 返回值：
//   - txCtx：携带 *Tx 的新 context，传给下游 Service/Repository 即可让它们复用同一事务；
//   - tx：事务句柄，调用方负责调用 Commit 或 Rollback 收尾。
//
// 注意：不在 panic 时自动 Rollback，调用方需自行 defer 兜底；
// Begin 本身的错误会体现在 tx.DB.Error 中，调用方应在使用前检查。
// 每次成功 Begin 会递增活跃事务计数器，配套的 Commit/Rollback 会递减。
func (tm *TransactionManager) Begin(ctx context.Context) (context.Context, *Tx) {
	db := tm.db.WithContext(ctx).Begin()
	tx := &Tx{DB: db}
	tm.activeTxCount.Add(1)
	return context.WithValue(ctx, TxKey, tx), tx
}

// Commit 提交事务。
//
// 返回提交时的 error：调用方据此决定是否上抛业务错误。
// 提交后需要执行的副作用（如缓存失效）应由调用方在 Commit 成功后自行处理，
// 这样可以保证「DB 已提交 → 缓存失效」的顺序，避免「缓存已清但 DB 回滚」中间态。
// 无论提交成功或失败都会递减活跃事务计数器（事务已结束）。
func (tm *TransactionManager) Commit(tx *Tx) error {
	err := tx.DB.Commit().Error
	tm.activeTxCount.Add(-1)
	return err
}

// Rollback 回滚事务。无返回值：调用方通常在错误路径上调用，
// 失败仅以 slog.Warn 记录（如重复提交/回滚导致的 sql.ErrTxDone），不向上传播。
// 无论回滚成功或失败都会递减活跃事务计数器（事务已结束）。
func (tm *TransactionManager) Rollback(tx *Tx) {
	if err := tx.DB.Rollback().Error; err != nil {
		slog.Warn("transaction rollback failed", "error", err)
	}
	tm.activeTxCount.Add(-1)
}

// ActiveTransactions 返回当前活跃事务数（已 Begin 但尚未 Commit/Rollback）。
// 主要用于优雅关闭时检测是否有未提交事务：若 > 0 应 slog.Error 告警，
// 提示运维人员可能有数据丢失风险（事务未提交就被强制关闭）。
// 使用 atomic.Int64 的 Load 方法，并发安全。
func (tm *TransactionManager) ActiveTransactions() int64 {
	return tm.activeTxCount.Load()
}

// WithTransaction 执行闭包内的事务，自动处理 panic 与 error 时的 Rollback。
// 适用于线性多步写场景；复杂分支场景仍可用手动 Begin/Commit/Rollback API。
// panic 时自动 Rollback 后重抛，让上层中间件（recovery）记录。
//
// 用法：
//
//	err := tm.WithTransaction(ctx, func(txCtx context.Context) error {
//	    if err := s.repo.A(txCtx, ...); err != nil { return err }
//	    if err := s.repo.B(txCtx, ...); err != nil { return err }
//	    return nil
//	})
//	if err != nil { return err }
//	// 事务后副作用（如缓存失效）用原始 ctx 在此执行
//
// 注意：fn 内的所有 repo 调用必须传 txCtx（非原始 ctx）。
// 事务后副作用（缓存失效、事件发布等）应在 WithTransaction 返回 nil 后用原始 ctx 执行，
// 以保证「DB 已提交 → 副作用执行」的顺序。
func (tm *TransactionManager) WithTransaction(
	ctx context.Context,
	fn func(txCtx context.Context) error,
) (err error) {
	txCtx, tx := tm.Begin(ctx)
	defer func() {
		if p := recover(); p != nil {
			tm.Rollback(tx)
			panic(p) // 重抛让 recovery 中间件捕获 + Sentry 上报
		}
	}()
	if err = fn(txCtx); err != nil {
		tm.Rollback(tx)
		return err
	}
	return tm.Commit(tx)
}

// GetDB 是 Repository 层的统一取 DB 入口：根据 context 中是否携带事务，
// 返回事务内的 *gorm.DB 或回退到 fallback。
//
// 典型用法：
//
//	func (r *UserRepo) getDB(ctx context.Context) *gorm.DB {
//	    return database.GetDB(ctx, r.db)
//	}
//
// 行为：
//   - 若 ctx 中存在 *Tx 且非 nil：返回 tx.DB.WithContext(ctx)，使后续操作落入该事务；
//   - 否则：返回 fallback.WithContext(ctx)，正常走连接池。
//
// 注意每次调用都会返回新的 *gorm.DB 实例（GORM 的链式 API 语义），
// 但底层连接/事务句柄由 GORM 内部共享，不会产生额外连接开销。
func GetDB(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(TxKey).(*Tx); ok && tx != nil {
		return tx.DB.WithContext(ctx)
	}
	return fallback.WithContext(ctx)
}
