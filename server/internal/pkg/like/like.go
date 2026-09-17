package like

import "strings"

// likeReplacer escapes LIKE pattern wildcards and the escape character itself.
// PostgreSQL treats backslash as the default LIKE escape character, so escaped
// values match literally without an explicit ESCAPE clause.
var likeReplacer = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLike 转义 LIKE 模式通配符（% _ \），使输入按字面量匹配。
//
// 参数化查询本身无注入风险，但不转义时用户输入 % / _ 会改变匹配语义
// （如输入 % 匹配全部记录、无法搜索字面含 %_ 的内容）。
func EscapeLike(s string) string {
	return likeReplacer.Replace(s)
}

// LikeContains 构造"包含"语义的 LIKE 模式（%input%），输入先经 EscapeLike 转义。
func LikeContains(s string) string {
	return "%" + EscapeLike(s) + "%"
}
