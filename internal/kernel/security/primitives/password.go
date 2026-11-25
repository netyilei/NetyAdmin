package primitives

import "golang.org/x/crypto/bcrypt"

// bcryptDefaultCost 是密码哈希默认成本（security.md §2：bcrypt/argon2；argon2 随按需引入）。
const bcryptDefaultCost = 12

// bcryptMinCost 是允许的最低成本（<10 的哈希在 Modern hardware 下可暴力破解——
// 调用方传低值一律抬到默认，不给弱哈希入口）。
const bcryptMinCost = 10

// HashPassword 用 bcrypt 哈希口令（cost < bcryptMinCost 时抬到默认成本）。
// 注意：bcrypt 只取口令前 72 字节——超长口令登录面应限制长度。
func HashPassword(password string, cost int) (string, error) {
	if cost < bcryptMinCost {
		cost = bcryptDefaultCost
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// ComparePassword 校验口令与哈希是否匹配（恒定时间比较，由 bcrypt 内部保证）。
func ComparePassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
