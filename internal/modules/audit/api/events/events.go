// Package events 是 audit 的事件 topic 唯一封装（§5.5：domain.<context>.<entity>.<event>）。
// audit 只归属存储不发布事件；订阅面随消费方（如告警联动）扩展时登记。
package events
