package user

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	userEntity "NetyAdmin/internal/domain/entity/user"
)

// TestUpdateHashesRequiresRowRuntime 真库验证 token hash 回写的契约：
// UPDATE 命中 0 行必须显式报错（gorm.ErrRecordNotFound），不能静默成功；
// 先 UpsertAndIncrement 建行后同一调用必须成功落库；同值重写仍算命中。
// 自建临时库（DSN 替换后校验 current_database），PG 不可达自动 Skip。
func TestUpdateHashesRequiresRowRuntime(t *testing.T) {
	db := newRuntimePG(t)
	repo := NewUserTokenRepository(db)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	exp := now.Add(time.Hour)

	t.Run("missing row returns ErrRecordNotFound", func(t *testing.T) {
		assert.ErrorIs(t, repo.UpdateHashes(ctx, "missing", "web", "a1", "r1", exp, exp),
			gorm.ErrRecordNotFound)
		assert.ErrorIs(t, repo.UpdateAccessHash(ctx, "missing", "web", "a1", exp),
			gorm.ErrRecordNotFound)
	})

	t.Run("upsert then update succeeds", func(t *testing.T) {
		_, err := repo.UpsertAndIncrement(ctx, &userEntity.UserToken{UserID: "exists", Platform: "web"})
		require.NoError(t, err)
		require.NoError(t, repo.UpdateHashes(ctx, "exists", "web", "a1", "r1", exp, exp))

		got, err := repo.GetByPlatform(ctx, "exists", "web")
		require.NoError(t, err)
		assert.Equal(t, "a1", got.AccessHash)
		assert.Equal(t, "r1", got.RefreshHash)
		assert.Equal(t, uint64(1), got.TokenVersion)
	})

	t.Run("rewriting identical hashes still matches the row", func(t *testing.T) {
		// PostgreSQL 的 UPDATE 计数包含"新值等于旧值"的行；同一秒内签发的
		// token 字符串可能完全相同，刷新回写同值不能误报 0 行。
		require.NoError(t, repo.UpdateHashes(ctx, "exists", "web", "a1", "r1", exp, exp))
	})
}

// newRuntimePG 连接维护库、创建以 PID 命名的临时库并 AutoMigrate，
// 返回测试库连接；测试结束关闭连接后 DROP。PG 不可达自动 Skip。
func newRuntimePG(t *testing.T) *gorm.DB {
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
	dbName := fmt.Sprintf("netyadmin_rt_user_token_%d", os.Getpid())
	maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + dbName)
	require.NoError(t, maint.WithContext(ctx).Exec("CREATE DATABASE "+dbName).Error)
	// 建库成功立即注册兜底清理：后续任何一步失败也不泄漏临时库
	// （LIFO：真正的"关连接+删库"清理后注册、先执行，这里幂等兜底）。
	t.Cleanup(func() { maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + dbName) })

	testDSN, err := replaceDBName(dsn, dbName)
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{})
	require.NoError(t, err)

	// 安全门：DSN 替换必须生效，绝不能在真实库上建表/写数据。
	var current string
	require.NoError(t, db.WithContext(ctx).Raw("SELECT current_database()").Scan(&current).Error)
	require.Equal(t, dbName, current, "DSN 替换失败，拒绝在非测试库执行 DDL")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB.Close()
		maint.WithContext(ctx).Exec("DROP DATABASE IF EXISTS " + dbName)
	})

	require.NoError(t, db.WithContext(ctx).AutoMigrate(&userEntity.UserToken{}))
	return db
}

// replaceDBName 同时支持 keyword DSN（dbname=...）与 URL DSN（postgres://.../dbname）。
func replaceDBName(dsn, dbName string) (string, error) {
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
