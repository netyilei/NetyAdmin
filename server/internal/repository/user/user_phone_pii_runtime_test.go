package user

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	userEntity "NetyAdmin/internal/domain/entity/user"
	"NetyAdmin/internal/pkg/pii"
)

// TestUserPhonePIIRuntime 真库验证手机号 PII 加密全链路（0059）：
//   - 写：Create/UpdatePhone 明文 → 三列密文/盲索引/尾号，明文列恒空；
//   - 读：GetByID 解密回填明文；GetByPhone/ExistsByPhone 走盲索引；
//   - 唯一性：phone_idx 部分唯一索引拦截并发同号（23505）；
//   - 筛选：尾号 4 位精确；掩码值写入被防御性忽略。
//
// 自建临时库（同 user_token_repo_runtime_test 的安全门），PG 不可达自动 Skip。
func TestUserPhonePIIRuntime(t *testing.T) {
	db := newPhonePIIRuntimeDB(t)
	c := pii.NewCipher("test-master-key-32-bytes-long!!")
	repo := NewUserRepository(db, c)
	ctx := context.Background()

	// 1. Create：明文进、三列落、明文列空
	u := &userEntity.User{ID: "01HPIITESTUSER0001", Username: "pii_alice", Phone: "13812345678", Status: "1"}
	require.NoError(t, repo.Create(ctx, u))
	row := struct {
		Phone      *string
		PhoneEnc   string
		PhoneIdx   string
		PhoneTail4 string
	}{}
	require.NoError(t, db.Raw(
		`SELECT phone, phone_enc, phone_idx, phone_tail4 FROM users WHERE id = ?`, u.ID).
		Scan(&row).Error)
	assert.Empty(t, row.Phone, "明文列必须为空")
	assert.NotEmpty(t, row.PhoneEnc)
	assert.Equal(t, c.Index("13812345678"), row.PhoneIdx)
	assert.Equal(t, "5678", row.PhoneTail4)

	// 2. GetByID：解密回填明文
	got, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "13812345678", got.Phone)

	// 3. GetByPhone / ExistsByPhone：走盲索引（含 +86 形态归一化）
	byPhone, err := repo.GetByPhone(ctx, "+8613812345678")
	require.NoError(t, err)
	assert.Equal(t, u.ID, byPhone.ID)
	exists, err := repo.ExistsByPhone(ctx, "138 1234-5678")
	require.NoError(t, err)
	assert.True(t, exists)

	// 4. 唯一索引：另一用户同号 → Create 撞部分唯一索引（23505）
	dup := &userEntity.User{ID: "01HPIITESTUSER0002", Username: "pii_bob", Phone: "13812345678", Status: "1"}
	err = repo.Create(ctx, dup)
	require.Error(t, err, "同号第二行必须被 phone_idx 唯一索引拦截")

	// 5. UpdatePhone（换号）：三列更新、旧号检索失效、新号检索生效
	require.NoError(t, repo.UpdatePhone(ctx, u.ID, "13999999999"))
	_, err = repo.GetByPhone(ctx, "13812345678")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "旧号检索应落空")
	got2, err := repo.GetByPhone(ctx, "13999999999")
	require.NoError(t, err)
	assert.Equal(t, u.ID, got2.ID)
	assert.Equal(t, "13999999999", got2.Phone, "读回填明文")

	// 6. 尾号筛选：9999 命中 alice、5678 落空
	list, total, err := repo.List(ctx, &UserRepoQuery{Phone: "9999", Current: 1, Size: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, u.ID, list[0].ID)
	_, total2, err := repo.List(ctx, &UserRepoQuery{Phone: "5678", Current: 1, Size: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total2, "旧号尾号应落空")

	// 7. Update（Save 全字段）携带明文 Phone（GetByID 回填值）→ 落库仍密文、明文列空
	got3, _ := repo.GetByID(ctx, u.ID)
	got3.Nickname = "masked-check"
	require.NoError(t, repo.Update(ctx, got3))
	row2 := struct {
		Phone    *string
		PhoneEnc string
	}{}
	require.NoError(t, db.Raw(`SELECT phone, phone_enc FROM users WHERE id = ?`, u.ID).Scan(&row2).Error)
	assert.Empty(t, row2.Phone, "Save 回写不得复活明文列")
	assert.NotEmpty(t, row2.PhoneEnc)

	// 8. 掩码值防御：Update 实体 Phone 为掩码 → 三列保留、明文列仍空
	got4, _ := repo.GetByID(ctx, u.ID)
	got4.Phone = "139****9999"
	require.NoError(t, repo.Update(ctx, got4))
	got5, _ := repo.GetByID(ctx, u.ID)
	assert.Equal(t, "13999999999", got5.Phone, "掩码写入不得破坏原手机号")

	// 9. email 全链：Create 明文→密文列、明文列空、大小写归一化检索、唯一索引、掩码防御
	ue := &userEntity.User{ID: "01HPIITESTUSER0003", Username: "pii_carol", Email: "Carol@Example.COM", Status: "1"}
	require.NoError(t, repo.Create(ctx, ue))
	erow := struct {
		Email    *string
		EmailEnc string
		EmailIdx string
	}{}
	require.NoError(t, db.Raw(`SELECT email, email_enc, email_idx FROM users WHERE id = ?`, ue.ID).Scan(&erow).Error)
	assert.Empty(t, erow.Email, "email 明文列必须为空")
	assert.NotEmpty(t, erow.EmailEnc)
	assert.Equal(t, c.IndexEmail("carol@example.com"), erow.EmailIdx, "盲索引小写归一化")

	ge, err := repo.GetByEmail(ctx, "CAROL@example.COM")
	require.NoError(t, err)
	assert.Equal(t, "Carol@Example.COM", ge.Email, "解密回填保留原始大小写")

	dupE := &userEntity.User{ID: "01HPIITESTUSER0004", Username: "pii_dave", Email: "carol@example.com", Status: "1"}
	err = repo.Create(ctx, dupE)
	require.Error(t, err, "同邮箱（大小写归一后）必须被 email_idx 唯一索引拦截")

	require.NoError(t, repo.UpdateEmail(ctx, ue.ID, "carol.new@example.com"))
	_, err = repo.GetByEmail(ctx, "Carol@Example.COM")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "旧邮箱检索应落空")
	ge2, err := repo.GetByEmail(ctx, "carol.new@example.com")
	require.NoError(t, err)
	assert.Equal(t, ue.ID, ge2.ID)

	// email 掩码写入防御
	ge3, _ := repo.GetByEmail(ctx, "carol.new@example.com")
	ge3.Email = "c***@example.com"
	require.NoError(t, repo.Update(ctx, ge3))
	ge4, _ := repo.GetByEmail(ctx, "carol.new@example.com")
	assert.Equal(t, "carol.new@example.com", ge4.Email, "掩码写入不得破坏原邮箱")
}

// newPhonePIIRuntimeDB 连接维护库、创建 PID 命名临时库、AutoMigrate users
// 并补建 phone_idx 部分唯一索引（迁移 0059 的索引 GORM 无法自动生成）。
func newPhonePIIRuntimeDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("NETYADMIN_TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=localhost user=root password=123456 dbname=postgres sslmode=disable"
	}
	maint, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("pg 不可达: %v", err)
	}
	ctx := context.Background()
	dbName := "netyadmin_rt_phone_pii_test"
	maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + dbName)
	require.NoError(t, maint.WithContext(ctx).Exec("CREATE DATABASE "+dbName).Error)
	t.Cleanup(func() { maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + dbName) })

	testDSN, err := replaceDBName(dsn, dbName)
	require.NoError(t, err)
	require.NotEqual(t, dsn, testDSN, "DSN 替换必须生效（拒绝在真实库建表）")
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{})
	require.NoError(t, err)
	var current string
	require.NoError(t, db.WithContext(ctx).Raw("SELECT current_database()").Scan(&current).Error)
	require.Equal(t, dbName, current, "非测试库，拒绝执行 DDL")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	require.NoError(t, db.WithContext(ctx).AutoMigrate(&userEntity.User{}))
	// 0059/0060 的部分唯一索引 GORM 无法自动生成，测试 setup 手工补建
	require.NoError(t, db.WithContext(ctx).Exec(
		`CREATE UNIQUE INDEX users_phone_idx_key ON users(phone_idx)
		 WHERE deleted_at = 0 AND phone_idx IS NOT NULL AND phone_idx <> ''`).Error)
	require.NoError(t, db.WithContext(ctx).Exec(
		`CREATE UNIQUE INDEX users_email_idx_key ON users(email_idx)
		 WHERE deleted_at = 0 AND email_idx IS NOT NULL AND email_idx <> ''`).Error)
	return db
}
