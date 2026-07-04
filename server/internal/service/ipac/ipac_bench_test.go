package ipac

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/yl2chen/cidranger"

	ipacEntity "NetyAdmin/internal/domain/entity/ipac"
	ipacRepo "NetyAdmin/internal/repository/ipac"
)

// buildBenchService 直接构造 ipacService 并填充 N 条 CIDR 规则到 globalDeny，
// 跳过 NewIPACService 的 ReloadCache（避免依赖真实 repo）。
// 用于 benchmark CheckIP 在大规则集下的查询性能。
func buildBenchService(n int) *ipacService {
	s := &ipacService{
		appRules:   make(map[string]appIPRules),
		globalDeny: cidranger.NewPCTrieRanger(),
		// globalAllow 保留 nil：未配置白名单，CheckIP 走默认放行
	}

	// 生成 n 条 CIDR 规则：10.x.y.0/24, 10.x.y.z/32, 192.168.x.0/24, 172.16.x.y/32
	// 覆盖不同前缀长度，模拟真实场景的混合规则集
	for i := 0; i < n; i++ {
		var cidr string
		switch i % 4 {
		case 0:
			cidr = fmt.Sprintf("10.%d.0.0/16", i/4%256)
		case 1:
			cidr = fmt.Sprintf("10.%d.%d.0/24", i/4%256, (i/16)%256)
		case 2:
			cidr = fmt.Sprintf("192.168.%d.0/24", i%256)
		case 3:
			cidr = fmt.Sprintf("172.16.%d.%d/32", i%256, (i/4)%256)
		}
		ipNet := parseIPNet(cidr)
		if ipNet == nil {
			continue
		}
		_ = s.globalDeny.Insert(cidranger.NewBasicRangerEntry(*ipNet))
	}
	return s
}

// BenchmarkCheckIP_Trie_Hit benchmarks trie-based CheckIP when IP matches a rule.
// 命中规则：查询 10.0.0.5（命中 10.0.0.0/16，第 0 条规则）。
func BenchmarkCheckIP_Trie_Hit(b *testing.B) {
	s := buildBenchService(1000)
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = s.CheckIP(ctx, "10.0.0.5", nil)
	}
}

// BenchmarkCheckIP_Trie_Miss benchmarks trie-based CheckIP when IP does NOT match any rule.
// 未命中：查询 8.8.8.8（不在任何规则 CIDR 内），最坏情况需要遍历整个 trie。
func BenchmarkCheckIP_Trie_Miss(b *testing.B) {
	s := buildBenchService(1000)
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = s.CheckIP(ctx, "8.8.8.8", nil)
	}
}

// BenchmarkCheckIP_Trie_CIDRQuery benchmarks trie-based CheckIP for a CIDR-range query pattern.
// 用查询 IP "10.5.3.7" 命中 "10.5.0.0/16"（典型 CIDR 命中，验证 trie 在多前缀混合场景下的查询性能）。
func BenchmarkCheckIP_Trie_CIDRQuery(b *testing.B) {
	s := buildBenchService(1000)
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = s.CheckIP(ctx, "10.5.3.7", nil)
	}
}

// BenchmarkCheckIP_Trie_AppDefault benchmarks trie-based CheckIP 命中应用级 IPFilterEnabled=true 但无 Allow/Deny 配置（默认放行路径，最常见场景）。
func BenchmarkCheckIP_Trie_AppDefault(b *testing.B) {
	s := buildBenchService(1000)
	// 配置一个 app：IPFilterEnabled=true，无 Allow/Deny 规则（命中默认放行路径）
	appID := "01HAPPBENCHAPPBENCHAPPBENCH"
	s.appRules[appID] = appIPRules{
		Deny:            cidranger.NewPCTrieRanger(),
		Allow:           nil, // 未配置白名单
		IPFilterEnabled: true,
	}
	ctx := context.Background()
	appIDPtr := &appID
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = s.CheckIP(ctx, "8.8.8.8", appIDPtr)
	}
}

// BenchmarkCheckIP_Linear_Hit baseline：原线性扫描实现，用于对比 trie 性能提升。
// 用相同的 1000 条规则做线性 Contains 扫描。
func BenchmarkCheckIP_Linear_Hit(b *testing.B) {
	nets := buildLinearNets(1000)
	ip := net.ParseIP("10.0.0.5")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = linearContains(nets, ip)
	}
}

// BenchmarkCheckIP_Linear_Miss baseline：原线性扫描最坏情况（未命中需遍历全部规则）。
func BenchmarkCheckIP_Linear_Miss(b *testing.B) {
	nets := buildLinearNets(1000)
	ip := net.ParseIP("8.8.8.8")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = linearContains(nets, ip)
	}
}

// buildLinearNets 构造与 buildBenchService 相同的 1000 条 *net.IPNet，用于线性扫描 baseline。
func buildLinearNets(n int) []*net.IPNet {
	out := make([]*net.IPNet, 0, n)
	for i := 0; i < n; i++ {
		var cidr string
		switch i % 4 {
		case 0:
			cidr = fmt.Sprintf("10.%d.0.0/16", i/4%256)
		case 1:
			cidr = fmt.Sprintf("10.%d.%d.0/24", i/4%256, (i/16)%256)
		case 2:
			cidr = fmt.Sprintf("192.168.%d.0/24", i%256)
		case 3:
			cidr = fmt.Sprintf("172.16.%d.%d/32", i%256, (i/4)%256)
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		out = append(out, ipNet)
	}
	return out
}

// linearContains 复现原 CheckIP 中 []*net.IPNet 的线性扫描逻辑。
func linearContains(nets []*net.IPNet, ip net.IP) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// TestTrieVsLinearEquivalence 验证 trie 实现与线性扫描在相同规则集下结果一致。
// 这是正确性回归测试，确保 CheckIP 在 trie 化后行为未改变。
func TestTrieVsLinearEquivalence(t *testing.T) {
	const n = 1000
	s := buildBenchService(n)
	nets := buildLinearNets(n)

	// 选若干代表性 IP 做等价性校验
	testIPs := []string{
		"10.0.0.5",     // 命中 10.0.0.0/16
		"10.0.1.1",     // 命中 10.0.0.0/16
		"10.1.0.1",     // 命中 10.1.0.0/24（如果存在）
		"192.168.0.1",  // 命中 192.168.0.0/24
		"172.16.0.0",   // 命中 172.16.0.0/32（如果存在）
		"8.8.8.8",      // 未命中
		"127.0.0.1",    // 未命中
		"10.255.255.1", // 可能命中 10.255.0.0/16
	}

	ctx := context.Background()
	for _, ipStr := range testIPs {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Fatalf("invalid ip: %s", ipStr)
		}
		// trie 版 CheckIP（注意：CheckIP 命中 Deny 返回 false=拒绝）
		trieAllowed, _ := s.CheckIP(ctx, ipStr, nil)
		// 线性版只判断是否在 globalDeny 中
		linearInDeny := linearContains(nets, ip)

		// CheckIP 命中 Deny → trieAllowed=false；线性版 inDeny=true → 也应返回 false
		if linearInDeny && trieAllowed {
			t.Errorf("ip=%s: linear says in deny (should block), trie allowed (bug)", ipStr)
		}
		if !linearInDeny && !trieAllowed {
			t.Errorf("ip=%s: linear says not in deny (should allow), trie blocked (bug)", ipStr)
		}
	}
}

// TestReloadCache_VersionSkip 验证 SubTask 22.3：相同规则集合第二次 ReloadCache 应跳过重建。
// 通过 fingerprint 比对，相同输入不应触发 trie 重建（此处用 mock repo 提供相同数据）。
func TestReloadCache_VersionSkip(t *testing.T) {
	repo := &mockIPACRepo{
		rules:         buildMockRules(100),
		appStrategies: map[string]bool{},
	}
	s := &ipacService{
		repo:       repo,
		appRules:   make(map[string]appIPRules),
		globalDeny: cidranger.NewPCTrieRanger(),
	}

	// 第一次 ReloadCache：应触发重建，fingerprint 被缓存
	if err := s.ReloadCache(context.Background()); err != nil {
		t.Fatalf("first reload failed: %v", err)
	}
	fp1 := s.cachedRuleFingerprint
	if fp1 == "" {
		t.Fatal("fingerprint should be set after first reload")
	}

	// 第二次 ReloadCache：相同规则集，应跳过重建
	if err := s.ReloadCache(context.Background()); err != nil {
		t.Fatalf("second reload failed: %v", err)
	}
	// fingerprint 应保持不变（跳过重建时不更新）
	if s.cachedRuleFingerprint != fp1 {
		t.Errorf("fingerprint changed on identical reload: got %s, want %s", s.cachedRuleFingerprint, fp1)
	}

	// 修改规则集：应触发重建
	repo.rules = append(repo.rules, &ipacEntity.IPAccessControl{
		AppID:  nil,
		IPAddr: "99.99.99.99/32",
		Type:   ipacEntity.IPACTypeDeny,
		Status: ipacEntity.IPACStatusEnabled,
	})
	if err := s.ReloadCache(context.Background()); err != nil {
		t.Fatalf("third reload failed: %v", err)
	}
	if s.cachedRuleFingerprint == fp1 {
		t.Error("fingerprint should change after rules modified")
	}
}

// buildMockRules 构造 n 条 mock IPAC 规则用于 ReloadCache 测试。
func buildMockRules(n int) []*ipacEntity.IPAccessControl {
	rules := make([]*ipacEntity.IPAccessControl, 0, n)
	for i := 0; i < n; i++ {
		var cidr string
		switch i % 4 {
		case 0:
			cidr = fmt.Sprintf("10.%d.0.0/16", i/4%256)
		case 1:
			cidr = fmt.Sprintf("10.%d.%d.0/24", i/4%256, (i/16)%256)
		case 2:
			cidr = fmt.Sprintf("192.168.%d.0/24", i%256)
		case 3:
			cidr = fmt.Sprintf("172.16.%d.%d/32", i%256, (i/4)%256)
		}
		rules = append(rules, &ipacEntity.IPAccessControl{
			AppID:  nil,
			IPAddr: cidr,
			Type:   ipacEntity.IPACTypeDeny,
			Status: ipacEntity.IPACStatusEnabled,
		})
	}
	return rules
}

// mockIPACRepo 仅实现 IPACRepository 接口的部分方法，供 ReloadCache 测试使用。
// 未实现的方法 panic 即可（测试不会触发）。
type mockIPACRepo struct {
	rules         []*ipacEntity.IPAccessControl
	appStrategies map[string]bool
}

func (m *mockIPACRepo) Create(ctx context.Context, item *ipacEntity.IPAccessControl) error {
	panic("not implemented")
}
func (m *mockIPACRepo) Update(ctx context.Context, item *ipacEntity.IPAccessControl) error {
	panic("not implemented")
}
func (m *mockIPACRepo) Delete(ctx context.Context, id uint) error {
	panic("not implemented")
}
func (m *mockIPACRepo) GetByID(ctx context.Context, id uint) (*ipacEntity.IPAccessControl, error) {
	panic("not implemented")
}
func (m *mockIPACRepo) ExistsByID(ctx context.Context, id uint) (bool, error) {
	panic("not implemented")
}
func (m *mockIPACRepo) List(ctx context.Context, query *ipacRepo.IPACQuery) ([]*ipacEntity.IPAccessControl, int64, error) {
	panic("not implemented")
}
func (m *mockIPACRepo) GetByIP(ctx context.Context, ip string, appID *string) (*ipacEntity.IPAccessControl, error) {
	panic("not implemented")
}
func (m *mockIPACRepo) GetAllEffective(ctx context.Context) ([]*ipacEntity.IPAccessControl, error) {
	return m.rules, nil
}
func (m *mockIPACRepo) DeleteBatch(ctx context.Context, ids []uint) error {
	panic("not implemented")
}
func (m *mockIPACRepo) GetAppIPFilterEnabled(ctx context.Context) (map[string]bool, error) {
	return m.appStrategies, nil
}
func (m *mockIPACRepo) LinkRulesToApp(ctx context.Context, appID string, ruleIDs []uint) error {
	panic("not implemented")
}
