package utils

import "regexp"

var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	phoneRegex = regexp.MustCompile(`^\+?[1-9]\d{1,14}$`)
)

// IsEmail 校验是否为合法邮箱
func IsEmail(target string) bool {
	return emailRegex.MatchString(target)
}

// IsPhone 校验是否为合法手机号 (简化版，适配多国)
func IsPhone(target string) bool {
	return phoneRegex.MatchString(target)
}
