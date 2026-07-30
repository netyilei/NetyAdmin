// record_test.go 上传记录 CompleteUpload 状态机单元测试基线。
//
// 覆盖范围（Task 9.4）：
//   - CompleteUpload 成功：pending → uploaded
//   - secret 校验失败（错误签名）→ CodeUploadSignatureInvalid
//   - record.Secret 为空 → CodeUploadSignatureInvalid
//   - status 已 uploaded → CodeUploadRecordCompleted（防重复提交）
//   - status 已 expired → CodeUploadRecordExpired
//   - objectKey 不匹配 → CodeUploadRecordMismatch
//   - record 不存在 → CodeUploadRecordNotFound
//   - pending 但 ExpiresAt 已过 → CodeUploadRecordExpired
//
// Mock 策略：
//   - recordRepo 使用手写 mock 结构体
//   - tm 使用 mockTxManager（database.TxManager 接口的内存实现），不依赖真实数据库
//   - hmacKey 使用已知常量，配合 utils.SignUploadRecord 生成有效签名
//   - configSvc / storageMgr / appSvc 在 CompleteUpload 路径不使用，置 nil
//
// 注：CompleteUpload 内部使用 tm.WithTransaction 调用 LockRecordByID + FlipStatusToUploaded。
package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	storageEntity "NetyAdmin/internal/domain/entity/storage"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/utils"
	storageRepo "NetyAdmin/internal/repository/storage"
)

// ============== mockRecordRepo：storageRepo.RecordRepository 内存实现 ==============
//
// 仅实现 CompleteUpload 路径用到的方法（GetByID / LockRecordByID / FlipStatusToUploaded），
// 其余方法返回零值；通过字段控制返回值与错误注入。
type mockRecordRepo struct {
	getByIDRecord *storageEntity.Record
	getByIDErr    error

	lockRecord    *storageEntity.Record
	lockRecordErr error

	flipResult bool
	flipErr    error

	// flipCalls 用于断言 FlipStatusToUploaded 是否被调用
	flipCalls int
}

func (r *mockRecordRepo) GetByID(_ context.Context, _ uint) (*storageEntity.Record, error) {
	if r.getByIDErr != nil {
		return nil, r.getByIDErr
	}
	return r.getByIDRecord, nil
}

func (r *mockRecordRepo) LockRecordByID(_ context.Context, _ uint) (*storageEntity.Record, error) {
	if r.lockRecordErr != nil {
		return nil, r.lockRecordErr
	}
	return r.lockRecord, nil
}

func (r *mockRecordRepo) FlipStatusToUploaded(_ context.Context, _ uint, _ int64, _, _ string, _ string) (bool, error) {
	r.flipCalls++
	if r.flipErr != nil {
		return false, r.flipErr
	}
	return r.flipResult, nil
}

// 以下方法 CompleteUpload 不使用，仅满足接口签名
func (r *mockRecordRepo) Create(_ context.Context, _ *storageEntity.Record) error { return nil }
func (r *mockRecordRepo) UpdateSecret(_ context.Context, _ uint, _ string) error  { return nil }
func (r *mockRecordRepo) Delete(_ context.Context, _ uint) error                  { return nil }
func (r *mockRecordRepo) DeleteMultiple(_ context.Context, _ []uint) error        { return nil }
func (r *mockRecordRepo) CleanupExpiredPending(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (r *mockRecordRepo) GetByMD5(_ context.Context, _ string) (*storageEntity.Record, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *mockRecordRepo) List(_ context.Context, _ *storageRepo.RecordQuery) ([]*storageEntity.Record, int64, error) {
	return nil, 0, nil
}
func (r *mockRecordRepo) GetByStorageConfigID(_ context.Context, _ uint) ([]*storageEntity.Record, error) {
	return nil, nil
}
func (r *mockRecordRepo) GetBySource(_ context.Context, _ storageEntity.UploadSource, _ string) ([]*storageEntity.Record, error) {
	return nil, nil
}
func (r *mockRecordRepo) GetByBusiness(_ context.Context, _ string, _ string) ([]*storageEntity.Record, error) {
	return nil, nil
}

var _ storageRepo.RecordRepository = (*mockRecordRepo)(nil)

// ============== 测试夹具 ==============

const testHMACKey = "TestHMACKey-2025-ABC!@#def123"

// newTestRecordService 构造 recordService + 配套 mocks。
// configSvc / storageMgr / appSvc 在 CompleteUpload 路径不使用，置 nil。
func newTestRecordService(t *testing.T) (*recordService, *mockRecordRepo) {
	t.Helper()
	repo := &mockRecordRepo{}

	svc := &recordService{
		recordRepo: repo,
		configSvc:  nil,
		storageMgr: nil,
		appSvc:     nil,
		tm:         &database.MockTxManager{},
		hmacKey:    testHMACKey,
	}
	return svc, repo
}

// signTestRecord 用测试 HMAC key 生成有效签名
func signTestRecord(t *testing.T, recordID uint, objectKey, source, sourceID string, expiresAtUnix int64) string {
	t.Helper()
	return utils.SignUploadRecord(testHMACKey, recordID, objectKey, source, sourceID, expiresAtUnix)
}

// makePendingRecord 构造一个 pending 状态的 record（含有效签名）。
// id 通过嵌入的 entity.Model.ID 设置，保证签名与验签 ID 一致。
func makePendingRecord(t *testing.T, id uint) *storageEntity.Record {
	t.Helper()
	expiresAt := time.Now().Add(30 * time.Minute)
	objectKey := "test/object-key-001"
	source := storageEntity.UploadSourceAdmin
	sourceID := "01HTESTADMINID001"
	secret := signTestRecord(t, id, objectKey, string(source), sourceID, expiresAt.Unix())

	r := &storageEntity.Record{
		StorageConfigID: 1,
		FileName:        "test.png",
		FilePath:        objectKey,
		FileSize:        1024,
		MimeType:        "image/png",
		Source:          source,
		SourceID:        sourceID,
		Status:          storageEntity.RecordStatusPending,
		Secret:          secret,
		ExpiresAt:       &expiresAt,
	}
	r.ID = id // 通过嵌入的 entity.Model.ID 设置，确保签名验签 ID 一致
	return r
}

// ============== CompleteUpload 测试 ==============

// TestCompleteUpload_Success 验证：pending record + 有效签名 → 翻转为 uploaded
func TestCompleteUpload_Success(t *testing.T) {
	svc, repo := newTestRecordService(t)
	record := makePendingRecord(t, 100)
	repo.getByIDRecord = record
	repo.lockRecord = record // 行锁返回同一 record（pending 状态）
	repo.flipResult = true   // FlipStatusToUploaded 成功

	vo, err := svc.CompleteUpload(context.Background(),
		100,
		record.Secret,
		record.FilePath,
		"https://cdn.example.com/test.png",
		2048,
		"image/png",
		"abc123md5")

	require.NoError(t, err)
	require.NotNil(t, vo)
	assert.Equal(t, "test.png", vo.FileName)
	assert.Equal(t, 1, repo.flipCalls, "应调用 FlipStatusToUploaded 一次")
}

// TestCompleteUpload_RecordNotFound 验证：record 不存在 → CodeUploadRecordNotFound
func TestCompleteUpload_RecordNotFound(t *testing.T) {
	svc, repo := newTestRecordService(t)
	repo.getByIDErr = gorm.ErrRecordNotFound

	_, err := svc.CompleteUpload(context.Background(), 999, "any-secret", "any-key", "", 0, "", "")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUploadRecordNotFound, bizErr.Code)
}

// TestCompleteUpload_EmptySecret 验证：record.Secret 为空 → CodeUploadSignatureInvalid
// （凭证签发异常导致 secret 未写入）
func TestCompleteUpload_EmptySecret(t *testing.T) {
	svc, repo := newTestRecordService(t)
	record := makePendingRecord(t, 101)
	record.Secret = "" // 模拟 secret 未写入
	repo.getByIDRecord = record

	_, err := svc.CompleteUpload(context.Background(), 101, "any-secret", record.FilePath, "", 0, "", "")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUploadSignatureInvalid, bizErr.Code)
	assert.Equal(t, 0, repo.flipCalls, "secret 校验失败不应进入事务")
}

// TestCompleteUpload_SecretInvalid 验证：secret 签名不匹配 → CodeUploadSignatureInvalid
func TestCompleteUpload_SecretInvalid(t *testing.T) {
	svc, repo := newTestRecordService(t)
	record := makePendingRecord(t, 102)
	repo.getByIDRecord = record

	_, err := svc.CompleteUpload(context.Background(), 102, "wrong-secret", record.FilePath, "", 0, "", "")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUploadSignatureInvalid, bizErr.Code)
	assert.Equal(t, 0, repo.flipCalls, "secret 校验失败不应进入事务")
}

// TestCompleteUpload_ObjectKeyMismatch 验证：客户端上报的 objectKey 与凭证绑定不一致 → CodeUploadRecordMismatch
func TestCompleteUpload_ObjectKeyMismatch(t *testing.T) {
	svc, repo := newTestRecordService(t)
	record := makePendingRecord(t, 103)
	repo.getByIDRecord = record

	_, err := svc.CompleteUpload(context.Background(),
		103,
		record.Secret,
		"different/object-key", // 与 record.FilePath 不匹配
		"", 0, "", "")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUploadRecordMismatch, bizErr.Code)
	assert.Equal(t, 0, repo.flipCalls, "objectKey 校验失败不应进入事务")
}

// TestCompleteUpload_StatusAlreadyUploaded 验证：status 已 uploaded → CodeUploadRecordCompleted（防重复提交）
func TestCompleteUpload_StatusAlreadyUploaded(t *testing.T) {
	svc, repo := newTestRecordService(t)
	record := makePendingRecord(t, 104)
	record.Status = storageEntity.RecordStatusUploaded // 已上传
	repo.getByIDRecord = record

	_, err := svc.CompleteUpload(context.Background(),
		104, record.Secret, record.FilePath, "", 0, "", "")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUploadRecordCompleted, bizErr.Code)
	assert.Equal(t, 0, repo.flipCalls, "已 uploaded 不应进入事务")
}

// TestCompleteUpload_StatusExpired 验证：status 已 expired → CodeUploadRecordExpired
func TestCompleteUpload_StatusExpired(t *testing.T) {
	svc, repo := newTestRecordService(t)
	record := makePendingRecord(t, 105)
	record.Status = storageEntity.RecordStatusExpired // 已过期
	repo.getByIDRecord = record

	_, err := svc.CompleteUpload(context.Background(),
		105, record.Secret, record.FilePath, "", 0, "", "")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUploadRecordExpired, bizErr.Code)
	assert.Equal(t, 0, repo.flipCalls, "已 expired 不应进入事务")
}

// TestCompleteUpload_PendingButExpired 验证：status=pending 但 ExpiresAt 已过 → CodeUploadRecordExpired
func TestCompleteUpload_PendingButExpired(t *testing.T) {
	svc, repo := newTestRecordService(t)
	record := makePendingRecord(t, 106)
	// 覆盖 ExpiresAt 为过去时间，并重签 secret 以保持签名有效
	pastTime := time.Now().Add(-1 * time.Minute)
	record.ExpiresAt = &pastTime
	record.Secret = signTestRecord(t, record.ID, record.FilePath, string(record.Source), record.SourceID, pastTime.Unix())
	repo.getByIDRecord = record

	_, err := svc.CompleteUpload(context.Background(),
		106, record.Secret, record.FilePath, "", 0, "", "")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUploadRecordExpired, bizErr.Code)
	assert.Equal(t, 0, repo.flipCalls, "已过期（ExpiresAt）不应进入事务")
}

// TestCompleteUpload_LockStatusNotPending 验证：行锁下 status 已非 pending（并发竞争）→ 友好错误
// 模拟场景：GetByID 时 status=pending，但 LockRecordByID 时 status 已变（并发已翻转）
func TestCompleteUpload_LockStatusNotPending(t *testing.T) {
	svc, repo := newTestRecordService(t)
	pendingRecord := makePendingRecord(t, 107)
	repo.getByIDRecord = pendingRecord

	// 行锁返回的 record status 已变（并发已翻转）
	uploadedRecord := *pendingRecord // 浅拷贝
	uploadedRecord.Status = storageEntity.RecordStatusUploaded
	repo.lockRecord = &uploadedRecord
	repo.flipResult = false

	_, err := svc.CompleteUpload(context.Background(),
		107, pendingRecord.Secret, pendingRecord.FilePath, "", 0, "", "")

	require.Error(t, err)
	var bizErr *errorx.BizError
	require.True(t, errors.As(err, &bizErr))
	assert.Equal(t, errorx.CodeUploadRecordCompleted, bizErr.Code, "行锁下已 uploaded 应返回 CodeUploadRecordCompleted")
}
