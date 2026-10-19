package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// DeleteBatchBefore 分批删除 created_at 早于 before 的行，直到删尽。
//
// 一次性大范围 DELETE 会产生长事务与锁压力（保留期首次生效时最明显），
// 按 id 子查询限批循环将压力摊平。返回累计删除行数。
// 表必须含 id 与 created_at 列（基座日志表均满足）。
func DeleteBatchBefore(ctx context.Context, db *gorm.DB, table string, before time.Time, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 1000
	}
	var total int64
	for {
		res := db.WithContext(ctx).Exec(
			fmt.Sprintf("DELETE FROM %s WHERE id IN (SELECT id FROM %s WHERE created_at < ? LIMIT ?)", table, table),
			before, batchSize,
		)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < int64(batchSize) {
			return total, nil
		}
	}
}
