package app

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/pkg/errorx"
)

// 本文件是 Bootstrap→Run→HTTP 全链的真机运行时测试（PG 可达即跑、不可达 Skip，
// 与 batch_runtime_test 同一环境门控惯例 NETYADMIN_TEST_PG_DSN）。
// 背景：单元测试全部手工构造 App/ClientRouter，真实 Bootstrap→RegisterTypedAuthModule→
// Run→typed 请求路径此前从未执行过（缺陷审计 F4）；本测试补上该缺口，并锁定
// admin_router_enabled=false 的 NoRoute 行为（不落 SPA 200，统一 404 JSON）。
//
// 数据库语义：测试自建 PID 命名临时空库、走基座自动迁移全新装路径（wire.go 的
// migration.Run 在 Bootstrap 内同步建表），结束后 DROP——不依赖、不污染开发库。
// 测试本身不写业务数据。

// parsePGKeywordDSN 解析 keyword 形式 PG DSN（"host=x user=y ..."）；URL 形式不支持。
func parsePGKeywordDSN(t *testing.T, dsn string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, part := range strings.Fields(dsn) {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && kv[0] != "" {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

// freePort 取一个当前空闲端口（经典 取用即关，存在理论竞态窗口，测试场景可接受）。
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

// createTempTestDB 自建 PID 命名临时空库并在测试结束后 DROP（batch_runtime_test 同款模式）。
// 返回临时库名；连接信息（host/port/user/password）沿用维护库 DSN。
func createTempTestDB(t *testing.T, kv map[string]string) string {
	t.Helper()
	maintDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		orDefault(kv["host"], "localhost"), orDefault(kv["port"], "5432"),
		orDefault(kv["user"], "root"), orDefault(kv["password"], "123456"))
	maint, err := gorm.Open(postgres.Open(maintDSN), &gorm.Config{})
	if err != nil {
		t.Skipf("pg 不可达: %v", err)
	}
	dbName := fmt.Sprintf("netyadmin_rt_bootstrap_%d", os.Getpid())
	maint.Exec("DROP DATABASE IF EXISTS " + dbName)
	require.NoError(t, maint.Exec("CREATE DATABASE "+dbName).Error, "create temp db")
	// WITH (FORCE)：测试的 Run goroutine（configWatcher 等后台组件）在 Cleanup 时
	// 可能重连占库，普通 DROP 会因活跃连接失败残留——FORCE 先断开全部连接再删。
	t.Cleanup(func() {
		maint.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)")
	})
	return dbName
}

// writeJWTKeyPair 生成一次性 RSA 2048 密钥对（PEM 文件），测试自包含、不依赖仓库 keys/。
func writeJWTKeyPair(t *testing.T, dir string) (privPath, pubPath string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	privPath = filepath.Join(dir, "jwt_private.pem")
	pubPath = filepath.Join(dir, "jwt_public.pem")
	require.NoError(t, os.WriteFile(privPath, privPEM, 0o600))
	require.NoError(t, os.WriteFile(pubPath, pubPEM, 0o600))
	return privPath, pubPath
}

// orDefault 空串回落默认值。
func orDefault(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

// writeRuntimeConfig 生成最小可启动配置（debug 模式跳过生产校验；Redis 关闭走 L1 降级；
// 迁移开启——临时空库走全新装路径）。adminEnabled 控制 [base].admin_router_enabled。
func writeRuntimeConfig(t *testing.T, kv map[string]string, dbName string, port int, adminEnabled bool) *config.Config {
	t.Helper()
	dir := t.TempDir()
	privPath, pubPath := writeJWTKeyPair(t, dir)

	dbPort := 5432
	if p := kv["port"]; p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			dbPort = v
		}
	}

	var b strings.Builder
	b.WriteString("server:\n")
	fmt.Fprintf(&b, "  port: %d\n", port)
	b.WriteString("  mode: debug\n")
	b.WriteString("base:\n")
	b.WriteString("  client_router_enabled: true\n")
	if !adminEnabled {
		b.WriteString("  admin_router_enabled: false\n")
	}
	b.WriteString("database:\n")
	fmt.Fprintf(&b, "  host: %q\n  port: %d\n  user: %q\n  password: %q\n  dbname: %q\n  sslmode: disable\n",
		orDefault(kv["host"], "localhost"), dbPort,
		orDefault(kv["user"], "root"), orDefault(kv["password"], "123456"), dbName)
	b.WriteString("redis:\n  enabled: false\n")
	b.WriteString("migration:\n  enabled: true\n")
	b.WriteString("jwt:\n")
	fmt.Fprintf(&b, "  private_key_file: %q\n  public_key_file: %q\n", privPath, pubPath)
	b.WriteString("  access_token_ttl: \"30m\"\n  refresh_token_ttl: \"168h\"\n")
	b.WriteString("security:\n  aes_key: \"runtime-test-aes-key-32ch!!\"\n")

	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(b.String()), 0o600))
	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	return cfg
}

// waitHealthy 轮询 /health 直到 200（总预算 15s；单请求 2s 超时——无超时的
// http.Get 在 PG 半死态（TCP 通但不响应）下会无限挂起，测试进程假死）。
func waitHealthy(t *testing.T, port int) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(15 * time.Second)
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("服务 15s 内未就绪: %s", url)
}

// TestBootstrapRuntime_TypedRoutesAndFlags 真机全链：
// 自建临时空库 → Bootstrap（迁移自动建表）→ RegisterTypedAuthModule → Run → HTTP 断言。
// 锁定三件单测无法证明的事：
//  1. 空库全新装路径：Bootstrap 内 migration.Run 自动建表，服务正常起
//  2. typed 路由经真实 Run 挂载（public 200 / authed 缺 token → code=100002）
//  3. admin_router_enabled=false：未匹配 GET 返回 404 JSON（不落 SPA 200）、
//     /health 不受影响；Run 后 RegisterTypedAuthModule panic（started 守卫，
//     client_router_enabled=false 的 flag 守卫由 module_test 单测以真实 cfg 结构锁定）
func TestBootstrapRuntime_TypedRoutesAndFlags(t *testing.T) {
	dsn := os.Getenv("NETYADMIN_TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=localhost user=root password=123456 dbname=postgres sslmode=disable"
	}
	kv := parsePGKeywordDSN(t, dsn)
	dbName := createTempTestDB(t, kv)

	// —— admin 开启的实例：全链正向 ——
	port := freePort(t)
	cfg := writeRuntimeConfig(t, kv, dbName, port, true)
	db, err := InitDB(cfg)
	require.NoError(t, err, "InitDB 失败：确认运行时 PG 可达")
	// Cleanup LIFO：此关闭先于 createTempTestDB 注册的 DROP 执行——
	// 连接池不关，DROP 会因「database is being accessed by other users」静默失败、临时库残留
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	application, err := Bootstrap(cfg, db)
	require.NoError(t, err, "Bootstrap 失败：空库全新装路径（迁移自动建表）应可用")
	application.RegisterTypedAuthModule("tech", &recTypedAppModule{name: "tech"}, nil)

	go func() { _ = application.Run() }()
	waitHealthy(t, port)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)

	// typed public：无鉴权直达
	resp, err := http.Get(base + "/client/v1/tech/public/hello")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "typed public 路由应经真实 Run 挂载")

	// typed authed：TypedUserJWTAuth 真实拒绝缺 token。项目契约「全接口 HTTP 200，
	// code 字段区分业务结果」（FailWithCode 恒 200），故断言响应体 code=100002
	//（CodeUnauthorized）而非 HTTP 401；该断言同时证明路由已真实挂载——
	// 未挂载会落入 NoRoute/SPA 兜底，响应体不会有此 code。
	resp, err = http.Get(base + "/client/v1/tech/profile")
	require.NoError(t, err)
	var authBody struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&authBody))
	resp.Body.Close()
	assert.Equal(t, errorx.CodeUnauthorized.String(), authBody.Code,
		"typed authed 路由应挂真实鉴权链（缺 token → code=100002）")

	// 基座 admin 路由面存在（POST login 空 body → 业务响应，非 404）
	resp, err = http.Post(base+"/admin/v1/auth/login", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode, "基座 admin 路由应已注册")

	// —— admin 关闭的实例：NoRoute 行为 + Run 后注册 panic ——
	port2 := freePort(t)
	cfg2 := writeRuntimeConfig(t, kv, dbName, port2, false)
	db2, err := InitDB(cfg2)
	require.NoError(t, err)
	sqlDB2, err := db2.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB2.Close() })
	app2, err := Bootstrap(cfg2, db2)
	require.NoError(t, err)
	go func() { _ = app2.Run() }()
	waitHealthy(t, port2)
	base2 := fmt.Sprintf("http://127.0.0.1:%d", port2)

	resp, err = http.Get(base2 + "/admin/v1/auth/login")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode,
		"admin_router_enabled=false 时未匹配 GET 不得落入 SPA 兜底 200")

	// GET /health 仍然可用（开关不影响健康检查）
	resp, err = http.Get(base2 + "/health")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Run 已启动（a.started=true）→ RegisterTypedAuthModule fail-fast panic
	//（时机守卫；client_router_enabled=false 的 flag 守卫由 module_test 单测锁定）
	assert.Panics(t, func() {
		app2.RegisterTypedAuthModule("tech2", &recTypedAppModule{name: "x"}, nil)
	}, "Run 后调用 RegisterTypedAuthModule 必须 panic")
}
