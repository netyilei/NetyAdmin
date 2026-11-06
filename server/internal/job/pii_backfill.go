package job

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"NetyAdmin/internal/pkg/pii"
	"NetyAdmin/internal/pkg/recovery"
)

// piiBackfillBatchSize 是惰性加密迁移的批大小：单批行数小、事务短，
// 避免长事务锁表；users/admin_user 基座行数量级小，一轮即可完成。
const piiBackfillBatchSize = 500

// piiColumnSet 描述一张表上的一组 PII 列（手机号或邮箱）的惰性迁移定义。
type piiColumnSet struct {
	table string // 表名
	idCol string // 主键列（users=ulid string，admin_user=uint，统一按文本扫描、原值回填）
	plain string // 废弃明文列
	enc   string // 密文列
	idx   string // 盲索引列
	tail  string // 尾号辅助列（手机号有、邮箱无；空串=不写该列）
	seal  func(v string) (enc, idx, tail string, err error)
}

// RunPIIPhoneBackfill 启动后台惰性迁移：把存量明文手机号/邮箱加密写入
// 密文/盲索引（/尾号）列后置明文列 NULL（0059/0060）。
//
// 为什么是后台任务而不是 SQL 迁移：加密需要应用侧密钥（HKDF 派生自
// [security].aes_key），基座迁移是纯 SQL 无法调用。任务幂等且多节点安全：
//   - 只处理明文非空且密文为空的行，UPDATE 带 WHERE 守卫，
//     两节点处理同一行时后到者 WHERE 不匹配（0 行，无害）；
//   - users 的 phone_idx/email_idx 唯一索引兜底并发注册；
//     重复加密同值产同索引。
//
// 失败容忍：单行失败记日志继续；整体失败下次启动自动重跑（幂等）。
//
// 在 Bootstrap 内以 GoSafe 启动（不进任务表——这是基座基建自愈动作，
// 不是业务任务；下游无感知）。明文列在任务完成后全库为 NULL。
func RunPIIPhoneBackfill(ctx context.Context, db *gorm.DB, c *pii.Cipher) {
	recovery.GoSafe("pii_backfill", func() {
		bg := context.Background()
		start := time.Now()

		sets := []piiColumnSet{
			{table: "users", idCol: "id", plain: "phone", enc: "phone_enc", idx: "phone_idx", tail: "phone_tail4",
				seal: func(v string) (string, string, string, error) { e, i, t, err := c.Seal(v); return e, i, t, err }},
			{table: "users", idCol: "id", plain: "email", enc: "email_enc", idx: "email_idx", tail: "",
				seal: func(v string) (string, string, string, error) { e, i, err := c.SealEmail(v); return e, i, "", err }},
			{table: "admin_user", idCol: "id", plain: "phone", enc: "phone_enc", idx: "phone_idx", tail: "phone_tail4",
				seal: func(v string) (string, string, string, error) { e, i, t, err := c.Seal(v); return e, i, t, err }},
			{table: "admin_user", idCol: "id", plain: "email", enc: "email_enc", idx: "email_idx", tail: "",
				seal: func(v string) (string, string, string, error) { e, i, err := c.SealEmail(v); return e, i, "", err }},
		}

		counts := map[string]int{}
		for _, set := range sets {
			n := backfillColumn(bg, db, set)
			counts[set.table+"."+set.plain] = n
		}

		total := 0
		for _, n := range counts {
			total += n
		}
		if total > 0 {
			slog.Info("pii backfill completed",
				"counts", counts, "elapsed", time.Since(start).String())
		}
	})
}

// backfillColumn 迁移单表单列组：循环分批取「明文非空且密文为空」的行，
// 逐行加密写回（WHERE 守卫保证多节点幂等），返回迁移行数。
func backfillColumn(bg context.Context, db *gorm.DB, set piiColumnSet) int {
	migrated := 0
	for {
		// 主键统一 ::text 扫描（users=varchar / admin_user=int4 类型不同，
		// GORM Scan 不支持 interface{} 目标字段——泛型化尝试已实测失败）
		rows := []struct {
			ID    string
			Value string
		}{}
		if err := db.WithContext(bg).
			Table(set.table).
			Where(set.plain + " IS NOT NULL AND " + set.plain + " <> '' AND (" + set.enc + " IS NULL OR " + set.enc + " = '')").
			Select(set.idCol + "::text AS id, " + set.plain + " AS value").
			Limit(piiBackfillBatchSize).
			Scan(&rows).Error; err != nil {
			slog.Error("pii backfill: scan failed, retry on next boot",
				"table", set.table, "col", set.plain, "err", err)
			return migrated
		}
		if len(rows) == 0 {
			return migrated
		}
		for _, r := range rows {
			enc, idx, tail, err := set.seal(r.Value)
			if err != nil {
				slog.Error("pii backfill: seal failed, skip row",
					"table", set.table, "id", r.ID, "err", err)
				continue
			}
			updates := map[string]interface{}{
				set.enc:   enc,
				set.idx:   idx,
				set.plain: "",
			}
			if set.tail != "" {
				updates[set.tail] = tail
			}
			res := db.WithContext(bg).Table(set.table).
				Where(set.idCol+"::text = ? AND ("+set.enc+" IS NULL OR "+set.enc+" = '')", r.ID).
				Updates(updates)
			if res.Error != nil {
				slog.Error("pii backfill: update failed, skip row",
					"table", set.table, "id", r.ID, "err", res.Error)
				continue
			}
			migrated += int(res.RowsAffected)
		}
		if len(rows) < piiBackfillBatchSize {
			return migrated
		}
	}
}
