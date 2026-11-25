// Package primitives 提供安全与通用原语：JWT（RS256）、password（bcrypt）、
// HMAC/nonce、ULID/Clock、分页规整、LIKE 转义、排序白名单校验、掩码注册机制。
//
// 定位（架构设计全景表行 34 / security.md）：只提供原语与机制，不绑定策略——
// AuthN/RateLimit/IPAC 策略在 kernel/security 其他子包（..05）。
// 分层：domain 禁止 import 本包（domain 允许面=标准库+领域内；ID 由
// application 生成后传入聚合）。
//
// 范围（G1-15 需求审计 2026-11-14 + 用户扩展点形态，逐项对到消费者）：
//
//	保留：password→ID-01 · ULID/Clock→/BUS/SCH · HMAC/nonce→/ ·
//	      分页/LIKE/排序→ · JWT→；
//	机制：掩码注册表（RegisterMasker/Mask）——零预设，phone/email/身份证/银行卡
//	      等全部由使用方自行注册（演示场景可注册看效果；不需要 PII 可完全不注册，
//	      机制零介入）；
//	不建：PII 加密/盲索引的具体实现（基座无 pii 级字段；下游需要时自建并可
//	      参考本包 git 历史）、证书/密钥加载。
package primitives
