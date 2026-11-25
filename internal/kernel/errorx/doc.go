//go:generate node ../../../scripts/gen/status-codes.mjs

// Package errorx 提供 5 位状态码错误模型与统一响应信封。
//
// 形态（对齐 docs/rules/status-codes.md §8 权威定义）：
//   - Error：code（codes_gen.go 生成常量）+ message_key（元数据派生，i18n 渲染）+
//     details（结构化上下文）+ cause 链；无文案字段，无独立 BizCode（5 位码即业务码）；
//   - Envelope：{code, message_key, data, details, request_id, trace_id}，api-style §4；
//   - 未登记码构造时 panic——手写数字在 CI 扫描之外的运行期兜底。
//
// 分层：domain 不 import 本包；application 负责 domain 错误 → errorx 映射。
// 再生成：go generate ./... 或 node scripts/gen/status-codes.mjs（--check 进 CI 门禁）。
package errorx
