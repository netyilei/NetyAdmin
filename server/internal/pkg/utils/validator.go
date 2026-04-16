package utils

import "regexp"

// IsEmail 校验是否为合法邮箱
func IsEmail(target string) bool {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	reg := regexp.MustCompile(pattern)
	return reg.MatchString(target)
}

// IsPhone 校验是否为合法手机号 (简化版，适配多国)
func IsPhone(target string) bool {
	pattern := `^\+?[1-9]\d{1,14}$`
	reg := regexp.MustCompile(pattern)
	return reg.MatchString(target)
}
