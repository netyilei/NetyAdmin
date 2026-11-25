// bind.go：DTO 解析与校验前置（TR-01；api-style §3「校验前置」+ §4 错误信封）。
// 解析失败 → 10001（invalid json）；validator 校验失败 → 10002（details 带字段级
// 错误：field→tag，结构化上下文，禁止回显原始错误文案）。
package transport

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
)

// dtoValidate 是进程级共享 validator（无状态；唯一实例约定）。
var dtoValidate = validator.New()

// Bind 解析 JSON body 并做 DTO 校验；失败时渲染统一信封并返回 false
// （调用方直接 return）。dst 必须是指针。
func Bind(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		Fail(c, errorx.New(errorx.CodeInvalidParams, "reason", "invalid json body"))
		return false
	}
	if err := dtoValidate.Struct(dst); err != nil {
		fields := map[string]string{}
		var ves validator.ValidationErrors
		if errors.As(err, &ves) {
			for _, fe := range ves {
				fields[fe.Field()] = fe.Tag() // field→违反规则（如 required/gt），不外泄输入值
			}
		}
		Fail(c, errorx.New(errorx.CodeValidationFailed, "fields", fields))
		return false
	}
	return true
}
