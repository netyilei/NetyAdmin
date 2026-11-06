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

// TestUpdateHashesRequiresRowRuntime 真库验证 token hash 回写的 CAS 契约：
// UPDATE 按 (token_version, refresh_hash) 乐观守卫——期望状态匹配才写入，
// 0 行（行缺失 / 并发 Login 递增版本 / Logout 清空 hash）必须显式报错
// （gorm.ErrRecordNotFound），不能静默成功；ClearHashes 按 access_hash
// 守卫且幂等（0 行 = 成功）。自建临时库（DSN 替换后校验 current_database），
// PG 不可达自动 Skip。
func TestUpdateHashesRequiresRowRuntime(t *testing.T) {
	db := newRuntimePG(t)
	repo := NewUserTokenRepository(db)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	exp := now.Add(time.Hour)

	t.Run("missing row: updates fail, clear is noop success", func(t *testing.T) {
		assert.ErrorIs(t, repo.UpdateHashes(ctx, "missing", "web", 1, "r1", "a1", "r1", exp, exp),
			gorm.ErrRecordNotFound)
		assert.ErrorIs(t, repo.UpdateAccessHash(ctx, "missing", "web", 1, "a1", exp),
			gorm.ErrRecordNotFound)
		assert.NoError(t, repo.ClearHashes(ctx, "missing", "web", "a1"), "登出幂等：无行也是成功")
	})

	t.Run("login CAS (platTV, empty hash) succeeds", func(t *testing.T) {
		// Login 的真实顺序：UpsertAndIncrement（EXCLUDED 清空 hash）→ CAS 回写
		v, err := repo.UpsertAndIncrement(ctx, &userEntity.UserToken{UserID: "alice", Platform: "web"})
		require.NoError(t, err)
		require.Equal(t, uint64(1), v)
		require.NoError(t, repo.UpdateHashes(ctx, "alice", "web", v, "", "a1", "r1", exp, exp))

		got, err := repo.GetByPlatform(ctx, "alice", "web")
		require.NoError(t, err)
		assert.Equal(t, "a1", got.AccessHash)
		assert.Equal(t, "r1", got.RefreshHash)
	})

	t.Run("refresh CAS with observed state succeeds, identical rewrite still matches", func(t *testing.T) {
		// 刷新路径：校验阶段读到 (v1, r1)，按同一状态回写。
		ut, err := repo.GetByPlatform(ctx, "alice", "web")
		require.NoError(t, err)
		require.NoError(t, repo.UpdateHashes(ctx, "alice", "web",
			ut.TokenVersion, ut.RefreshHash, ut.AccessHash, ut.RefreshHash, exp, exp))
		// PostgreSQL 的 UPDATE 计数包含"新值等于旧值"的行；同一秒内签发的
		// token 字符串可能完全相同，刷新回写同值不能误报 0 行。
	})

	t.Run("concurrent login bump makes stale refresh CAS fail", func(t *testing.T) {
		// 模拟并发 Login：版本递增 + EXCLUDED 清空 hash 后写新值。
		v2, err := repo.UpsertAndIncrement(ctx, &userEntity.UserToken{UserID: "alice", Platform: "web"})
		require.NoError(t, err)
		require.Equal(t, uint64(2), v2)
		require.NoError(t, repo.UpdateHashes(ctx, "alice", "web", v2, "", "a2", "r2", exp, exp))

		// 旧刷新会话仍按 (v1, r1) 回写 → 0 行报错，而不是覆盖 v2 会话的 hash。
		assert.ErrorIs(t, repo.UpdateHashes(ctx, "alice", "web", 1, "r1", "ax", "rx", exp, exp),
			gorm.ErrRecordNotFound)
		got, err := repo.GetByPlatform(ctx, "alice", "web")
		require.NoError(t, err)
		assert.Equal(t, "a2", got.AccessHash, "新会话 hash 不被旧刷新覆盖")
	})

	t.Run("logout CAS clears only the owning session", func(t *testing.T) {
		// 持有当前会话的 access hash → 清空成功
		require.NoError(t, repo.ClearHashes(ctx, "alice", "web", "a2"))
		got, err := repo.GetByPlatform(ctx, "alice", "web")
		require.NoError(t, err)
		assert.Empty(t, got.AccessHash)
		assert.Empty(t, got.RefreshHash)

		// 幂等：重复登出 / 旧 token 登出（hash 不匹配）都是成功 no-op
		require.NoError(t, repo.ClearHashes(ctx, "alice", "web", "a2"))
		require.NoError(t, repo.ClearHashes(ctx, "alice", "web", "a1"))

		// 行内 hash 已被并发 Login 更新时，旧登出不得覆盖（守卫 生效）
		v3, err := repo.UpsertAndIncrement(ctx, &userEntity.UserToken{UserID: "alice", Platform: "web"})
		require.NoError(t, err)
		require.NoError(t, repo.UpdateHashes(ctx, "alice", "web", v3, "", "a3", "r3", exp, exp))
		require.NoError(t, repo.ClearHashes(ctx, "alice", "web", "a1"), "旧 token 登出 no-op")
		got, err = repo.GetByPlatform(ctx, "alice", "web")
		require.NoError(t, err)
		assert.Equal(t, "a3", got.AccessHash, "新会话 hash 不被旧登出清掉")
	})

	t.Run("update access hash CAS", func(t *testing.T) {
		v, err := repo.UpsertAndIncrement(ctx, &userEntity.UserToken{UserID: "alice", Platform: "web"})
		require.NoError(t, err)
		require.NoError(t, repo.UpdateHashes(ctx, "alice", "web", v, "", "a4", "r4", exp, exp))

		assert.ErrorIs(t, repo.UpdateAccessHash(ctx, "alice", "web", v-1, "ax", exp),
			gorm.ErrRecordNotFound, "版本已递增 → 拒绝")
		require.NoError(t, repo.UpdateAccessHash(ctx, "alice", "web", v, "a4b", exp))
		got, err := repo.GetByPlatform(ctx, "alice", "web")
		require.NoError(t, err)
		assert.Equal(t, "a4b", got.AccessHash)
		assert.Equal(t, "r4", got.RefreshHash, "UpdateAccessHash 不动 refresh hash")
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
