package primitives

import (
	"sort"
	"sync"
)

// —— 掩码注册机制（security.md §3；G1-15 需求审计 + 用户 2026-11-14） ——
//
// 基座只提供机制：kind → 掩码函数的注册表与统一出口，**零预设**——
// phone/email/身份证/银行卡都是使用方的字段，由使用方（含演示场景）自行注册。
// 不需要 PII 处理的使用方一个都不注册，机制零介入（无 goroutine/无扫描/无开销）。
// 消费链：使用方组合 Mask 构建 Redact 函数，注入 log.Manager Options.Redact
// （出口统一脱敏）；users 演示模块为首个示范注册/组合者。
//
// fail-closed：未注册的 kind 被调用时全文遮蔽（宁过度脱敏，不漏明文）。

type MaskFunc func(value string) string

type maskEntry struct {
	fn    MaskFunc
	owner string
}

var (
	maskMu   sync.RWMutex
	maskRegs = map[string]maskEntry{}
)

// RegisterMasker 注册/覆盖某类 PII 的掩码函数（后注册覆盖先前——下游可覆盖基座预设）。
// owner 用于诊断（RegisteredMaskKinds 展示归属），传模块名或 "base"；空 kind/nil fn 忽略。
func RegisterMasker(kind string, fn MaskFunc, owner string) {
	if kind == "" || fn == nil {
		return
	}
	maskMu.Lock()
	defer maskMu.Unlock()
	maskRegs[kind] = maskEntry{fn: fn, owner: owner}
}

// Mask 按 kind 分发掩码；未注册的 kind 全文遮蔽（fail-closed）。
func Mask(kind, value string) string {
	maskMu.RLock()
	entry, ok := maskRegs[kind]
	maskMu.RUnlock()
	if !ok {
		return "[redacted]"
	}
	return entry.fn(value)
}

// RegisteredMaskKinds 返回已注册 kind → 归属（诊断端点用， 接入）。
func RegisteredMaskKinds() map[string]string {
	maskMu.RLock()
	defer maskMu.RUnlock()
	out := make(map[string]string, len(maskRegs))
	for k, e := range maskRegs {
		out[k] = e.owner
	}
	return out
}

// MaskKindNames 返回排序后的 kind 名列表（测试与诊断便利）。
func MaskKindNames() []string {
	maskMu.RLock()
	defer maskMu.RUnlock()
	out := make([]string, 0, len(maskRegs))
	for k := range maskRegs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// —— 公共掩码实现（机制零预设不变：模块仍须显式 RegisterMasker 才生效；
// 这里只收拢多个模块共用的字节级实现——G1-06 第二次即收拢） ——

// MaskEmail 是邮箱掩码公共实现（保留首字符+域名：someone@example.com → s***@example.com）。
func MaskEmail(v string) string {
	at := -1
	for i, r := range v {
		if r == '@' {
			at = i
			break
		}
	}
	if at <= 0 {
		return "***"
	}
	local := v[:at]
	return string(local[0]) + "***" + v[at:]
}

// MaskPhone 是手机号掩码公共实现（保留前三后四：13812345678 → 138****5678）。
func MaskPhone(v string) string {
	if len(v) < 4 {
		return "***"
	}
	return v[:3] + "****" + v[len(v)-4:]
}
