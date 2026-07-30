package database

import "context"

// MockTxManager 是 TxManager 接口的 no-op 内存实现，专供服务层单元测试使用。
//
// WithTransaction 直接执行传入的闭包，不依赖任何真实数据库连接。
// 业务 Repository 在测试中已使用 mock 替代，因此无需实际读写数据库。
//
// 用法：
//
//	svc := &someService{
//	    tm: &database.MockTxManager{},
//	    // ... 其他 mock 依赖
//	}
type MockTxManager struct{}

func (m *MockTxManager) Begin(ctx context.Context) (context.Context, *Tx) {
	return ctx, &Tx{}
}

func (m *MockTxManager) Commit(*Tx) error { return nil }

func (m *MockTxManager) Rollback(*Tx) {}

func (m *MockTxManager) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (m *MockTxManager) ActiveTransactions() int64 { return 0 }

// compile-time check: *MockTxManager 满足 TxManager 接口
var _ TxManager = (*MockTxManager)(nil)
