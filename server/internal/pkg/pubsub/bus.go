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
	handlers map[string][]func(msg []byte)
	mu       sync.RWMutex
}

func newBaseBus() *baseBus {
	return &baseBus{
		handlers: make(map[string][]func(msg []byte)),
	}
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
}

func NewMemoryDriver() EventBus {
	d := &MemoryDriver{
		baseBus:  newBaseBus(),
		stopChan: make(chan struct{}),
		msgChan:  make(chan *Message, 1000),
	}
	go d.loop()
	return d
}

func (d *MemoryDriver) Publish(ctx context.Context, topic string, msg interface{}) error {
	m, err := NewMessage(topic, msg)
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
		case msg := <-d.msgChan:
			d.dispatch(msg.Topic, msg.Payload)
		}
	}
}

func (d *MemoryDriver) Close() error {
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
}

func NewRedisDriver(redisClient *redis.Client, prefix string) EventBus {
	if prefix == "" {
		prefix = "netyadmin"
	}

	d := &RedisDriver{
		baseBus:     newBaseBus(),
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
	if d.redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}

	m, err := NewMessage(topic, msg)
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

	ctx := context.Background()
	sub := d.redisClient.Subscribe(ctx, d.channel)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-d.stopChan:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var m Message
			if err := json.Unmarshal([]byte(msg.Payload), &m); err != nil {
				continue
			}
			d.dispatch(m.Topic, m.Payload)
		}
	}
}

func (d *RedisDriver) Close() error {
	close(d.stopChan)
	d.wg.Wait()
	return nil
}
