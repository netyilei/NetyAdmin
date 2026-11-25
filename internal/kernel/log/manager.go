package log

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// —— 类型与级别（logging.md） ——

// Types 是允许的日志类型（logging.md：app/access/error/security/audit）。
// 统计快照 gw_stats_hourly_<node>.jsonl 直写不经本表路由（JSON 行格式
// 全局 format——修正：stats 枚举无 writer 消费者=死枚举已撤）。
var Types = map[string]bool{
	"app": true, "access": true, "error": true, "security": true, "audit": true,
}

// validModule 模块名校验：非空、无点号/路径分隔符（防日志目录逃逸）。
func validModule(s string) bool {
	return s != "" && !strings.Contains(s, ".") && !strings.ContainsAny(s, `/\`)
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("log: 非法级别 %q（debug|info|warn|error）", s)
	}
}

func levelString(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "error"
	case l >= slog.LevelWarn:
		return "warn"
	case l >= slog.LevelInfo:
		return "info"
	default:
		return "debug"
	}
}

// —— Manager ——

// Options 是 Manager 构造参数（由 bootstrap 从 config.Log 组装注入；本包不 import config）。
type Options struct {
	Dir     string              // 日志根目录（logs）
	Level   string              // 全局级别（debug|info|warn|error）
	Console bool                // 控制台输出（仅开发； config.Log.Console）
	Format  string              // json|text（logging.md §5：开发 text；生产 json；空=json）
	NodeID  string              // 节点标识（字段 node_id）
	Redact  func(string) string // 出口脱敏函数（nil=恒等；由使用方用 primitives 掩码机制组合，零预设）
	// RedactKeys 是属性键名黑名单（大小写不敏感；精确或 _/- 后缀匹配——覆盖
	// old_password 等变体；命中=字符串值替换为 [redacted]，nil/空=不启用）。
	// 与 Redact（值模式）互补：password/token/secret 等凭据类值无稳定模式，
	// 只能按键名 fail-closed。
	RedactKeys []string
	Now        func() time.Time // 时钟注入（测试用；nil=time.Now）

	// 轮转面（logging.md §5；0 值=既有默认——MaxSizeMB 100/RetentionDays 30/
	// MaxBackups 不限；Compress 默认关）
	RetentionDays int           // 按日目录保留天数（lumberjack MaxAge + 日期目录清扫共用）
	MaxSizeMB     int           // 单文件大小上限 MB
	MaxBackups    int           // 备份份数（0=不限）
	Compress      bool          // 轮转备份 gzip 压缩（lumberjack Compress）
	DebugTTL      time.Duration // debug 覆盖自动恢复时长（0=不启用——§5 debug_ttl）
}

// Manager 是运行日志出口（logging.md）：
//   - 路径 <Dir>/<YYYY-MM-DD>/<module>_<type>.log，按日分目录；
//   - 类型 app/access/error/security/audit；debug 默认关；
//   - 全局级别 + 模块级/类型级覆盖，动态生效（含历史 logger）；
//   - 出口脱敏经 Options.Redact（默认恒等——PII 机制零预设，使用方组合 primitives 掩码注入）；
//   - 禁止写业务 DB（本包无任何 DB 依赖）。
type Manager struct {
	mu             sync.Mutex
	opts           Options
	global         slog.Level
	modLv          map[string]slog.Level // 模块级覆盖
	typLv          map[string]slog.Level // 类型级覆盖（优先于模块）
	loggerVars     map[string][]*slog.LevelVar
	loggers        map[string]*slog.Logger
	writers        map[string]*dailyWriter
	closed         atomic.Bool
	redactKeys     map[string]bool           // 键名黑名单（RedactKeys 小写归一；nil=不启用）
	debugTTL       time.Duration             // debug 覆盖自动恢复（0=不启用；logging.md §5 debug_ttl）
	debugOverrides map[string]*debugOverride // 在期 debug 回退计时（key: module:/type）
	sweptDay       string                    // 目录清扫已覆盖的自然日（按日一次）
}

// debugOverride 是一个在期 debug 覆盖的回退计时与恢复快照。
type debugOverride struct {
	timer   *time.Timer
	prev    *slog.Level // 覆盖前级别快照（nil=此前无覆盖，到期回落全局）
	restore func()      // 到期恢复动作（写回 prev 或删除覆盖）
}

// New 创建 Manager：初始化根目录；失败 fail-fast（启动期错误）。
func New(opts Options) (*Manager, error) {
	if opts.Dir == "" {
		return nil, fmt.Errorf("log: Dir 不能为空")
	}
	lvl, err := parseLevel(opts.Level)
	if err != nil {
		return nil, err
	}
	if opts.Format != "" && opts.Format != "json" && opts.Format != "text" {
		return nil, fmt.Errorf("log: 非法 format %q（json|text）", opts.Format)
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	redactKeys := make(map[string]bool, len(opts.RedactKeys))
	for _, k := range opts.RedactKeys {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			redactKeys[k] = true
		}
	}
	m := &Manager{
		opts:           opts,
		redactKeys:     redactKeys,
		debugTTL:       opts.DebugTTL,
		debugOverrides: map[string]*debugOverride{},
		global:         lvl,
		modLv:          map[string]slog.Level{},
		typLv:          map[string]slog.Level{},
		loggerVars:     map[string][]*slog.LevelVar{},
		loggers:        map[string]*slog.Logger{},
		writers:        map[string]*dailyWriter{},
	}
	if err := os.MkdirAll(m.dayDir(m.opts.Now()), 0o750); err != nil {
		return nil, fmt.Errorf("log: mkdir: %w", err)
	}
	m.sweepExpiredDirs(m.opts.Now())
	m.sweptDay = m.opts.Now().Format("2006-01-02")
	return m, nil
}

func (m *Manager) dayDir(now time.Time) string {
	return filepath.Join(m.opts.Dir, now.Format("2006-01-02"))
}

func (m *Manager) logPath(now time.Time, module, typ string) string {
	return filepath.Join(m.dayDir(now), fmt.Sprintf("%s_%s.log", module, typ))
}

// effectiveLevel 计算 (module,type) 生效级别：类型覆盖 > 模块覆盖 > 全局。
func (m *Manager) effectiveLevel(module, typ string) slog.Level {
	if l, ok := m.typLv[typ]; ok {
		return l
	}
	if l, ok := m.modLv[module]; ok {
		return l
	}
	return m.global
}

// applyLevels 把当前生效级别推进到所有历史 logger（动态开关的核心）。
func (m *Manager) applyLevels() {
	for key, vars := range m.loggerVars {
		module, typ, _ := strings.Cut(key, "|")
		eff := m.effectiveLevel(module, typ)
		for _, v := range vars {
			v.Set(eff)
		}
	}
}

// Dir 返回日志根目录（stats 快照 jsonl 路径派生——同目录清扫纪律）。
func (m *Manager) Dir() string { return m.opts.Dir }

// NodeID 返回本进程节点标识（快照文件名后缀等派生面单一来源——
// 与日志行 node_id 字段同值；config.deployment.node_id）。
func (m *Manager) NodeID() string { return m.opts.NodeID }

// Logger 返回 (module,type) 的 slog.Logger（固定字段 module/type/node_id）。
func (m *Manager) Logger(module, typ string) *slog.Logger {
	if !validModule(module) {
		panic(fmt.Sprintf("log: 非法模块名 %q（禁空/路径分隔符/..）", module))
	}
	if !Types[typ] {
		panic(fmt.Sprintf("log: 非法日志类型 %q（允许 app/access/error/security/audit）", typ))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Load() {
		panic("log: manager 已关闭")
	}
	key := module + "|" + typ
	if lg, ok := m.loggers[key]; ok {
		return lg
	}
	lv := new(slog.LevelVar)
	lv.Set(m.effectiveLevel(module, typ))
	m.loggerVars[key] = append(m.loggerVars[key], lv)

	w := m.writers[key]
	if w == nil {
		w = &dailyWriter{m: m, module: module, typ: typ, day: m.opts.Now().Format("2006-01-02")}
		w.open(m.opts.Now())
		m.writers[key] = w
	}
	out := io.Writer(w)
	if m.opts.Console {
		out = io.MultiWriter(w, os.Stdout)
	}

	var h slog.Handler
	if m.opts.Format == "text" {
		h = slog.NewTextHandler(out, &slog.HandlerOptions{Level: lv, ReplaceAttr: coreAttrReplace})
	} else {
		h = slog.NewJSONHandler(out, &slog.HandlerOptions{Level: lv, ReplaceAttr: coreAttrReplace})
	}
	h = &redactHandler{next: h, redact: m.opts.Redact, keys: m.redactKeys}
	lg := slog.New(h).With(
		slog.String("module", module),
		slog.String("type", typ),
		slog.String("node_id", m.opts.NodeID),
	)
	m.loggers[key] = lg
	return lg
}

// coreAttrReplace 是 text/json 两 handler 共用的核心属性改写（ts 字段名 +
// 级别字符串化）——单一实现防双份漂移。
func coreAttrReplace(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.TimeKey:
		a.Key = "ts"
	case slog.LevelKey:
		if lv, ok := a.Value.Any().(slog.Level); ok {
			a.Value = slog.StringValue(levelString(lv))
		}
	}
	return a
}

// startDebugTTL 为 debug 覆盖启动到期回退计时（logging.md §5 debug_ttl：防忘关）。
// 到期恢复覆盖前级别（prev=nil 回落全局/类型默认）；回调经计时器身份校验，
// 防旧回调误删新设置（TOCTOU）。
func (m *Manager) startDebugTTL(key string, prev *slog.Level, restore func()) {
	if m.debugTTL <= 0 {
		return
	}
	o := &debugOverride{prev: prev, restore: restore}
	o.timer = time.AfterFunc(m.debugTTL, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.debugOverrides[key] != o { // 身份不符=已被新设置取代，不动作
			return
		}
		delete(m.debugOverrides, key)
		o.restore()
		m.applyLevels()
	})
	m.debugOverrides[key] = o
}

// clearDebugTTL 停止并移除在期回退计时（reset/改非 debug 级别时调用）。
func (m *Manager) clearDebugTTL(key string) {
	if o, ok := m.debugOverrides[key]; ok {
		if o.timer != nil {
			o.timer.Stop()
		}
		delete(m.debugOverrides, key)
	}
}

// SetModuleLevel 动态设置模块级别覆盖（"reset" 恢复全局），对历史 logger 立即生效。
func (m *Manager) SetModuleLevel(module, level string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := "module:" + module
	if level == "reset" {
		m.clearDebugTTL(key)
		delete(m.modLv, module)
		m.applyLevels()
		return nil
	}
	lvl, err := parseLevel(level)
	if err != nil {
		return err
	}
	m.clearDebugTTL(key)
	var prev *slog.Level
	if p, had := m.modLv[module]; had {
		prev = &p
	}
	m.modLv[module] = lvl
	if lvl == slog.LevelDebug {
		m.startDebugTTL(key, prev, func() {
			if prev != nil {
				m.modLv[module] = *prev
			} else {
				delete(m.modLv, module)
			}
		})
	}
	m.applyLevels()
	return nil
}

// SetTypeLevel 动态设置类型级别覆盖（优先于模块级；"reset" 恢复）。
func (m *Manager) SetTypeLevel(typ, level string) error {
	if !Types[typ] {
		return fmt.Errorf("log: 非法日志类型 %q", typ)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := "type:" + typ
	if level == "reset" {
		m.clearDebugTTL(key)
		delete(m.typLv, typ)
		m.applyLevels()
		return nil
	}
	lvl, err := parseLevel(level)
	if err != nil {
		return err
	}
	m.clearDebugTTL(key)
	var prev *slog.Level
	if p, had := m.typLv[typ]; had {
		prev = &p
	}
	m.typLv[typ] = lvl
	if lvl == slog.LevelDebug {
		m.startDebugTTL(key, prev, func() {
			if prev != nil {
				m.typLv[typ] = *prev
			} else {
				delete(m.typLv, typ)
			}
		})
	}
	m.applyLevels()
	return nil
}

// Close 关闭全部文件句柄（优雅关闭， 调用；幂等）。
func (m *Manager) Close() error {
	m.mu.Lock()
	for _, o := range m.debugOverrides {
		if o.timer != nil {
			o.timer.Stop()
		}
	}
	m.debugOverrides = map[string]*debugOverride{}
	defer m.mu.Unlock()
	if m.closed.Load() {
		return nil
	}
	m.closed.Store(true)
	var firstErr error
	for _, w := range m.writers {
		if err := w.close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// —— 按日切换 writer（lumberjack 只做单文件大小备份；日期目录切换由 dailyWriter 负责） ——

// bufFlushSize 是高频类型（access/stats）的写缓冲（256KB——把 N 行合并为一次
// syscall，消高频日志单锁排队；app/error/security 保持同步直写——logging.md
// 「error/security 不丢」语义，崩溃零丢失）。
const bufFlushSize = 256 * 1024

// bufFlushInterval 是缓冲定时冲刷周期（1s——实时可见性与 syscall 合并的折衷；
// 冲刷时点=缓冲满自动/本定时/日期切换/Close）。
const bufFlushInterval = time.Second

// bufferedType 判定类型是否走缓冲（高频可丢容忍面）。
func bufferedType(typ string) bool { return typ == "access" || typ == "stats" }

type dailyWriter struct {
	m      *Manager
	module string
	typ    string
	day    string
	mu     sync.Mutex
	w      io.WriteCloser
	bw     *bufio.Writer // 非 nil=w 被缓冲（flush 时点：缓冲满自动/定时/日期切换/Close）
	armed  atomic.Bool   // 定时冲刷已排（首写武装→1s 后冲→解除——避免每行排定时器）
}

func (dw *dailyWriter) open(now time.Time) {
	path := dw.m.logPath(now, dw.module, dw.typ)
	f, err := newLumberjack(path, dw.m.opts.MaxSizeMB, dw.m.opts.MaxBackups, dw.m.opts.RetentionDays, dw.m.opts.Compress)
	if err != nil {
		// 文件写失败降级 stderr（运行日志不因磁盘问题阻断业务）
		fmt.Fprintf(os.Stderr, "log: open %s failed: %v\n", path, err)
		dw.w = nopWriteCloser{os.Stderr}
		return
	}
	if bufferedType(dw.typ) {
		dw.bw = bufio.NewWriterSize(f, bufFlushSize)
		dw.w = &bufWriteCloser{Writer: dw.bw, Closer: f}
		return
	}
	dw.w = f
}

// bufWriteCloser 把 bufio.Writer 与底层 Closer 组合（Close=先冲缓冲再关文件）。
type bufWriteCloser struct {
	*bufio.Writer
	Closer io.Closer
}

func (b *bufWriteCloser) Close() error {
	if err := b.Writer.Flush(); err != nil {
		return err
	}
	return b.Closer.Close()
}

func (dw *dailyWriter) Write(p []byte) (int, error) {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	if dw.m.closed.Load() {
		return len(p), nil // 关闭后丢弃（幂等关闭语义）
	}
	now := dw.m.opts.Now()
	day := now.Format("2006-01-02")
	if day != dw.day {
		_ = dw.w.Close()
		dw.day = day
		dw.open(now)
		dw.m.sweepOncePerDay(now)
	}
	n, err := dw.w.Write(p)
	if err != nil {
		// lumberjack 惰性打开，打开/写失败在此暴露：降级 stderr（日志不静默丢失）
		fmt.Fprintf(os.Stderr, "log: file write failed, fallback stderr: %v\n", err)
		return os.Stderr.Write(p)
	}
	if dw.bw != nil && dw.armed.CompareAndSwap(false, true) {
		time.AfterFunc(bufFlushInterval, dw.scheduledFlush) // 1s 定时冲刷（实时可见性）
	}
	return n, nil
}

// scheduledFlush 是缓冲定时冲刷回调（并发安全——与写/日期切换互斥于 dw.mu）。
func (dw *dailyWriter) scheduledFlush() {
	if dw.m.closed.Load() {
		return
	}
	dw.mu.Lock()
	defer dw.mu.Unlock()
	dw.armed.Store(false)
	if dw.bw != nil {
		_ = dw.bw.Flush()
	}
}

func (dw *dailyWriter) close() error {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	return dw.w.Close()
}

// —— 出口脱敏 handler（msg 与全部 string 属性值——含 With 预挂——过 Redact；
// 键名命中 RedactKeys 的字符串属性整值 [redacted]——凭据类无值模式可依） ——

type redactHandler struct {
	next   slog.Handler
	redact func(string) string
	keys   map[string]bool
}

func (h *redactHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

func (h *redactHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.redact != nil || len(h.keys) > 0 {
		if h.redact != nil {
			r.Message = h.redact(r.Message)
		}
		var out slog.Record
		out.Time = r.Time
		out.Level = r.Level
		out.PC = r.PC
		out.Message = r.Message
		r.Attrs(func(a slog.Attr) bool {
			out.AddAttrs(redactValue(a, h.redact, h.keys))
			return true
		})
		r = out
	}
	return h.next.Handle(ctx, r)
}

func (h *redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if h.redact != nil || len(h.keys) > 0 {
		for i := range attrs {
			attrs[i] = redactValue(attrs[i], h.redact, h.keys)
		}
	}
	return &redactHandler{next: h.next.WithAttrs(attrs), redact: h.redact, keys: h.keys}
}

func (h *redactHandler) WithGroup(name string) slog.Handler {
	return &redactHandler{next: h.next.WithGroup(name), redact: h.redact, keys: h.keys}
}

// redactValue 递归脱敏属性值（键名黑名单整值遮蔽；string 过值模式脱敏；group 逐子项）。
func redactValue(a slog.Attr, redact func(string) string, keys map[string]bool) slog.Attr {
	v := a.Value
	if len(keys) > 0 && v.Kind() == slog.KindString && sensitiveKey(keys, a.Key) {
		return slog.Attr{Key: a.Key, Value: slog.StringValue("[redacted]")}
	}
	switch v.Kind() {
	case slog.KindString:
		if redact == nil {
			return a
		}
		return slog.Attr{Key: a.Key, Value: slog.StringValue(redact(v.String()))}
	case slog.KindGroup:
		group := v.Group()
		for i := range group {
			group[i] = redactValue(group[i], redact, keys)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(group...)}
	default:
		return a
	}
}

// sensitiveKey 判定属性键是否命中黑名单（精确或下划线/连字符后缀——
// 覆盖 old_password/new-password 等变体；大小写不敏感）。
func sensitiveKey(keys map[string]bool, key string) bool {
	k := strings.ToLower(key)
	if keys[k] {
		return true
	}
	for base := range keys {
		if strings.HasSuffix(k, "_"+base) || strings.HasSuffix(k, "-"+base) {
			return true
		}
	}
	return false
}

// —— 按日目录保留清扫（B1：lumberjack 的 mill 只扫当前文件所在目录，
// 按日目录的历史日期目录永不自清——由 Manager 补目录级清理） ——

// sweepOncePerDay 触发过期目录清扫（每自然日最多一次；由日期翻转驱动，
// 无后台 goroutine——模块契约「goroutine 不在 Init 起」的同源精神）。
func (m *Manager) sweepOncePerDay(now time.Time) {
	day := now.Format("2006-01-02")
	m.mu.Lock()
	if m.sweptDay == day {
		m.mu.Unlock()
		return
	}
	m.sweptDay = day
	m.mu.Unlock()
	m.sweepExpiredDirs(now)
}

// sweepExpiredDirs 删除超过保留期的按日日志目录（目录名必须严格为
// YYYY-MM-DD；解析失败/未过期的跳过）。清扫失败仅 stderr 提示，不阻断业务。
func (m *Manager) sweepExpiredDirs(now time.Time) {
	days := m.opts.RetentionDays
	if days <= 0 {
		days = 30 // 与 newLumberjack 的 retention 默认一致
	}
	cutoff := now.AddDate(0, 0, -days)
	entries, err := os.ReadDir(m.opts.Dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "log: sweep read %s failed: %v\n", m.opts.Dir, err)
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		d, err := time.Parse("2006-01-02", e.Name())
		if err != nil || !d.Before(cutoff) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(m.opts.Dir, e.Name())); err != nil {
			fmt.Fprintf(os.Stderr, "log: sweep %s failed: %v\n", e.Name(), err)
		}
	}
}

// —— 小帮手 ——

// newLumberjack 构造轮转 writer（logging.md §5 参数面：max_size_mb/
// max_backups/retention_days/compress——数值 0 分别取默认 100/不限）。
func newLumberjack(path string, maxSizeMB, maxBackups, retentionDays int, compress bool) (io.WriteCloser, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	if maxSizeMB <= 0 {
		maxSizeMB = 100
	}
	if retentionDays <= 0 {
		retentionDays = 30
	}
	return &lumberjack.Logger{
		Filename:   path,
		MaxSize:    maxSizeMB,  // MB，单文件大小备份（按日目录已按天分割）
		MaxBackups: maxBackups, // 0=保留全部历史（目录按日组织）
		MaxAge:     retentionDays,
		Compress:   compress,
	}, nil
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
