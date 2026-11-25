// tx.go：UnitOfWork——ctx 传播的事务管理器（禁跨模块事务）。
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// txKey 是 ctx 中的事务标识（同事务内嵌套调用共享同一 uow）。
type txKey struct{}

// ErrNoTxInContext 保护性哨兵：OnCommit/OnRollback 只能在事务内注册。
var ErrNoTxInContext = errors.New("database: no transaction in context")

// ErrCommittedCallback 是「事务已提交但 OnCommit 回调失败」的哨兵：
// 数据已落库（不可重试业务），调用方只能告警/补偿回调动作本身，
// 严禁以此错误触发业务重试（会重复执行已提交的操作）。
var ErrCommittedCallback = errors.New("database: committed but callback failed")

// WithinTx 在事务中执行 fn：
//   - ctx 已在事务内（嵌套）→ 复用同一事务与回调列表（fn 注册的回调随外层统一执行）；
//   - 否则 Begin → fn → 成功：commit 后按注册序执行 OnCommit 回调（缓存失效等
//     「commit 之后」的动作挂这里，G2）→ 失败/panic：rollback 后执行 OnRollback。
//
// fn 内通过 RegisterOnCommit/RegisterOnRollback 注册回调；仓储层用 TxFrom(ctx)
// 取事务句柄执行 SQL（nil=不在事务内，回退池直查）。
// 注意契约：嵌套事务无 savepoint——内层失败即整体回滚（内层 onRollback 不单独执行）。
func WithinTx(ctx context.Context, pool *Pool, fn func(ctx context.Context) error) error {
	if TxFrom(ctx) != nil {
		// 嵌套：复用外层事务（回调注册直接落到共享 uow，外层统一执行）
		return fn(ctx)
	}
	start := time.Now()
	tx, err := pool.Begin(ctx)
	if err != nil {
		pool.observeTx(ctx, "rollback", start, err)
		return fmt.Errorf("database: begin: %w", err)
	}
	uow := &unitOfWork{tx: tx}
	tctx := context.WithValue(ctx, txKey{}, uow)

	fnErr := runTxFn(tctx, fn)
	if fnErr != nil {
		_ = tx.Rollback(ctx)
		pool.observeTx(ctx, "rollback", start, nil)
		for _, cb := range uow.onRollback {
			cb(ctx)
		}
		return fnErr
	}
	if err := tx.Commit(ctx); err != nil {
		pool.observeTx(ctx, "rollback", start, err)
		for _, cb := range uow.onRollback {
			cb(ctx)
		}
		return fmt.Errorf("database: commit: %w", err)
	}
	pool.observeTx(ctx, "commit", start, nil)
	// commit 成功后才执行 OnCommit（G2：缓存失效在 commit 之后）。
	// 回调失败：数据已提交不可逆——返回 ErrCommittedCallback 包装错误
	// （调用方严禁据此重试业务，只能告警/补偿回调动作本身）；回调 panic 同样隔离转错误。
	for _, cb := range uow.onCommit {
		if err := runTxFn(ctx, cb); err != nil {
			return fmt.Errorf("%w: %w", ErrCommittedCallback, err)
		}
	}
	return nil
}

// runTxFn 执行 fn 并隔离 panic（panic 转错误走回滚路径）。
func runTxFn(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("database: panic in tx: %v", p)
		}
	}()
	return fn(ctx)
}

type unitOfWork struct {
	tx         pgx.Tx
	onCommit   []func(context.Context) error
	onRollback []func(context.Context)
}

func uowFrom(ctx context.Context) *unitOfWork {
	uow, _ := ctx.Value(txKey{}).(*unitOfWork)
	return uow
}

// TxFrom 取 ctx 中的事务句柄（nil=不在事务内，仓储层回退池直查）。
func TxFrom(ctx context.Context) pgx.Tx {
	if uow := uowFrom(ctx); uow != nil {
		return uow.tx
	}
	return nil
}

// RegisterOnCommit 注册 commit 成功后的回调（仅在事务内有效；嵌套时落到共享 uow）。
func RegisterOnCommit(ctx context.Context, cb func(context.Context) error) error {
	uow := uowFrom(ctx)
	if uow == nil {
		return ErrNoTxInContext
	}
	uow.onCommit = append(uow.onCommit, cb)
	return nil
}

// RegisterOnRollback 注册回滚后的通知回调（通知/计数类，吞错）。
func RegisterOnRollback(ctx context.Context, cb func(context.Context)) error {
	uow := uowFrom(ctx)
	if uow == nil {
		return ErrNoTxInContext
	}
	uow.onRollback = append(uow.onRollback, cb)
	return nil
}
