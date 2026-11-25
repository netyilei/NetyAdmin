// port.go：api.Users 的模块侧实现（domain.User → api.User 转换）。
package users

import (
	"context"

	"github.com/netyilei/netyadmin/internal/modules/users/api"
	"github.com/netyilei/netyadmin/internal/modules/users/application"
	"github.com/netyilei/netyadmin/internal/modules/users/domain"
)

// usersPort 实现 api.Users（注册名 users.Users）。
type usersPort struct{ svc *application.Service }

var _ api.Users = usersPort{}

// Profile 适配 Service.Profile。
func (p usersPort) Profile(ctx context.Context, principalID string) (api.User, error) {
	u, err := p.svc.Profile(ctx, principalID)
	if err != nil {
		return api.User{}, err
	}
	return toUser(u), nil
}

// Get 适配 Service.GetUser。
func (p usersPort) Get(ctx context.Context, id string) (api.User, error) {
	u, err := p.svc.GetUser(ctx, id)
	if err != nil {
		return api.User{}, err
	}
	return toUser(u), nil
}

// TokenVersion 适配 Service.TokenVersionSource。
func (p usersPort) TokenVersion(ctx context.Context, id string) (int64, error) {
	return p.svc.TokenVersionSource(ctx, id)
}

func toUser(u *domain.User) api.User {
	if u == nil {
		return api.User{}
	}
	return api.User{
		ID: u.ID, Username: u.Username, Email: u.Email, Phone: u.Phone,
		Status: u.Status, TokenVersion: u.TokenVersion,
	}
}
