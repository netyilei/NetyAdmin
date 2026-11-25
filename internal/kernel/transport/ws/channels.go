// channels.go：Hub 频道订阅面（WS 频道——按频道定向推送）。
// 分工：kernel 只承载「频道→连接」订阅注册表与字节帧投递（协议语义/ACL/
// 信封在 realtime 模块——kernel 不定义协议常量， 注记约束）；订阅校验
// （频道权限）由调用方在 Subscribe 前完成（realtime ACL 面）。
package ws

import (
	"errors"
	"fmt"
	"sync"
)

// ErrNotSubscribed 是退订不存在的订阅。
var ErrNotSubscribed = errors.New("ws: not subscribed")

// Channels 是频道订阅注册表（Hub 内嵌；conn 注销时须同步清理订阅）。
type Channels struct {
	mu sync.RWMutex
	// byChannel: channel → connID 集
	byChannel map[string]map[string]struct{}
	// byConn: connID → channel 集（注销清理用）
	byConn map[string]map[string]struct{}
}

// NewChannels 创建。
func NewChannels() *Channels {
	return &Channels{
		byChannel: map[string]map[string]struct{}{},
		byConn:    map[string]map[string]struct{}{},
	}
}

// Subscribe 订阅频道（幂等）。
func (c *Channels) Subscribe(connID, channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byChannel[channel] == nil {
		c.byChannel[channel] = map[string]struct{}{}
	}
	c.byChannel[channel][connID] = struct{}{}
	if c.byConn[connID] == nil {
		c.byConn[connID] = map[string]struct{}{}
	}
	c.byConn[connID][channel] = struct{}{}
}

// Unsubscribe 退订（不存在 → ErrNotSubscribed）。
func (c *Channels) Unsubscribe(connID, channel string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	subs, ok := c.byChannel[channel]
	if !ok {
		return ErrNotSubscribed
	}
	if _, ok := subs[connID]; !ok {
		return ErrNotSubscribed
	}
	delete(subs, connID)
	if len(subs) == 0 {
		delete(c.byChannel, channel)
	}
	if chans := c.byConn[connID]; chans != nil {
		delete(chans, channel)
		if len(chans) == 0 {
			delete(c.byConn, connID)
		}
	}
	return nil
}

// DropConn 清理连接的全部订阅（Hub.Unregister 时调用——防幽灵订阅）。
func (c *Channels) DropConn(connID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for channel := range c.byConn[connID] {
		delete(c.byChannel[channel], connID)
		if len(c.byChannel[channel]) == 0 {
			delete(c.byChannel, channel)
		}
	}
	delete(c.byConn, connID)
}

// Subscribers 是频道订阅者 connID 快照（投递用——锁外发送防持锁写）。
func (c *Channels) Subscribers(channel string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	subs := c.byChannel[channel]
	if len(subs) == 0 {
		return nil
	}
	out := make([]string, 0, len(subs))
	for id := range subs {
		out = append(out, id)
	}
	return out
}

// ConnChannels 是连接订阅频道快照（诊断/清理面）。
func (c *Channels) ConnChannels(connID string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	chans := c.byConn[connID]
	out := make([]string, 0, len(chans))
	for ch := range chans {
		out = append(out, ch)
	}
	return out
}

// Count 是频道订阅总数（诊断面）。
func (c *Channels) Count() (channels int, subscriptions int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, subs := range c.byChannel {
		subscriptions += len(subs)
	}
	return len(c.byChannel), subscriptions
}

// —— Hub 频道面（Hub 内嵌 Channels，直接代理）——

// PublishChannel 按频道定向投递（订阅者逐一 Send；单连接满/关闭计数跳过——
// 背压决策归调用方：realtime 层按 ErrSendBlocked 计数/慢速踢出）。
// 返回实际入队数与失败数。
func (h *Hub) PublishChannel(frame []byte, channel string) (sent, failed int) {
	for _, connID := range h.channels.Subscribers(channel) {
		conn, ok := h.Get(connID)
		if !ok {
			continue // 竞态：注销中（DropConn 前窗口）
		}
		if err := conn.Send(frame); err != nil {
			failed++
			continue
		}
		sent++
	}
	return sent, failed
}

// PublishToPrincipal 按身份向全部连接投递（同一身份多端——§6 同身份上限由
// surface 决策）。
func (h *Hub) PublishToPrincipal(frame []byte, principalID string) (sent, failed int) {
	for _, conn := range h.ByPrincipal(principalID) {
		if err := conn.Send(frame); err != nil {
			failed++
			continue
		}
		sent++
	}
	return sent, failed
}

// Subscribe 代理（频道权限校验在 realtime ACL 层完成后再调）。
func (h *Hub) Subscribe(connID, channel string) error {
	if _, ok := h.Get(connID); !ok {
		return fmt.Errorf("ws: conn %s not registered", connID)
	}
	h.channels.Subscribe(connID, channel)
	return nil
}

// Unsubscribe 代理。
func (h *Hub) Unsubscribe(connID, channel string) error {
	return h.channels.Unsubscribe(connID, channel)
}

// ConnChannels 代理（连接订阅频道快照——数量上限/诊断面）。
func (h *Hub) ConnChannels(connID string) []string {
	return h.channels.ConnChannels(connID)
}
