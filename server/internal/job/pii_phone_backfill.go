package job

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"NetyAdmin/internal/pkg/pii"
	"NetyAdmin/internal/pkg/recovery"
)

// piiPhoneBackfillBatchSize 是惰性加密迁移的批大小：单批行数小、事务短，
// 避免长事务锁表；users/admin_user 基座行数量级小，一轮即可完成。
const piiPhoneBackfillBatchSize = 500

// RunPIIPhoneBackfill 启动后台惰性迁移：把存量明文手机号加密写入
// (phone_enc, phone_idx, phone_tail4) 三列后置明文列 NULL（0059）。
//
// 为什么是后台任务而不是 SQL 迁移：加密需要应用侧密钥（HKDF 派生自
// [security].aes_key），基座迁移是纯 SQL 无法调用。任务幂等且多节点安全：
//   - 只处理 phone 非空且 phone_enc 为空的行，UPDATE 带 WHERE 守卫，
//     两节点处理同一行时后到者 WHERE 不匹配（0 行，无害）；
//   - users.phone_idx 唯一索引兜底并发注册；重复加密同号产同 idx。
//
// 失败容忍：单行失败记日志继续；整体失败下次启动自动重跑（幂等）。
//
// 在 Bootstrap 内以 GoSafe 启动（不进任务表——这是基座基建自愈动作，
// 不是业务任务；下游无感知）。phone 明文列在任务完成后全库为 NULL。
func RunPIIPhoneBackfill(ctx context.Context, db *gorm.DB, c *pii.Cipher) {
	recovery.GoSafe("pii_phone_backfill", func() {
		bg := context.Background()
		start := time.Now()
		var usersMigrated, adminsMigrated int

		for {
			type row struct {
				ID    string
				Phone string
			}
			var rows []row
			if err := db.WithContext(bg).
				Table("users").
				Where("deleted_at = 0 AND phone IS NOT NULL AND phone <> '' AND (phone_enc IS NULL OR phone_enc = '')").
				Select("id", "phone").Limit(piiPhoneBackfillBatchSize).
				Scan(&rows).Error; err != nil {
				slog.Error("pii phone backfill: scan users failed, retry on next boot", "err", err)
				return
			}
			if len(rows) == 0 {
				break
			}
			for _, r := range rows {
				enc, idx, tail4, err := c.Seal(r.Phone)
				if err != nil {
					slog.Error("pii phone backfill: seal users failed, skip row", "userID", r.ID, "err", err)
					continue
				}
				res := db.WithContext(bg).Exec(
					`UPDATE users SET phone_enc = ?, phone_idx = ?, phone_tail4 = ?, phone = ''
					 WHERE id = ? AND (phone_enc IS NULL OR phone_enc = '')`,
					enc, idx, tail4, r.ID)
				if res.Error != nil {
					slog.Error("pii phone backfill: update users failed, skip row", "userID", r.ID, "err", res.Error)
					continue
				}
				usersMigrated += int(res.RowsAffected)
			}
			if len(rows) < piiPhoneBackfillBatchSize {
				break
			}
		}

		// admin_user 同构处理（无唯一索引，重复号共存，索引不冲突）
		for {
			type row struct {
				ID    uint
				Phone string
			}
			var rows []row
			if err := db.WithContext(bg).
				Table("admin_user").
				Where("deleted_at = 0 AND phone IS NOT NULL AND phone <> '' AND (phone_enc IS NULL OR phone_enc = '')").
				Select("id", "phone").Limit(piiPhoneBackfillBatchSize).
				Scan(&rows).Error; err != nil {
				slog.Error("pii phone backfill: scan admin_user failed, retry on next boot", "err", err)
				return
			}
			if len(rows) == 0 {
				break
			}
			for _, r := range rows {
				enc, idx, tail4, err := c.Seal(r.Phone)
				if err != nil {
					slog.Error("pii phone backfill: seal admin_user failed, skip row", "adminID", r.ID, "err", err)
					continue
				}
				res := db.WithContext(bg).Exec(
					`UPDATE admin_user SET phone_enc = ?, phone_idx = ?, phone_tail4 = ?, phone = ''
					 WHERE id = ? AND (phone_enc IS NULL OR phone_enc = '')`,
					enc, idx, tail4, r.ID)
				if res.Error != nil {
					slog.Error("pii phone backfill: update admin_user failed, skip row", "adminID", r.ID, "err", res.Error)
					continue
				}
				adminsMigrated += int(res.RowsAffected)
			}
			if len(rows) < piiPhoneBackfillBatchSize {
				break
			}
		}

		if usersMigrated > 0 || adminsMigrated > 0 {
			slog.Info("pii phone backfill completed",
				"users", usersMigrated, "admins", adminsMigrated, "elapsed", time.Since(start).String())
		}
	})
}
