// unique.go：唯一约束冲突判定收拢（G1-06——users/storage isUnique 与 message
// 内联判定三处重复；23505 错误分类单一实现）。
package database

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// IsUniqueViolation 判定 PG 唯一约束冲突（SQLSTATE 23505；pgconn.PgError 码
// 优先，字符串兜底覆盖驱动层包装路径）。
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
