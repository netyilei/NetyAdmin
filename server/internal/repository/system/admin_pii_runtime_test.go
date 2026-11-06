package system

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	systemEntity "NetyAdmin/internal/domain/entity/system"
	"NetyAdmin/internal/pkg/pii"
)

// TestAdminPhonePIIRuntime 真库验证 admin_user 的 PII 加密链路：
//   - GetByID/GetByUsername/List 三个读出口都必须解密回填（第 5 轮批量 patch
//     曾丢失 GetByID 的回填——本测试是该回归的守卫）；
//   - Create/Update 写路径 seal、掩码防御、空明文保持原列；
//   - List 的 phone/email 筛选走盲索引/尾号列。
func TestAdminPhonePIIRuntime(t *testing.T) {
	db := newAdminPIIRuntimeDB(t)
	c := pii.NewCipher("admin-test-master-key-32-byte!")
	repo := NewAdminRepository(db, c)
	ctx := context.Background()

	a := &systemEntity.Admin{Username: "pii_admin", Password: "x", Nickname: "n",
		Phone: "13812345678", Email: "Admin@Example.COM", Status: "1"}
	require.NoError(t, repo.Create(ctx, a))

	// 1. GetByID 必须回填（回归守卫：该出口曾丢失 fillPII）
	byID, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, "13812345678", byID.Phone, "GetByID 必须解密回填手机号")
	assert.Equal(t, "Admin@Example.COM", byID.Email, "GetByID 必须解密回填邮箱（保留大小写）")

	// 2. GetByUsername 同一守卫
	byName, err := repo.GetByUsername(ctx, "pii_admin")
	require.NoError(t, err)
	assert.Equal(t, "13812345678", byName.Phone)

	// 3. List 回填 + 盲索引筛选（完整号精确 / 尾号精确 / email 大小写不敏感）
	list, total, err := repo.List(ctx, &AdminRepoQuery{Phone: "13812345678", Current: 1, Size: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	assert.Equal(t, "13812345678", list[0].Phone, "List 必须回填")

	_, totalTail, err := repo.List(ctx, &AdminRepoQuery{Phone: "5678", Current: 1, Size: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalTail, "尾号筛选")

	_, totalMail, err := repo.List(ctx, &AdminRepoQuery{Email: "admin@example.com", Current: 1, Size: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalMail, "email 筛选（大小写归一化）")

	// 4. 掩码回写防御：Save 掩码值不得破坏原手机号/邮箱
	byID.Phone = "138****5678"
	byID.Email = "A***@Example.COM"
	require.NoError(t, repo.Update(ctx, byID))
	again, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, "13812345678", again.Phone)
	assert.Equal(t, "Admin@Example.COM", again.Email)

	// 5. 解密失败守卫（错密钥）：空明文 Save 保持密文列
	wrong := NewAdminRepository(db, pii.NewCipher("wrong-admin-key-32-bytes-long."))
	degraded, err := wrong.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.Empty(t, degraded.Phone, "错密钥降级为空")
	var encBefore string
	require.NoError(t, db.Raw(`SELECT phone_enc FROM admin_user WHERE id = ?`, a.ID).Scan(&encBefore).Error)
	require.NotEmpty(t, encBefore)
	require.NoError(t, wrong.Update(ctx, degraded))
	var encAfter string
	require.NoError(t, db.Raw(`SELECT phone_enc FROM admin_user WHERE id = ?`, a.ID).Scan(&encAfter).Error)
	assert.Equal(t, encBefore, encAfter, "解密失败后的 Save 不得清空密文")
}

// newAdminPIIRuntimeDB 建 PID 命名临时库并 AutoMigrate Admin 及其关联表。
func newAdminPIIRuntimeDB(t *testing.T) *gorm.DB {
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
	dbName := fmt.Sprintf("netyadmin_rt_admin_pii_%d", os.Getpid())
	maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + dbName)
	require.NoError(t, maint.WithContext(ctx).Exec("CREATE DATABASE "+dbName).Error)
	t.Cleanup(func() { maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + dbName) })

	testDSN := replaceAdminTestDBName(dsn, dbName)
	require.NotEqual(t, dsn, testDSN, "DSN 替换必须生效")
	// DisableForeignKeyConstraintWhenMigrating：生产迁移 0007 未建外键，AutoMigrate
	// 默认会按实体关系生成自关联外键（Create 时 UpdatedBy=0 会违反）——与生产对齐。
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	var current string
	require.NoError(t, db.WithContext(ctx).Raw("SELECT current_database()").Scan(&current).Error)
	require.Equal(t, dbName, current, "非测试库，拒绝 DDL")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	require.NoError(t, db.WithContext(ctx).AutoMigrate(&systemEntity.Admin{}))
	return db
}

// replaceAdminTestDBName 与 user 包测试同名 helper 同逻辑（跨包 _test 不可共享）。
func replaceAdminTestDBName(dsn, dbName string) string {
	// DSN 均为 keyword 形式（env 或默认值）；user 包 helper 已校验 URL 形式存在性
	const marker = "dbname="
	i := len(dsn)
	for j := 0; j+len(marker) <= len(dsn); j++ {
		if dsn[j:j+len(marker)] == marker {
			i = j
			break
		}
	}
	if i == len(dsn) {
		return dsn
	}
	start := i + len(marker)
	end := start
	for end < len(dsn) && dsn[end] != ' ' {
		end++
	}
	return dsn[:start] + dbName + dsn[end:]
}
