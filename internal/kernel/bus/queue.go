// queue.go：有界异步投递队列（§5.4 背压）——Broadcast 本地/远端段与
// Cluster 降级/消费段共用。满丢弃（计数归调用方）；Close 排空等待
// （ShutdownDrain 上限，超时残留交由后台 worker 继续处理完再退出）。
// worker ctx 固定 Background：异步投递不随发布方请求 ctx 取消而中断。
package bus

import (
	"sync"
	"time"
)

// deliveryItem 单条异步投递；ack 非 nil 表示 Cluster 条目（成功后 ack）。
type deliveryItem struct {
	msg    Message
	domain Domain
	ack    *pendingAck
}

// pendingAck Cluster 条目确认上下文。
type pendingAck struct {
	stream  string
	entryID string
}

// runFn worker 对条目的执行回调（Bus.runItem）。
type runFn func(item deliveryItem)

type queue struct {
	mu     sync.Mutex
	ch     chan deliveryItem
	closed bool
	wg     sync.WaitGroup
	run    runFn
}

func newQueue(size, workers int, run runFn) *queue {
	q := &queue{ch: make(chan deliveryItem, size), run: run}
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go func() {
			defer q.wg.Done()
			for item := range q.ch {
				q.run(item)
			}
		}()
	}
	return q
}

// enqueue 入队；满或已关闭返回 false（调用方计数丢弃）。
func (q *queue) enqueue(item deliveryItem) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return false
	}
	select {
	case q.ch <- item:
		return true
	default:
		return false
	}
}

// close 停止接收并等待排空；超过 timeout 返回残留条数（近似值，后台仍在处理）。
func (q *queue) close(timeout time.Duration) int {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return 0
	}
	q.closed = true
	close(q.ch)
	q.mu.Unlock()
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return 0
	case <-time.After(timeout):
		return len(q.ch)
	}
}
