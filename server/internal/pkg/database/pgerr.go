package database

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// IsUniqueViolation 判断是否为 PostgreSQL 唯一约束冲突（SQLSTATE 23505）。
//
// 用于 service 层把并发写竞态下的 DB 兜底（部分唯一索引/唯一约束）转换为
// 友好的业务错误码：前置 ExistsBy 检查只是加速路径，两个并发请求可能
// 双双通过检查，最终由 DB 约束裁决。
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
