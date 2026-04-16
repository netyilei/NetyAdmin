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
	Push(ctx context.Context, msg *Message, weight int) error
	Pop(ctx context.Context) (*Message, error)
	Close() error
}

// --- Local Multi-Channel 实现 ---

type localQueue struct {
	high   chan *Message
	normal chan *Message
	low    chan *Message
}

func NewLocalQueue(size int) Queue {
	if size <= 0 {
		size = 1000
	}
	return &localQueue{
		high:   make(chan *Message, size),
		normal: make(chan *Message, size),
		low:    make(chan *Message, size),
	}
}

func (q *localQueue) Push(ctx context.Context, msg *Message, weight int) error {
	var target chan *Message
	if weight >= WeightEssential {
		target = q.high
	} else if weight >= WeightNormal {
		target = q.normal
	} else {
		target = q.low
	}

	select {
	case target <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("local queue is full for weight %d", weight)
	}
}

func (q *localQueue) Pop(ctx context.Context) (*Message, error) {
	for {
		// 1. 尝试非阻塞式读取 (优先级: high > normal > low)
		select {
		case msg := <-q.high:
			return msg, nil
		default:
		}

		select {
		case msg := <-q.normal:
			return msg, nil
		default:
		}

		select {
		case msg := <-q.low:
			return msg, nil
		default:
		}

		// 2. 全部为空，进入阻塞等待
		select {
		case msg := <-q.high:
			return msg, nil
		case msg := <-q.normal:
			return msg, nil
		case msg := <-q.low:
			return msg, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// 定时唤醒重新进行优先级检查
			continue
		}
	}
}

func (q *localQueue) Close() error {
	close(q.high)
	close(q.normal)
	close(q.low)
	return nil
}

// --- Redis Multi-List 实现 ---

type redisQueue struct {
	client *redis.Client
	high   string
	normal string
	low    string
}

func NewRedisQueue(client *redis.Client, prefix string) Queue {
	return &redisQueue{
		client: client,
		high:   fmt.Sprintf("%s:task:queue:high", prefix),
		normal: fmt.Sprintf("%s:task:queue:normal", prefix),
		low:    fmt.Sprintf("%s:task:queue:low", prefix),
	}
}

func (q *redisQueue) Push(ctx context.Context, msg *Message, weight int) error {
	var key string
	if weight >= WeightEssential {
		key = q.high
	} else if weight >= WeightNormal {
		key = q.normal
	} else {
		key = q.low
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return q.client.LPush(ctx, key, data).Err()
}

func (q *redisQueue) Pop(ctx context.Context) (*Message, error) {
	// BRPop 按顺序检查 key，第一个非空的 key 会被弹出
	// 这完美实现了多级优先级队列
	res, err := q.client.BRPop(ctx, 5*time.Second, q.high, q.normal, q.low).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // 超时
		}
		return nil, err
	}

	var msg Message
	// res[0] 是 key 名，res[1] 是值
	if err := json.Unmarshal([]byte(res[1]), &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (q *redisQueue) Close() error {
	return nil
}
