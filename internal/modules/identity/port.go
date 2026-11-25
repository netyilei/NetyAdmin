// port.go：api.Identity 的模块侧实现（domain.Profile → api.Profile 转换）。
package identity

import (
	"context"

	"github.com/netyilei/netyadmin/internal/modules/identity/api"
	"github.com/netyilei/netyadmin/internal/modules/identity/application"
	"github.com/netyilei/netyadmin/internal/modules/identity/domain"
)

// identityPort 实现 api.Identity（注册名 identity.Service）。
type identityPort struct{ svc *application.Service }

var _ api.Identity = identityPort{}

// Profile 适配 Service.GetProfile。
func (p identityPort) Profile(ctx context.Context, adminID string) (api.Profile, error) {
	pr, err := p.svc.GetProfile(ctx, adminID)
	if err != nil {
		return api.Profile{}, err
	}
	return toProfile(pr), nil
}

// ListAdmins 适配 Service.ListAdmins（无过滤条件，仅分页）。
func (p identityPort) ListAdmins(ctx context.Context, offset, limit int) ([]api.Profile, int64, error) {
	list, total, err := p.svc.ListAdmins(ctx, domain.ListFilter{Offset: offset, Limit: limit})
	if err != nil {
		return nil, 0, err
	}
	out := make([]api.Profile, 0, len(list))
	for _, pr := range list {
		out = append(out, toProfile(pr))
	}
	return out, total, nil
}

func toProfile(pr domain.Profile) api.Profile {
	return api.Profile{
		ID: pr.ID, Username: pr.Username, Name: pr.Name,
		Email: pr.Email, Phone: pr.Phone, AvatarURL: pr.AvatarURL,
		Status: int8(pr.Status),
	}
}
