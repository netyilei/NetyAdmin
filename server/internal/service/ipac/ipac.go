package ipac

import (
	"context"
	"net"
	"sync"

	"NetyAdmin/internal/domain/entity/ipac"
	"NetyAdmin/internal/pkg/pubsub"
	ipacRepo "NetyAdmin/internal/repository/ipac"
)

type IPACService interface {
	CheckIP(ctx context.Context, ip string, appID *string) (bool, error)
	ReloadCache(ctx context.Context) error

	// CRUD
	List(ctx context.Context, query *ipacRepo.IPACQuery) ([]*ipac.IPAccessControl, int64, error)
	Create(ctx context.Context, item *ipac.IPAccessControl) error
	Update(ctx context.Context, item *ipac.IPAccessControl) error
	Delete(ctx context.Context, id uint) error
	DeleteBatch(ctx context.Context, ids []uint) error
}

type ipacService struct {
	repo     ipacRepo.IPACRepository
	eventBus pubsub.EventBus

	mu          sync.RWMutex
	globalDeny  []*net.IPNet
	globalAllow []*net.IPNet
	appRules    map[string]appIPRules
}

type appIPRules struct {
	Allow           []*net.IPNet
	Deny            []*net.IPNet
	IPFilterEnabled bool
}

func NewIPACService(repo ipacRepo.IPACRepository, eventBus pubsub.EventBus) IPACService {
	s := &ipacService{
		repo:     repo,
		eventBus: eventBus,
		appRules: make(map[string]appIPRules),
	}
	// Initial load
	_ = s.ReloadCache(context.Background())

	return s
}

func (s *ipacService) ReloadCache(ctx context.Context) error {
	rules, err := s.repo.GetAllEffective(ctx)
	if err != nil {
		return err
	}

	appStrategies, err := s.repo.GetAppIPFilterEnabled(ctx)
	if err != nil {
		return err
	}

	newGlobalDeny := make([]*net.IPNet, 0)
	newGlobalAllow := make([]*net.IPNet, 0)
	newAppRules := make(map[string]appIPRules)

	for _, r := range rules {
		_, ipNet, err := net.ParseCIDR(r.IPAddr)
		if err != nil {
			ip := net.ParseIP(r.IPAddr)
			if ip == nil {
				continue
			}
			mask := net.CIDRMask(32, 32)
			if ip.To4() == nil {
				mask = net.CIDRMask(128, 128)
			}
			ipNet = &net.IPNet{IP: ip, Mask: mask}
		}

		if r.AppID == nil || *r.AppID == "" {
			if r.Type == ipac.IPACTypeDeny {
				newGlobalDeny = append(newGlobalDeny, ipNet)
			} else {
				newGlobalAllow = append(newGlobalAllow, ipNet)
			}
		} else {
			appID := *r.AppID
			ar := newAppRules[appID]
			if r.Type == ipac.IPACTypeDeny {
				ar.Deny = append(ar.Deny, ipNet)
			} else {
				ar.Allow = append(ar.Allow, ipNet)
			}
			newAppRules[appID] = ar
		}
	}

	for appID := range appStrategies {
		ar := newAppRules[appID]
		ar.IPFilterEnabled = true
		newAppRules[appID] = ar
	}

	s.mu.Lock()
	s.globalDeny = newGlobalDeny
	s.globalAllow = newGlobalAllow
	s.appRules = newAppRules
	s.mu.Unlock()

	return nil
}

func (s *ipacService) CheckIP(ctx context.Context, ipStr string, appID *string) (bool, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. 全局封禁 (Global Deny) - 优先级最高
	for _, ipNet := range s.globalDeny {
		if ipNet.Contains(ip) {
			return false, nil
		}
	}

	// 2. 全局放行 (Global Allow)
	for _, ipNet := range s.globalAllow {
		if ipNet.Contains(ip) {
			return true, nil
		}
	}

	// 3. 应用级校验
	if appID != nil && *appID != "" {
		if rules, ok := s.appRules[*appID]; ok {
			if !rules.IPFilterEnabled {
				return true, nil
			}
			// 先检查应用封禁
			for _, ipNet := range rules.Deny {
				if ipNet.Contains(ip) {
					return false, nil
				}
			}
			// 再检查应用放行
			for _, ipNet := range rules.Allow {
				if ipNet.Contains(ip) {
					return true, nil
				}
			}
		}
	}

	// 默认放行
	return true, nil
}

func (s *ipacService) List(ctx context.Context, query *ipacRepo.IPACQuery) ([]*ipac.IPAccessControl, int64, error) {
	return s.repo.List(ctx, query)
}

func (s *ipacService) notifyReload(ctx context.Context) {
	if s.eventBus != nil {
		_ = s.eventBus.Publish(ctx, pubsub.TopicIPACReload, "reload")
	}
}

func (s *ipacService) Create(ctx context.Context, item *ipac.IPAccessControl) error {
	if err := s.repo.Create(ctx, item); err != nil {
		return err
	}
	s.notifyReload(ctx)
	return s.ReloadCache(ctx)
}

func (s *ipacService) Update(ctx context.Context, item *ipac.IPAccessControl) error {
	if err := s.repo.Update(ctx, item); err != nil {
		return err
	}
	s.notifyReload(ctx)
	return s.ReloadCache(ctx)
}

func (s *ipacService) Delete(ctx context.Context, id uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.notifyReload(ctx)
	return s.ReloadCache(ctx)
}

func (s *ipacService) DeleteBatch(ctx context.Context, ids []uint) error {
	if err := s.repo.DeleteBatch(ctx, ids); err != nil {
		return err
	}
	s.notifyReload(ctx)
	return s.ReloadCache(ctx)
}
