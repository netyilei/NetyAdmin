// Package domain 是 note 领域层：实体、值对象、领域事件、仓储接口、领域错误。
//
// 依赖禁令（CI check-imports 强制）：禁 import gin/gorm/go-redis/slog、
// kernel/errorx、任何 api/keys、任何 adapters。
// 领域错误用 errors.New 定义在本包，由 application 映射到接口状态码（kernel/errorx）。
// 注意：doc 注释行不要用行首空格缩进（gofmt 会重排）。
package domain
