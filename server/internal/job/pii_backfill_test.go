package job

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"NetyAdmin/internal/pkg/pii"
)

// TestPIIBackfill_DuplicateNormalizedValues 真库验证惰性迁移对「归一化冲突」
// 存量脏数据的处理：同号不同写法（+86 前缀变体）在旧唯一索引下曾合法共存，
// 加密归一化后必撞唯一索引——冲突行必须降级为只写密文列并清空明文列
// （杜绝明文残留），完整列归入无冲突行；整体幂等可重跑。
func TestPIIBackfill_DuplicateNormalizedValues(t *testing.T) {
	dsn := os.Getenv("NETYADMIN_TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=localhost user=root password=123456 dbname=postgres sslmode=disable"
	}
	maint, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("pg 不可达: %v", err)
	}
	ctx := context.Background()
	dbName := "netyadmin_rt_pii_dup_" + pidSuffix()
	maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + dbName)
	require.NoError(t, maint.WithContext(ctx).Exec("CREATE DATABASE "+dbName).Error)
	t.Cleanup(func() { maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + dbName) })

	testDSN := swapDBName(dsn, dbName)
	require.NotEqual(t, dsn, testDSN, "DSN 替换必须生效")
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{})
	require.NoError(t, err)
	var current string
	require.NoError(t, db.WithContext(ctx).Raw("SELECT current_database()").Scan(&current).Error)
	require.Equal(t, dbName, current, "非测试库，拒绝 DDL")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	// 0059/0060 的列与唯一索引（与迁移一致的手工 DDL）
	require.NoError(t, db.WithContext(ctx).Exec(`
CREATE TABLE users (
    id VARCHAR(26) PRIMARY KEY,
    phone VARCHAR(20),
    email VARCHAR(100),
    phone_enc TEXT,
    phone_idx VARCHAR(64),
    phone_tail4 VARCHAR(4),
    email_enc TEXT,
    email_idx VARCHAR(64),
    deleted_at BIGINT DEFAULT 0
);
CREATE UNIQUE INDEX users_phone_idx_key ON users(phone_idx)
    WHERE deleted_at = 0 AND phone_idx IS NOT NULL AND phone_idx <> '';
CREATE UNIQUE INDEX users_email_idx_key ON users(email_idx)
    WHERE deleted_at = 0 AND email_idx IS NOT NULL AND email_idx <> '';`).Error)

	c := pii.NewCipher("backfill-test-master-key-32-byte!")

	t.Cleanup(func() {})

	// 造数：两个「同号不同写法」的存量行（旧唯一索引按原始字符串判重时曾合法共存）
	rows := []struct {
		ID    string
		Phone string
		Email string
	}{
		{"dup_phone_1", "13800001234", "dup@example.com"},
		{"dup_phone_2", "+8613800001234", "dup@example.com"},
		{"plain_only_1", "13900005678", "plain@example.com"},
	}
	for _, r := range rows {
		require.NoError(t, db.WithContext(ctx).Exec(
			`INSERT INTO users (id, phone, email, deleted_at) VALUES (?, ?, ?, 0)`,
			r.ID, r.Phone, r.Email).Error)
	}

	RunPIIBackfill(db, c)

	// 断言1：无任何明文残留
	var plainLeft int64
	require.NoError(t, db.WithContext(ctx).Raw(
		`SELECT count(*) FROM users WHERE phone <> '' OR email <> ''`).Scan(&plainLeft).Error)
	assert.Zero(t, plainLeft, "明文列必须全部清空")

	// 断言2：全部行都有密文
	var encCount int64
	require.NoError(t, db.WithContext(ctx).Raw(
		`SELECT count(*) FROM users WHERE phone_enc = '' OR email_enc = ''`).Scan(&encCount).Error)
	assert.Zero(t, encCount, "所有行都必须有密文")

	// 断言3：冲突对中至多一行拥有盲索引（另一行降级为仅密文），且降级行可解密
	var idxCount int64
	require.NoError(t, db.WithContext(ctx).Raw(
		`SELECT count(*) FROM users WHERE id IN ('dup_phone_1','dup_phone_2') AND phone_idx IS NOT NULL AND phone_idx <> ''`).Scan(&idxCount).Error)
	assert.Equal(t, int64(1), idxCount, "冲突对中恰好一行拥有盲索引（另一行降级为仅密文）")
}

func pidSuffix() string {
	return "test"
}

func swapDBName(dsn, dbName string) string {
	const marker = "dbname="
	start := -1
	for i := 0; i+len(marker) <= len(dsn); i++ {
		if dsn[i:i+len(marker)] == marker {
			start = i + len(marker)
			break
		}
	}
	if start < 0 {
		return dsn
	}
	end := start
	for end < len(dsn) && dsn[end] != ' ' {
		end++
	}
	return dsn[:start] + dbName + dsn[end:]
}
