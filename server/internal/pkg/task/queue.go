package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Message 队列消息
type Message struct {
	TaskName string          `json:"task_name"`
	Payload  json.RawMessage `json:"payload"`
}

// Queue 驱动接口
type Queue interface {
	Push(ctx context.Context, msg *Message) error
	Pop(ctx context.Context) (*Message, error)
	Close() error
}

// --- Local Channel 实现 ---

type localQueue struct {
	ch chan *Message
}

func NewLocalQueue(size int) Queue {
	if size <= 0 {
		size = 1000
	}
	return &localQueue{ch: make(chan *Message, size)}
}

func (q *localQueue) Push(ctx context.Context, msg *Message) error {
	select {
	case q.ch <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("local queue is full")
	}
}

func (q *localQueue) Pop(ctx context.Context) (*Message, error) {
	select {
	case msg, ok := <-q.ch:
		if !ok {
			return nil, fmt.Errorf("queue closed")
		}
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (q *localQueue) Close() error {
	close(q.ch)
	return nil
}

// --- Redis 实现 ---

type redisQueue struct {
	client *redis.Client
	key    string
}

func NewRedisQueue(client *redis.Client, prefix string) Queue {
	return &redisQueue{
		client: client,
		key:    fmt.Sprintf("%s:task:queue", prefix),
	}
}

func (q *redisQueue) Push(ctx context.Context, msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return q.client.LPush(ctx, q.key, data).Err()
}

func (q *redisQueue) Pop(ctx context.Context) (*Message, error) {
	// 使用 BRPop 阻塞读取
	res, err := q.client.BRPop(ctx, 5*time.Second, q.key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // 超时
		}
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal([]byte(res[1]), &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (q *redisQueue) Close() error {
	return nil
}
