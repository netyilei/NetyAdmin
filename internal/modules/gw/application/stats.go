// stats.go：网关访问统计（2026-11-19全文件化零 DB）——
// 流水=logs/<date>/gw_access.log（log.Manager access 类型，每调用一行）；
// 聚合=进程内当前小时桶（计数/错误/状态分布/延迟直方图）；快照=每小时
// logs/<date>/gw_stats_hourly_<node>.jsonl（JSON 行——文件即聚合台账，随目录
// retention 清扫）；查询=当前桶+本机全部节点快照历史合并（管理端点消费）。
//
// 多节点边界（明确职责，不做事后陷阱）：
//   - 写入：每进程只写本节点文件（文件名带 node_id——同机多进程不再互相覆盖）；
//   - 查询：聚合读**本机 logs 目录下全部节点快照**（同机多进程部署形态：
//     单机多实例共享日志目录即可看全量历史）；
//   - 内存实时面：仅本节点当前小时（跨机实时聚合=指标系统职责，非本文件面）；
//   - 跨机历史聚合：日志采集器（file shipping → 集中存储）职责，本进程只认
//     本机目录；旧版单文件 gw_stats_hourly.jsonl 仍被读取（升级窗口兼容）。
package application

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	klog "github.com/netyilei/netyadmin/internal/kernel/log"
)

// StatsEntry 是一次网关调用记录（dispatcher 业务分支产生）。
type StatsEntry struct {
	AppID      string
	APIID      string
	Method     string
	Path       string
	StatusCode int
	BizCode    string
	LatencyMS  int64
	ClientIP   string
	RequestID  string
}

// latBuckets 是延迟直方图桶边界（ms）——快照/分位数共用（固定口径）。
var latBuckets = []int64{10, 50, 100, 200, 500, 1000, 2000, 5000}

// latOverflowMs 是溢出桶近似值（末桶上界×2——分位数展示面）。
const latOverflowMs = 10000

// hourAgg 是单小时聚合桶。
type hourAgg struct {
	hour   int64
	total  int
	errors int
	apps   map[string]*statCell
	apis   map[string]*statCell // key=method+" "+path
	status map[int]int
	biz    map[string]int // 业务码分布（验收列「业务码」——错误诊断面）
	lat    latencyAgg
}

type statCell struct {
	Calls  int        `json:"calls"`
	Errors int        `json:"errors"`
	Lat    latencyAgg `json:"lat"`
}

// latencyAgg 字段导出（快照 JSON 行序列化面——statCell 同理）。
type latencyAgg struct {
	SumMS int64   `json:"sumMs"`
	Count int64   `json:"count"`
	Hist  []int64 `json:"hist,omitempty"` // len(latBuckets)+1（末桶=溢出）
}

func (l *latencyAgg) add(ms int64) {
	l.SumMS += ms
	l.Count++
	if len(l.Hist) == 0 {
		l.Hist = make([]int64, len(latBuckets)+1)
	}
	for i, b := range latBuckets {
		if ms < b {
			l.Hist[i]++
			return
		}
	}
	l.Hist[len(latBuckets)]++
}

func (l latencyAgg) merge(o latencyAgg) latencyAgg {
	out := latencyAgg{SumMS: l.SumMS + o.SumMS, Count: l.Count + o.Count}
	out.Hist = make([]int64, len(latBuckets)+1)
	for i := range out.Hist {
		if i < len(l.Hist) {
			out.Hist[i] += l.Hist[i]
		}
		if i < len(o.Hist) {
			out.Hist[i] += o.Hist[i]
		}
	}
	return out
}

// percentile 由直方图近似（桶上界——快照粒度）。
func (l latencyAgg) percentile(p float64) float64 {
	if l.Count == 0 {
		return 0
	}
	target := int64(float64(l.Count) * p)
	var cum int64
	for i, c := range l.Hist {
		cum += c
		if cum >= target {
			if i < len(latBuckets) {
				return float64(latBuckets[i])
			}
			return float64(latOverflowMs) // 溢出桶近似（G1-08 具名）
		}
	}
	return float64(l.SumMS / max64(l.Count, 1))
}

func (l latencyAgg) avg() float64 {
	if l.Count == 0 {
		return 0
	}
	return float64(l.SumMS) / float64(l.Count)
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// StatsService 是网关统计面（记录+聚合+快照+查询；零 DB）。
type StatsService struct {
	log       *klog.Manager // nil 守卫（无 logger 面=仅内存聚合——测试）
	accessLog *slog.Logger  // gw access 流水 logger（构造期缓存——热路径免 Manager.mu 争用）
	dir       string        // 快照 jsonl 根（log.Manager Dir）
	node      string        // 本节点标识（快照文件名后缀——gw_stats_hourly_<node>.jsonl）
	stop      chan struct{}
	once      sync.Once

	mu  sync.Mutex
	agg *hourAgg
	now func() time.Time
}

// NewStatsService 创建（log=nil 时仅内存聚合不落文件——测试面）。
// 流水/快照 logger 构造期缓存一次（Record 是网关每请求热路径——避免每次
// 取 Manager.mu）。node 取 log.Manager.NodeID（与日志行 node_id 同源）。
func NewStatsService(log *klog.Manager) *StatsService {
	s := &StatsService{log: log, stop: make(chan struct{}), now: time.Now}
	if log != nil {
		s.dir = log.Dir()
		s.node = log.NodeID()
		s.accessLog = log.Logger("gw", "access")
	}
	hour := s.now().Unix() / 3600
	s.agg = newHourAgg(hour)
	return s
}

func newHourAgg(hour int64) *hourAgg {
	return &hourAgg{hour: hour, apps: map[string]*statCell{}, apis: map[string]*statCell{},
		status: map[int]int{}, biz: map[string]int{}}
}

// Record 记录一次网关调用（流水行+聚合；并发安全）。
func (s *StatsService) Record(e StatsEntry) {
	s.mu.Lock()
	hour := s.now().Unix() / 3600
	if hour != s.agg.hour { // 小时滚动：先快照上一桶再开新桶（惰性——无后台 goroutine）
		s.writeSnapshot(s.agg)
		s.agg = newHourAgg(hour)
	}
	s.agg.total++
	if e.StatusCode >= 400 {
		s.agg.errors++
	}
	sc := s.agg.status[e.StatusCode]
	s.agg.status[e.StatusCode] = sc + 1
	if e.BizCode != "" {
		s.agg.biz[e.BizCode]++
	}
	app := s.agg.apps[e.AppID]
	if app == nil {
		app = &statCell{}
		s.agg.apps[e.AppID] = app
	}
	app.Calls++
	if e.StatusCode >= 400 {
		app.Errors++
	}
	app.Lat.add(e.LatencyMS)
	api := s.agg.apis[e.Method+" "+e.Path]
	if api == nil {
		api = &statCell{}
		s.agg.apis[e.Method+" "+e.Path] = api
	}
	api.Calls++
	if e.StatusCode >= 400 {
		api.Errors++
	}
	api.Lat.add(e.LatencyMS)
	s.agg.lat.add(e.LatencyMS)
	s.mu.Unlock()

	if s.accessLog != nil {
		s.accessLog.Info("gateway.call",
			"app_id", e.AppID, "api_id", e.APIID, "method", e.Method, "path", e.Path,
			"status", e.StatusCode, "biz_code", e.BizCode, "latency_ms", e.LatencyMS,
			"ip", e.ClientIP, "request_id", e.RequestID)
	}
}

// statsSnapshot 是快照 JSON 行（gw_stats_hourly_<node>.jsonl——文件即聚合台账）。
type statsSnapshot struct {
	Hour    int64                `json:"hour"` // unix 小时
	Total   int                  `json:"total"`
	Errors  int                  `json:"errors"`
	Apps    map[string]*statCell `json:"apps"`
	Apis    map[string]*statCell `json:"apis"`
	Status  map[int]int          `json:"status"`
	Biz     map[string]int       `json:"biz,omitempty"`
	Latency latencyAgg           `json:"latency"`
}

// 快照文件名（多节点关键面）：
//
//	snapshotBase     = "gw_stats_hourly"      —— 旧版单文件（升级窗口内继续读）
//	snapshotExt      = ".jsonl"
//	snapshotNodeFmt  = "gw_stats_hourly_%s.jsonl" —— 每节点独立文件（写侧）
const (
	snapshotBase    = "gw_stats_hourly"
	snapshotExt     = ".jsonl"
	snapshotNodeFmt = "gw_stats_hourly_%s" + snapshotExt
)

// snapshotFileName 返回本节点快照文件名。node 为空（测试/极端装配）回退旧版
// 单文件名——保持既有单机语义与读取兼容。
func (s *StatsService) snapshotFileName() string {
	if s.node == "" {
		return snapshotBase + snapshotExt
	}
	return fmt.Sprintf(snapshotNodeFmt, sanitizeNodeForFile(s.node))
}

// sanitizeNodeForFile 把节点标识收敛为文件安全段（配置层已校验字符集，
// 这里防御程序内构造的任意 NodeID——文件名不得携带路径分隔/空白）。
func sanitizeNodeForFile(node string) string {
	var b strings.Builder
	b.Grow(len(node))
	for _, r := range node {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// writeSnapshot 追加快照行（调用方持 s.mu）。
func (s *StatsService) writeSnapshot(agg *hourAgg) {
	if s.dir == "" || agg.total == 0 {
		return // 无文件面/空桶不落
	}
	// 目录=本地日期（与 log.Manager dayDir/清扫同源——修正：UTC 与本地
	// 双口径会错位快照归属与 retention 清扫；读取侧 loadSnapshots 同口径）
	date := time.Unix(agg.hour*3600, 0).Local().Format("2006-01-02")
	path := filepath.Join(s.dir, date, s.snapshotFileName())
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return
	}
	raw, err := json.Marshal(statsSnapshot{
		Hour: agg.hour, Total: agg.total, Errors: agg.errors,
		Apps: agg.apps, Apis: agg.apis, Status: agg.status, Biz: agg.biz, Latency: agg.lat,
	})
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(raw, '\n'))
}

// Close 写终桶快照（优雅关闭——幂等）。
func (s *StatsService) Close() {
	s.once.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.agg.total > 0 {
			s.writeSnapshot(s.agg)
		}
	})
}

// —— 查询（管理端点消费；当前桶+快照历史合并）——

// TrendItem 是趋势点（小时粒度）。
type TrendItem struct {
	Hour       int64   `json:"hour"`
	Total      int64   `json:"total"`
	Success    int64   `json:"success"`
	Fail       int64   `json:"fail"`
	AvgLatency float64 `json:"avgLatency"`
}

// TopItem 是 top 榜项。
type TopItem struct {
	Key    string `json:"key"`
	Calls  int64  `json:"calls"`
	Errors int64  `json:"errors"`
}

// StatusItem 是状态分布项。
type StatusItem struct {
	Code  int   `json:"code"`
	Calls int64 `json:"calls"`
}

// LatencySummary 是延迟分位摘要（ms）。
type LatencySummary struct {
	Avg float64 `json:"avg"`
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
}

// Overview 是总览。
type Overview struct {
	Hours      int     `json:"hours"`
	Total      int64   `json:"total"`
	Errors     int64   `json:"errors"`
	ErrorRate  float64 `json:"errorRate"`
	AvgLatency float64 `json:"avgLatency"`
	P95        float64 `json:"p95"`
	Apps       int     `json:"apps"`
	APIs       int     `json:"apis"`
}

// loadSnapshots 读 range 内快照行（历史面；文件缺失=空）。
// 合并面=本机日志目录下**全部节点**文件（gw_stats_hourly_<node>.jsonl）+
// 旧版单文件 gw_stats_hourly.jsonl（升级窗口兼容）。同一小时多节点各一行，
// 查询聚合天然按行累加。跨机历史聚合=日志采集器职责（本进程只认本机目录）。
func (s *StatsService) loadSnapshots(hours int) []statsSnapshot {
	if s.dir == "" {
		return nil
	}
	now := s.now()
	var out []statsSnapshot
	// 跨日扫窗口内日期目录（本地日期口径——与 writeSnapshot 同源）
	for d := 0; d <= hours/24+1 && d < 8; d++ {
		date := now.AddDate(0, 0, -d).Local().Format("2006-01-02")
		dir := filepath.Join(s.dir, date)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			if ent.IsDir() || !isSnapshotFile(ent.Name()) {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(dir, ent.Name()))
			if err != nil {
				continue
			}
			for _, line := range splitLines(raw) {
				var snap statsSnapshot
				if json.Unmarshal(line, &snap) == nil {
					out = append(out, snap)
				}
			}
		}
	}
	return out
}

// isSnapshotFile 判定统计快照文件：旧版单文件名或本节点命名族（多节点）。
func isSnapshotFile(name string) bool {
	if name == snapshotBase+snapshotExt {
		return true // 旧版单文件（升级窗口读取兼容）
	}
	return strings.HasPrefix(name, snapshotBase+"_") && strings.HasSuffix(name, snapshotExt)
}

func splitLines(raw []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, b := range raw {
		if b == '\n' {
			if i > start {
				out = append(out, raw[start:i])
			}
			start = i + 1
		}
	}
	if start < len(raw) {
		out = append(out, raw[start:])
	}
	return out
}

// inRange 小时过滤（含当前桶）。
func (s *StatsService) inRange(hour int64, hours int) bool {
	cur := s.now().Unix() / 3600
	return hour > cur-int64(hours) && hour <= cur
}

// mergeCell 合并 statCell 到累计器。
func mergeCell(dst *statCell, src *statCell) {
	dst.Calls += src.Calls
	dst.Errors += src.Errors
	dst.Lat = dst.Lat.merge(src.Lat)
}

// Overview 汇总（当前桶持锁聚合——浅拷贝共享 map 与 Record 写并发=fatal 竞态，
// 修正：聚合在锁内完成；快照历史为不可变文件数据锁外合并）。
func (s *StatsService) Overview(hours int) Overview {
	out := Overview{Hours: hours}
	lat := latencyAgg{Hist: make([]int64, len(latBuckets)+1)}
	apps, apis := map[string]bool{}, map[string]bool{}
	acc := func(a *hourAgg) {
		out.Total += int64(a.total)
		out.Errors += int64(a.errors)
		lat = lat.merge(a.lat)
		for k := range a.apps {
			apps[k] = true
		}
		for k := range a.apis {
			apis[k] = true
		}
	}
	s.collect(hours, acc)
	out.ErrorRate = rate64(out.Errors, out.Total)
	out.AvgLatency = lat.avg()
	out.P95 = lat.percentile(0.95)
	out.Apps, out.APIs = len(apps), len(apis)
	return out
}

// collect 是五查询共用脚手架（G1-06：当前桶持锁+快照历史 inRange 合并——
// 防逐查询复制锁序；acc 回调消费每桶）。
func (s *StatsService) collect(hours int, acc func(a *hourAgg)) {
	s.mu.Lock()
	acc(s.agg)
	s.mu.Unlock()
	for _, snap := range s.loadSnapshots(hours) {
		if s.inRange(snap.Hour, hours) {
			acc(&hourAgg{hour: snap.Hour, total: snap.Total, errors: snap.Errors,
				apps: snap.Apps, apis: snap.Apis, status: snap.Status, biz: snap.Biz, lat: snap.Latency})
		}
	}
}

// Trend 趋势（小时粒度；当前桶+快照）。
func (s *StatsService) Trend(hours int) []TrendItem {
	cur := s.now().Unix() / 3600
	byHour := map[int64]*TrendItem{}
	for h := cur - int64(hours) + 1; h <= cur; h++ {
		byHour[h] = &TrendItem{Hour: h}
	}
	acc := func(a *hourAgg) {
		it, ok := byHour[a.hour]
		if !ok {
			return
		}
		it.Total += int64(a.total)
		it.Fail += int64(a.errors)
		it.Success += int64(a.total - a.errors)
		if a.lat.Count > 0 {
			prevCount := it.Success + it.Fail - int64(a.total)
			it.AvgLatency = (it.AvgLatency*float64(prevCount) + a.lat.avg()*float64(a.lat.Count)) / float64(max64(prevCount+int64(a.lat.Count), 1))
		}
	}
	s.collect(hours, acc)
	out := make([]TrendItem, 0, len(byHour))
	for _, it := range byHour {
		out = append(out, *it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hour < out[j].Hour })
	return out
}

// Top 按 apps/apis 榜（calls 降序）。
func (s *StatsService) Top(kind string, limit, hours int) []TopItem {
	cells := map[string]*statCell{}
	acc := func(a *hourAgg) {
		src := a.apis
		if kind == "apps" {
			src = a.apps
		}
		for k, c := range src {
			dst := cells[k]
			if dst == nil {
				dst = &statCell{}
				cells[k] = dst
			}
			mergeCell(dst, c)
		}
	}
	s.collect(hours, acc)
	out := make([]TopItem, 0, len(cells))
	for k, c := range cells {
		out = append(out, TopItem{Key: k, Calls: int64(c.Calls), Errors: int64(c.Errors)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Calls > out[j].Calls })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// StatusDist 状态码分布。
func (s *StatsService) StatusDist(hours int) []StatusItem {
	dist := map[int]int64{}
	acc := func(a *hourAgg) {
		for k, v := range a.status {
			dist[k] += int64(v)
		}
	}
	s.collect(hours, acc)
	out := make([]StatusItem, 0, len(dist))
	for k, v := range dist {
		out = append(out, StatusItem{Code: k, Calls: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// Latency 延迟分位摘要。
func (s *StatsService) Latency(hours int) LatencySummary {
	lat := latencyAgg{}
	acc := func(a *hourAgg) { lat = lat.merge(a.lat) }
	s.collect(hours, acc)
	return LatencySummary{Avg: lat.avg(), P50: lat.percentile(0.5), P95: lat.percentile(0.95)}
}

func rate64(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total)
}
