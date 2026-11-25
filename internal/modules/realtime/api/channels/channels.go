// Package channels 是 WS 频道命名唯一封装（ws-protocol §2——格式
// user:<id>/admin:<id>/app:<appID>:<channel>/topic:<name>；生成函数唯一，
// 禁止字符串拼接；频道名不含 PII）。
package channels

import (
	"strings"
)

// 频道类型前缀（ws-protocol §2）。
const (
	prefixUser  = "user:"
	prefixAdmin = "admin:"
	prefixApp   = "app:"
	prefixTopic = "topic:"
)

// User 构造 C 端用户私有频道（ACL：仅本人）。
func User(id string) string { return prefixUser + id }

// Admin 构造管理员私有频道（ACL：仅本人）。
func Admin(id string) string { return prefixAdmin + id }

// App 构造外部 App 频道（ACL：Scope ws.channel.<pattern>）。
func App(appID, name string) string { return prefixApp + appID + ":" + name }

// Topic 构造公共/业务主题频道（ACL：surface 白名单+可选权限码）。
func Topic(name string) string { return prefixTopic + name }

// Kind 是频道类型分类（ACL 分派面）。
type Kind int8

const (
	KindUnknown Kind = 0
	KindUser    Kind = 1
	KindAdmin   Kind = 2
	KindApp     Kind = 3
	KindTopic   Kind = 4
)

// Classify 判定频道类型并返回主体 ID（user/admin 主体、app 的 appID）。
func Classify(channel string) (kind Kind, subject string) {
	switch {
	case strings.HasPrefix(channel, prefixUser):
		return KindUser, strings.TrimPrefix(channel, prefixUser)
	case strings.HasPrefix(channel, prefixAdmin):
		return KindAdmin, strings.TrimPrefix(channel, prefixAdmin)
	case strings.HasPrefix(channel, prefixApp):
		rest := strings.TrimPrefix(channel, prefixApp)
		appID, _, _ := strings.Cut(rest, ":")
		return KindApp, appID
	case strings.HasPrefix(channel, prefixTopic):
		return KindTopic, strings.TrimPrefix(channel, prefixTopic)
	default:
		return KindUnknown, ""
	}
}

// Valid 校验频道格式（长度 ≤128 + 前缀合法——ws-protocol §2）。
// user/admin 主体段=不可变 ID（ULID 含大写——不限字符集，仅非空）；
// topic name 段只允许小写字母/数字/-/_/.；app 为 <appID>:<name>（appID 含大写
// 同 user 面，name 段小写集）。
func Valid(channel string) bool {
	if len(channel) == 0 || len(channel) > 128 {
		return false
	}
	kind, subject := Classify(channel)
	switch kind {
	case KindUser, KindAdmin:
		return subject != ""
	case KindApp:
		// Classify 的 subject 只含 appID（ACL 身份校验面用），name 段须从原始
		// channel 取：app:<appID>:<name>（appID 非空，name 走主题字符集）。
		appID, name, ok := strings.Cut(strings.TrimPrefix(channel, prefixApp), ":")
		return ok && appID != "" && name != "" && topicNameValid(name)
	case KindTopic:
		return subject != "" && topicNameValid(subject)
	default:
		return false
	}
}

// topicNameValid 校验主题名段字符集（小写字母/数字/-/_/.——§2）。
func topicNameValid(name string) bool {
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}
