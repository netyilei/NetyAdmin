// ensure_apps.go：内置面应用自举（——client 面调用方作为开放平台
// 应用接受 IP/频率管控；不存在则创建，secret 逐应用随机 AES 加密入库不外显。
// 对照 identity.EnsureBootstrapAdmin 惯例——包级函数 pool 直调）。
// 用户终版：admin 端 API/WS 默认开放（JWT+RBAC），不纳入
// 应用限制面——admin-console 应用删除，应用载体仅剩 client 面 frontend-app。
package application

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/gw/domain"
)

// 内置面应用 appKey（应用载体唯一=client 面；admin 面无应用——无固定常量）。
const AppKeyFrontendApp = "frontend-app" // 前端小程序（/client/v1）

// EnsureGatewayApps 确保内置面应用存在（幂等；pool 直调——对照
// identity.EnsureBootstrapAdmin 惯例）；返回新建 appKey 清单（运维感知——
// secret 逐应用随机 AES 加密入库不外显，重置走管理面重置密钥端点）。
// 调用面=cmd migrate 子命令（独立进程，无运行中快照可失效）——服务进程的
// edge 判定快照由 AttachDB 初始装载/写路径 Reload 收敛，本函数无需广播。
func EnsureGatewayApps(ctx context.Context, pool *database.Pool, aesKey []byte) ([]string, error) {
	var created []string
	for _, appKey := range []string{AppKeyFrontendApp} {
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM gw_apps WHERE app_key=$1`, appKey).Scan(&count); err != nil {
			return nil, fmt.Errorf("gw: builtin app check %s: %w", appKey, err)
		}
		if count > 0 {
			continue // 已存在
		}
		enc, err := primitives.EncryptAESGCM(aesKey, []byte(randomSecret(32)))
		if err != nil {
			return nil, fmt.Errorf("gw: encrypt builtin secret %s: %w", appKey, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO gw_apps (id, app_key, app_secret_enc, name, status,
				rate_limit_enabled, quota_config, remark, storage_ref, ip_group_ids)
			VALUES ($1,$2,$3,$4,$5,true,$6,$7,'',$8)`,
			"gw-app-"+appKey, appKey, enc, "内置面应用 "+appKey, int16(domain.StatusEnabled),
			`{"rate":0}`, "内置面应用", []string{}); err != nil {
			if database.IsUniqueViolation(err) {
				continue // 并发自举：他人已建
			}
			return nil, fmt.Errorf("gw: insert builtin app %s: %w", appKey, err)
		}
		created = append(created, appKey)
	}
	return created, nil
}

func randomSecret(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	const charset = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = charset[v>>4]
		out[i*2+1] = charset[v&0x0f]
	}
	return string(out)
}
