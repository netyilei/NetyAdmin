package primitives

import (
	"fmt"
	"strings"
)

// 掩码定位（G1-15 需求审计 2026-11-14）：基座功能清单（identity 登录/资料/改密、
// content/storage/message、审计导出）无证件号/银行卡等字段，掩码不在本包预建；
// 日志/导出的 PII 脱敏由 log.Manager 出口按 security.md §3 禁明文清单统一实现。
// 下游业务字段出现时由下游自建专用掩码。

// —— 分页规整（api-style §3：page 1 起；默认 20 上限 100） ——

const (
	pageDefaultSize = 20
	pageMaxSize     = 100
)

// Page 是规整后的分页参数。
type Page struct {
	Number int // 1 起
	Size   int // 1..100
	Offset int // = (Number-1)*Size，SQL 直接可用
}

// NormalizePage 规整分页参数（非法值取默认/钳位，不报错——列表查询的容错边界）。
func NormalizePage(page, size int) Page {
	if page < 1 || page > 1_000_000 { // 上限防 int 溢出（Offset=页×大小）
		page = 1
	}
	if size < 1 {
		size = pageDefaultSize
	}
	if size > pageMaxSize {
		size = pageMaxSize
	}
	return Page{Number: page, Size: size, Offset: (page - 1) * size}
}

// —— LIKE 转义与排序白名单（data.md/：防 LIKE 通配注入与无界排序） ——

// EscapeLike 转义 LIKE 通配符（用户输入进 LIKE 模式前的必经步骤）。
func EscapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// ParseSort 解析 api-style §3 契约格式 "sort=<field>:asc|desc"（不含 sort= 前缀），
// 校验字段在白名单内；返回归一化字段名与降序标志。
func ParseSort(sortParam string, whitelist []string) (field string, desc bool, err error) {
	f, dir, found := strings.Cut(sortParam, ":")
	if !found || (dir != "asc" && dir != "desc") {
		return "", false, fmt.Errorf("primitives: 排序参数 %q 应为 <field>:asc|desc", sortParam)
	}
	for _, w := range whitelist {
		if f == w {
			return f, dir == "desc", nil
		}
	}
	return "", false, fmt.Errorf("primitives: 排序字段 %q 不在白名单", f)
}
