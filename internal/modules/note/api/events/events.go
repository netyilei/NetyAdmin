// Package events 是 note 的领域/平台事件 Topic 常量唯一封装。
//
// 命名：domain.note.<aggregate>.<action>；SchemaVer 只加字段；
// 事务内发布用 PublishAfterCommit（Outbox）；消费方必须幂等。
// 骨架示例：首个领域事件时按本包格式添加 Topic 常量。
package events
