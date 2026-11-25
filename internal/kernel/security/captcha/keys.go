// keys.go：验证码 Redis 键构造唯一入口。
// 契约登记：contracts/namespaces.yaml kernel_plane_keys.captcha
// = "<root>:<env>:captcha:<id>"。生产写侧/e2e 读侧一律经 AnswerKey 构造，
// 冒烟脚本（JS）按契约拼——禁止任何一侧本地字符串拼接（根修：
// 键失配根因=无单一引用源，写侧 namespace 注入后 e2e 独立拼法即失配）。
package captcha

// AnswerKey 构造验证码答案键（namespace=<prefix>:<env>:captcha——装配期
// 经 captchaNamespace 注入；id=生成侧返回的 captchaId）。
func AnswerKey(namespace, id string) string {
	return namespace + ":" + id
}
