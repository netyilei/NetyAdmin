package middleware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"NetyAdmin/internal/domain/entity"
	jwtPkg "NetyAdmin/internal/pkg/jwt"
)

// TestUserClaimsAccessorFor_TypeIsolation 锁定 typed 用户 accessor 的角色隔离契约：
// accessor 只接受 claims.Type == expectedType 的 token，跨角色一律按禁用拒绝，
// 且拒绝路径不得触碰账户查询（userRepo 为 nil，误触碰即 panic 暴露缺陷）。
func TestUserClaimsAccessorFor_TypeIsolation(t *testing.T) {
	// 零值 AuthMiddleware（userRepo=nil）：角色隔离拒绝路径不查库，安全可测。
	m := &AuthMiddleware{}

	t.Run("空userType归一化为默认user类型", func(t *testing.T) {
		a := m.UserClaimsAccessorFor("")
		assert.Equal(t, jwtPkg.DefaultUserType, a.(userClaimsAccessor).expectedType)
	})

	t.Run("typed角色按传入userType隔离", func(t *testing.T) {
		a := m.UserClaimsAccessorFor("tech")
		assert.Equal(t, "tech", a.(userClaimsAccessor).expectedType)
	})

	t.Run("默认user accessor拒绝typed token", func(t *testing.T) {
		a := m.UserClaimsAccessorFor("")
		result, err := a.LookupAccount(context.Background(), &jwtPkg.UserClaims{Type: "tech"}, "raw")
		assert.NoError(t, err)
		assert.Equal(t, entity.StatusDisabled, result.Status, "type=tech 的 token 不得通过默认 user accessor")
	})

	t.Run("typed accessor拒绝默认user token", func(t *testing.T) {
		a := m.UserClaimsAccessorFor("tech")
		result, err := a.LookupAccount(context.Background(), &jwtPkg.UserClaims{Type: jwtPkg.DefaultUserType}, "raw")
		assert.NoError(t, err)
		assert.Equal(t, entity.StatusDisabled, result.Status, "type=user 的 token 不得通过 tech accessor")
	})

	t.Run("typed accessor拒绝其他typed角色token", func(t *testing.T) {
		a := m.UserClaimsAccessorFor("tech")
		result, err := a.LookupAccount(context.Background(), &jwtPkg.UserClaims{Type: "merchant"}, "raw")
		assert.NoError(t, err)
		assert.Equal(t, entity.StatusDisabled, result.Status, "type=merchant 的 token 不得通过 tech accessor")
	})
}
