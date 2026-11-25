// Package keys 是 note 的 Redis/缓存/锁/限流/幂等 key 唯一封装。
//
// 铁律（docs/rules/namespaces.md）：业务代码禁止裸 key 字符串，一律 import 本包。
// 格式：<root>:<env>:[tenant:]note:<feature>:<id>[:<sub>]；key 必须带 TTL；禁 PII 进 key。
// 数据 key 不跨模块读（走 Port）；共享基础设施 key 须在 module.yaml 声明 shared_infra_keys。
// builder/常量由本模块按需添加；快照测试见 keys_test.go。
package keys
