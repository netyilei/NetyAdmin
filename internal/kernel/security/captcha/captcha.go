// captcha.go：登录验证码面（补强——2026-11-21 用户 admin 登录加
// 英数验证码）。base64Captcha DriverString 生成（字符集剔除易混淆 0O1lI）。
// 存储按 Redis 可用性二选一（降级语义登记 contracts/state-policies.yaml captcha 段）：
//   - redisStore（默认）：SETEX+GETDEL 一次性消费，多节点共享一致；
//   - memoryStore（Redis 不可用——未配置或启动探活失败，single 拓扑）：
//     进程内 map（TTL+一次性+容量上限）；multi 拓扑 Redis 必需（装配层
//     fail-fast 拒启），运行期 Redis 故障=校验失败拒登录（fail-closed 不降级）。
package captcha

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"strings"
	"sync"
	"time"

	"github.com/mojocn/base64Captcha"
	"github.com/redis/go-redis/v9"
)

// 字符集：数字+小写，剔除易混淆 0/o/1/l/i（用户要求——人类辨识优先）。
const charset = "23456789abcdefghjkmnpqrstuvwxyz"

const (
	codeLen = 4               // 验证码位数
	ttl     = 3 * time.Minute // 答案存活窗（生成→提交）
)

// maxMemoryEntries 降级存储容量上限（满且无可清过期项=拒绝新生成——生成端点
// 报错 fail-closed，防内存滥用）。
const maxMemoryEntries = 10000

// Options 装配选项（Memory 判定在 cmd 组合根——与 coordination 同判定点）。
type Options struct {
	RedisAddr string // config.redis.addr（Memory=true 时忽略）
	Namespace string // Redis 键命名空间 <prefix>:<env>:captcha（namespaces.yaml 登记）
	Memory    bool   // true=进程内存储（single 无 Redis 降级；multi 恒 false）
}

// Captcha 是验证码面（存储可插拔——Redis/进程内，语义一致：TTL+一次性消费）。
type Captcha struct {
	store  answerStore
	driver *base64Captcha.DriverString
}

// answerStore 答案存储最小面（set 落库小写化+TTL；consume 一次性取出——
// 取出即删除，重放/并发双消费安全）。
type answerStore interface {
	set(id, answer string) error
	consume(ctx context.Context, id string) (string, error)
}

// New 创建（Memory=true 进程内降级；否则 Redis——地址与 coordination/session
// 同源 config.redis.addr；多节点部署必须同一 Redis，生成与校验可落不同节点）。
func New(opts Options) *Captcha {
	if opts.Memory {
		return &Captcha{store: newMemoryStore(), driver: newDriver()}
	}
	return &Captcha{
		store: &redisStore{
			cli: redis.NewClient(&redis.Options{
				Addr: opts.RedisAddr, DialTimeout: time.Second,
				ReadTimeout: time.Second, WriteTimeout: time.Second, MaxRetries: 0,
			}),
			namespace: opts.Namespace,
		},
		driver: newDriver(),
	}
}

func newDriver() *base64Captcha.DriverString {
	return &base64Captcha.DriverString{
		Height: 40, Width: 130, NoiseCount: 2, ShowLineOptions: 2,
		Length: codeLen, Source: charset,
		BgColor: &color.RGBA{R: 245, G: 245, B: 245, A: 255},
		Fonts:   []string{"wqy-microhei.ttc"},
	}
}

// Generate 生成验证码：返回 id 与 base64 图片（dataURL——前端 img 直接展示）。
func (c *Captcha) Generate(ctx context.Context) (id, imageB64 string, err error) {
	id, content, answer, err := base64Captcha.NewCaptcha(c.driver, generateStore{store: c.store}).Generate()
	if err != nil {
		return "", "", fmt.Errorf("captcha: generate: %w", err)
	}
	_ = ctx
	// 答案经 generateStore.Set 落库（小写化+TTL）——返回值仅诊断面，正常链路丢弃
	_ = answer
	return id, content, nil
}

// Verify 一次性校验（取出即消费——校验即失效；答案大小写不敏感；存储故障
// 一律 false=fail-closed）。
func (c *Captcha) Verify(ctx context.Context, id, code string) bool {
	if id == "" || code == "" {
		return false
	}
	stored, err := c.store.consume(ctx, id)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(stored), strings.TrimSpace(code))
}

// generateStore 适配 base64Captcha.Store（生成链路只落库；Get/Verify 不被生成器
// 消费——校验统一走 Captcha.Verify 的 consume 面保证一次性语义）。
type generateStore struct{ store answerStore }

func (g generateStore) Set(id, value string) error       { return g.store.set(id, value) }
func (g generateStore) Get(string, bool) string          { return "" }
func (g generateStore) Verify(string, string, bool) bool { return false }

// —— redisStore ——

type redisStore struct {
	cli       *redis.Client
	namespace string // <prefix>:<env>:captcha（铁律 6 环境段；namespaces.yaml 登记）
}

func (s *redisStore) key(id string) string { return AnswerKey(s.namespace, id) }

func (s *redisStore) set(id, answer string) error {
	return s.cli.Set(context.Background(), s.key(id), strings.ToLower(answer), ttl).Err()
}

func (s *redisStore) consume(ctx context.Context, id string) (string, error) {
	return s.cli.GetDel(ctx, s.key(id)).Result()
}

// —— memoryStore（single 无 Redis 降级）——

// errCaptchaAnswerGone 答案不存在/已过期/已被消费（调用方统一 fail-closed）。
var errCaptchaAnswerGone = errors.New("captcha: answer gone")

type memoryEntry struct {
	answer    string
	expiresAt time.Time
}

// memoryStore 进程内降级存储（单节点语义；互斥锁保护——登录并发面）。
type memoryStore struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	now     func() time.Time // 测试时钟注入
}

func newMemoryStore() *memoryStore {
	return &memoryStore{entries: make(map[string]memoryEntry), now: time.Now}
}

func (s *memoryStore) set(id, answer string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked()
	if len(s.entries) >= maxMemoryEntries {
		return fmt.Errorf("captcha: memory store full (%d entries)", maxMemoryEntries)
	}
	s.entries[id] = memoryEntry{answer: strings.ToLower(answer), expiresAt: s.now().Add(ttl)}
	return nil
}

func (s *memoryStore) consume(_ context.Context, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if !ok {
		return "", errCaptchaAnswerGone
	}
	delete(s.entries, id) // 取出即删——一次性语义与 GETDEL 对齐
	if s.now().After(e.expiresAt) {
		return "", errCaptchaAnswerGone
	}
	return e.answer, nil
}

// purgeExpiredLocked 清理过期项（写入前惰性触发——无后台 goroutine）。
func (s *memoryStore) purgeExpiredLocked() {
	now := s.now()
	for id, e := range s.entries {
		if now.After(e.expiresAt) {
			delete(s.entries, id)
		}
	}
}
