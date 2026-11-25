// demote.go：hot_local 自动降级评估（contracts cache.admission.auto_demote）。
// 两个档位（同节拍评估——每 missStep 次 miss 一次；不自动升级）：
//  1. 命中率档：累计 hits/(hits+misses) < min_hit_ratio（0.7）→ 降为 shared；
//  2. QPS 档：滚动窗口（window）内请求数/窗口秒数 < min_qps（100）且观测已满窗
//     → 降为 shared（state-and-cache §5.1.1「QPS < 100 持续 10 分钟」）。
//
// QPS 窗口分桶环形计数（buckets 个桶，每桶 window/buckets）：读侧只在降级评估
// 节拍扫描桶（O(buckets)），写侧每请求 O(1) 锁内自增。
package cache

import (
	"sync"
	"time"
)

// demotePolicy 是 auto_demote 判定参数（mirror contracts；测试可覆写 Store.demote）。
type demotePolicy struct {
	minHitRatio float64       // auto_demote.min_hit_ratio
	minQPS      float64       // auto_demote.min_qps
	window      time.Duration // auto_demote.window
	buckets     int           // QPS 窗口分桶数（window/buckets=桶宽）
	missStep    int64         // 评估节拍（每 N 次 miss 评估一次；既有实现口径）
}

// defaultDemotePolicy 镜像 contracts/state-policies.yaml cache.admission.auto_demote。
func defaultDemotePolicy() demotePolicy {
	return demotePolicy{
		minHitRatio: 0.7,
		minQPS:      100,
		window:      10 * time.Minute,
		buckets:     10,
		missStep:    512,
	}
}

// qpsSlot 是一个时间桶的请求计数（ord=桶序号——防环回旧值误计入窗口）。
type qpsSlot struct {
	ord int64
	n   int64
}

// qpsWindow 是实体滚动请求计数（窗口化——与降级评估节拍同构）。
type qpsWindow struct {
	mu    sync.Mutex
	slots []qpsSlot
	start time.Time // 首次观测时间（满窗后才判定 QPS——防启动即降级）
	begun bool
}

// observe 记录一次请求（now 取 Store 注入时钟——测试可控）。
func (w *qpsWindow) observe(now time.Time, window time.Duration, buckets int) {
	bucket := bucketWidth(window, buckets)
	ord := now.UnixNano() / int64(bucket)
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.begun {
		w.start = now
		w.begun = true
	}
	if len(w.slots) != buckets {
		w.slots = make([]qpsSlot, buckets)
	}
	idx := int(((ord % int64(buckets)) + int64(buckets)) % int64(buckets))
	if w.slots[idx].ord != ord {
		w.slots[idx] = qpsSlot{ord: ord} // 环回覆盖：旧桶清零（防上一轮计数残留）
	}
	w.slots[idx].n++
}

// rate 返回窗口内平均 QPS 与观测是否已满窗（未满窗=不可判定，返回 false）。
func (w *qpsWindow) rate(now time.Time, window time.Duration, buckets int) (float64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.begun || window <= 0 {
		return 0, false
	}
	cur := now.UnixNano() / int64(bucketWidth(window, buckets))
	var total int64
	for _, s := range w.slots {
		if s.ord > cur-int64(buckets) && s.ord <= cur {
			total += s.n
		}
	}
	if now.Sub(w.start) < window {
		return 0, false // 观测未满窗（「持续 10 分钟」语义）
	}
	return float64(total) / window.Seconds(), true
}

// bucketWidth 返回桶宽（window/buckets，最小 1ns 防除零）。
func bucketWidth(window time.Duration, buckets int) time.Duration {
	if buckets <= 0 {
		buckets = 1
	}
	w := window / time.Duration(buckets)
	if w <= 0 {
		return time.Nanosecond
	}
	return w
}
