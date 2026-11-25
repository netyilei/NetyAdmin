// render.go：模板占位符渲染（send 面）。拆分自 service.go（R7）。
package application

import (
	"regexp"
	"strings"
)

// varPattern 是模板占位符 {{key}}。
var varPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.]+)\s*\}\}`)

// render 渲染模板占位符（{{key}}/{{ key }} → vars[key]；未提供=空串）。
// key TrimSpace——正则允许空格但查表必须归一（联测审计 A1：`{{ name }}`
// 渲染空串——外发正文/主题静默缺失）。
func render(tpl string, vars map[string]string) string {
	return varPattern.ReplaceAllStringFunc(tpl, func(m string) string {
		key := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(m, "{{"), "}}"))
		return vars[key]
	})
}
