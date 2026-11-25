// Package cache 实现 Cache 平面：L1（BigCache）/ L2（Redis）/ L3（DB）。
// 类型化 Entity + class 准入（hot_local 门槛与自动降级）、generation 失效、
// singleflight、负缓存、键命名空间（na:{env}:cache 装配注入）。
// PII（classification=pii）L2 值写前 AES-256-GCM 加密（contracts cache.pii.encrypt_l2；
// 密钥 env-only NETYADMIN_CACHE_L2_KEY 装配注入，缺失=pii 实体注册失败 fail-closed）。
// 规范见 docs/rules/state-and-cache.md。
package cache
