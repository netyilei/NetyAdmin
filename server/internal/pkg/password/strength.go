package password

import (
	"fmt"
	"regexp"
)

// 预编译字符类别正则（包级复用，避免每次调用即时编译）。
// 与 pkg/jwt 中 secret 强度校验共享同一套判定标准。
var (
	reLower   = regexp.MustCompile(`[a-z]`)
	reUpper   = regexp.MustCompile(`[A-Z]`)
	reDigit   = regexp.MustCompile(`[0-9]`)
	reSpecial = regexp.MustCompile(`[^a-zA-Z0-9]`)
)

// StrengthConfig 是密码强度校验的参数化配置。
//
// 抽取自原 user.go（配置驱动）与 admin.go（硬编码 3 类）两套实现，
// 消除跨端复制粘贴 + 即时编译正则（重构清单 B-AUTH-6）。
type StrengthConfig struct {
	MinLength    int // 最小长度，0 表示不校验
	RequireTypes int // 必须包含的字符类别数（1-4：小写/大写/数字/特殊符号）
}

// DefaultUserStrengthConfig 用户端默认强度配置（向后兼容原 service/user 的默认值）。
// 可被 configWatcher 配置覆盖。
var DefaultUserStrengthConfig = StrengthConfig{
	MinLength:    8,
	RequireTypes: 2,
}

// DefaultAdminStrengthConfig 管理员端默认强度配置（向后兼容原 service/system 的硬编码 3 类）。
var DefaultAdminStrengthConfig = StrengthConfig{
	MinLength:    0, // admin 原实现无长度门槛
	RequireTypes: 3,
}

// CountCharTypes 统计密码中包含的字符类别数（小写/大写/数字/特殊符号）。
func CountCharTypes(pwd string) int {
	types := 0
	if reLower.MatchString(pwd) {
		types++
	}
	if reUpper.MatchString(pwd) {
		types++
	}
	if reDigit.MatchString(pwd) {
		types++
	}
	if reSpecial.MatchString(pwd) {
		types++
	}
	return types
}

// ValidateStrength 按给定配置校验密码强度。
//
// 返回 nil 表示通过，否则返回描述具体不足的 error（调用方包装为 errorx.CodePasswordTooWeak）。
// 是 service/user.validatePasswordStrength 与 service/system.validateAdminPasswordStrength 的统一替代。
func ValidateStrength(pwd string, cfg StrengthConfig) error {
	if cfg.MinLength > 0 && len(pwd) < cfg.MinLength {
		return fmt.Errorf("密码长度不足，至少需要 %d 字符", cfg.MinLength)
	}
	if cfg.RequireTypes > 0 {
		actual := CountCharTypes(pwd)
		if actual < cfg.RequireTypes {
			return fmt.Errorf("密码必须包含大小写字母、数字、特殊符号中的至少 %d 种", cfg.RequireTypes)
		}
	}
	return nil
}
