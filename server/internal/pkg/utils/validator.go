package utils

import "regexp"

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// IsEmail 校验是否为合法邮箱
func IsEmail(target string) bool {
	return emailRegex.MatchString(target)
}
