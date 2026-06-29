package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// EventBus 统一消息总线接口
type EventBus interface {
	Publish(ctx context.Context, topic string, msg interface{}) error
	Subscribe(topic string, handler func(msg []byte)) error
	Close() error
}

// Message 统一消息协议
type Message struct {
	Topic     string          `json:"topic"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"timestamp"`
	SenderID  string          `json:"senderId,omitempty"` // 发布节点 ID，driver 接收层据此过滤本节点回环
}

// NewMessage 创建消息
func NewMessage(topic string, payload interface{}) (*Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload failed: %w", err)
	}
	return &Message{
		Topic:     topic,
		Payload:   data,
		Timestamp: time.Now().Unix(),
	}, nil
}

// baseBus 基础实现，包含公共逻辑
type baseBus struct {
	nodeID   string // 本节点唯一标识，用于过滤自身广播回环
	handlers map[string][]func(msg []byte)
	mu       sync.RWMutex
}

func newBaseBus(nodeID string) *baseBus {
	return &baseBus{
		nodeID:   nodeID,
		handlers: make(map[string][]func(msg []byte)),
	}
}

// buildMessage 构造带 SenderID 的消息
func (b *baseBus) buildMessage(topic string, payload interface{}) (*Message, error) {
	m, err := NewMessage(topic, payload)
	if err != nil {
		return nil, err
	}
	m.SenderID = b.nodeID
	return m, nil
}

// isFromSelf 判断消息是否由本节点发出（用于过滤回环，避免双重处理）
// 兼容旧版本：若 SenderID 为空（旧节点发布），返回 false 不误过滤
func (b *baseBus) isFromSelf(m *Message) bool {
	return b.nodeID != "" && m.SenderID == b.nodeID
}

func (b *baseBus) Subscribe(topic string, handler func(msg []byte)) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], handler)
	return nil
}

func (b *baseBus) dispatch(topic string, payload []byte) {
	b.mu.RLock()
	handlers := make([]func(msg []byte), len(b.handlers[topic]))
	copy(handlers, b.handlers[topic])
	b.mu.RUnlock()

	for _, h := range handlers {
		go h(payload)
	}
}

// MemoryDriver 单机模式：基于内存 channel 实现
type MemoryDriver struct {
	*baseBus
	stopChan chan struct{}
	msgChan  chan *Message
	closed   bool
	closeMu  sync.Mutex
}

func NewMemoryDriver(nodeID string) EventBus {
	d := &MemoryDriver{
		baseBus:  newBaseBus(nodeID),
		stopChan: make(chan struct{}),
		msgChan:  make(chan *Message, 1000),
	}
	go d.loop()
	return d
}

func (d *MemoryDriver) Publish(ctx context.Context, topic string, msg interface{}) error {
	d.closeMu.Lock()
	if d.closed {
		d.closeMu.Unlock()
		return fmt.Errorf("event bus is closed")
	}
	d.closeMu.Unlock()

	m, err := d.buildMessage(topic, msg)
	if err != nil {
		return err
	}

	select {
	case d.msgChan <- m:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *MemoryDriver) loop() {
	for {
		select {
		case <-d.stopChan:
			return
		case msg, ok := <-d.msgChan:
			if !ok {
				return
			}
			// 过滤本节点发出的回环消息，避免双重处理（与 RedisDriver 行为一致）
			if d.isFromSelf(msg) {
				continue
			}
			d.dispatch(msg.Topic, msg.Payload)
		}
	}
}

func (d *MemoryDriver) Close() error {
	d.closeMu.Lock()
	defer d.closeMu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	close(d.stopChan)
	return nil
}

// RedisDriver 集群模式：基于 Redis Pub/Sub
type RedisDriver struct {
	*baseBus
	redisClient *redis.Client
	prefix      string
	channel     string
	stopChan    chan struct{}
	wg          sync.WaitGroup
	closed      bool
	closeMu     sync.Mutex
}

func NewRedisDriver(redisClient *redis.Client, prefix string, nodeID string) EventBus {
	if prefix == "" {
		prefix = "netyadmin"
	}

	d := &RedisDriver{
		baseBus:     newBaseBus(nodeID),
		redisClient: redisClient,
		prefix:      prefix,
		channel:     fmt.Sprintf("%s:channel:system_bus", prefix),
		stopChan:    make(chan struct{}),
	}

	if redisClient != nil {
		d.wg.Add(1)
		go d.subscribeLoop()
	}

	return d
}

func (d *RedisDriver) Publish(ctx context.Context, topic string, msg interface{}) error {
	d.closeMu.Lock()
	if d.closed {
		d.closeMu.Unlock()
		return fmt.Errorf("event bus is closed")
	}
	d.closeMu.Unlock()

	if d.redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}

	m, err := d.buildMessage(topic, msg)
	if err != nil {
		return err
	}

	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal message failed: %w", err)
	}

	return d.redisClient.Publish(ctx, d.channel, data).Err()
}

func (d *RedisDriver) subscribeLoop() {
	defer d.wg.Done()

	const reconnectDelay = 3 * time.Second

	for {
		d.closeMu.Lock()
		if d.closed {
			d.closeMu.Unlock()
			return
		}
		d.closeMu.Unlock()

		ctx := context.Background()
		sub := d.redisClient.Subscribe(ctx, d.channel)
		ch := sub.Channel()

		func() {
			defer sub.Close()
			for {
				select {
				case <-d.stopChan:
					return
				case msg, ok := <-ch:
					if !ok {
						// channel 关闭，Redis 断连，尝试重连
						return
					}
					var m Message
					if err := json.Unmarshal([]byte(msg.Payload), &m); err != nil {
						continue
					}
					// 过滤本节点发出的回环消息（Redis Pub/Sub 不区分发布者）
					if d.isFromSelf(&m) {
						continue
					}
					d.dispatch(m.Topic, m.Payload)
				}
			}
		}()

		// 检查是否应该退出
		select {
		case <-d.stopChan:
			return
		case <-time.After(reconnectDelay):
			// 重连延迟后继续循环
		}
	}
}

func (d *RedisDriver) Close() error {
	d.closeMu.Lock()
	if d.closed {
		d.closeMu.Unlock()
		return nil
	}
	d.closed = true
	d.closeMu.Unlock()

	close(d.stopChan)
	d.wg.Wait()
	return nil
}
