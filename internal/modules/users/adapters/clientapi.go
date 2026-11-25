// clientapi.go：users client 端点（US-01 用户需求面——注册/登录/改用户名/
// 改手机号/profile）+ AuthMiddleware（User JWT—— ticket 签发面激活）。
// 路由由 users 模块自注册至 RouteRegistrar 组（cmd 传组——clientapi 零 import）。
package adapters

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/users/application"
)

// Deps 是 client 端点依赖。
type ClientDeps struct {
	Service *application.Service
}

// AuthMiddleware 是 User JWT 鉴权（client authed 组级——与 identity
// AuthMiddleware 同构；哨兵细分映射 10101-10104）。
func AuthMiddleware(v *authn.JWTVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, err := v.Authenticate(c.Request.Context(),
			authn.Credential{Token: bearerToken(c.GetHeader("Authorization"))})
		if err != nil {
			transport.FailOf(c, mapUserAuthnError(err))
			return
		}
		c.Set("principal", principal)
		// ctx 注入（audit D②同构根修 2026-11-21）：request hook payload 读 ctx 版
		// authn.PrincipalFrom——gin key 仅 gin 侧可见，双面注入单一事实。
		c.Request = c.Request.WithContext(authn.WithPrincipal(c.Request.Context(), principal))
		c.Next()
	}
}

// mapUserError 哨兵 → 业务码（AGENTS §6 单一源——禁裸哨兵归一 10009）。
func mapUserError(err error) error {
	switch {
	case errors.Is(err, application.ErrEmailTaken), errors.Is(err, application.ErrUsernameTaken),
		errors.Is(err, application.ErrPhoneTaken):
		return errorx.New(errorx.CodeConflict)
	case errors.Is(err, application.ErrInvalidCredentials), errors.Is(err, application.ErrUserDisabled):
		return errorx.New(errorx.CodeUnauthorized)
	case errors.Is(err, application.ErrUserNotFound):
		return errorx.New(errorx.CodeNotFound)
	case errors.Is(err, application.ErrInvalidEmail), errors.Is(err, application.ErrInvalidPhone),
		errors.Is(err, application.ErrUsernameInvalid), errors.Is(err, application.ErrPasswordTooLong):
		return errorx.New(errorx.CodeValidationFailed)
	}
	return err
}

// mapUserAuthnError 哨兵 → 业务码（FailOf 单一渲染面——HTTPStatusOf 同口径）。
func mapUserAuthnError(err error) *errorx.Error {
	switch {
	case errors.Is(err, authn.ErrNoCredential):
		return errorx.New(errorx.CodeTokenMissing)
	case errors.Is(err, authn.ErrExpiredToken):
		return errorx.New(errorx.CodeTokenExpired)
	case errors.Is(err, authn.ErrRevokedToken):
		return errorx.New(errorx.CodeTokenRevoked)
	case errors.Is(err, authn.ErrInvalidToken), errors.Is(err, authn.ErrUnknownScheme):
		return errorx.New(errorx.CodeTokenInvalid)
	default:
		return errorx.New(errorx.CodeUnavailable)
	}
}

func bearerToken(header string) string {
	const scheme = "Bearer "
	if len(header) > len(scheme) && header[:len(scheme)] == scheme {
		return header[len(scheme):]
	}
	return ""
}

// RegisterClientRoutes 是路由注册面（module.go 调用——public/authed 分组
// 由 module 层组合，本文件只暴露 Handler）。
func RegisterHandler(svc *application.Service) gin.HandlerFunc {
	return registerHandler(ClientDeps{Service: svc})
}
func LoginHandler(svc *application.Service) gin.HandlerFunc {
	return loginHandler(ClientDeps{Service: svc})
}
func ProfileHandler(svc *application.Service) gin.HandlerFunc {
	return profileHandler(ClientDeps{Service: svc})
}
func UpdateUsernameHandler(svc *application.Service) gin.HandlerFunc {
	return updateUsernameHandler(ClientDeps{Service: svc})
}
func UpdatePhoneHandler(svc *application.Service) gin.HandlerFunc {
	return updatePhoneHandler(ClientDeps{Service: svc})
}

// registerRequest 注册请求（用户需求面：邮箱+密码——用户名随机生成）。
type registerRequest struct {
	Email    string `json:"email" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// @Summary 用户注册（邮箱+密码——随机用户名）
// @Tags users-client
// @ID users.client.register
// @Accept json
// @Param body body registerRequest true "注册凭证"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /client/v1/users/register [post]
func registerHandler(deps ClientDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req registerRequest
		if !transport.Bind(c, &req) {
			return
		}
		u, err := deps.Service.Register(c.Request.Context(), req.Email, req.Password)
		if err != nil {
			transport.FailOf(c, mapUserError(err))
			return
		}
		transport.OK(c, gin.H{"id": u.ID, "username": u.Username,
			"email": primitives.Mask("email", u.Email)})
	}
}

// loginRequest 登录请求。
type loginRequest struct {
	Email    string `json:"email" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// @Summary 用户登录（User JWT）
// @Tags users-client
// @ID users.client.login
// @Accept json
// @Param body body loginRequest true "登录凭证"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Router /client/v1/users/login [post]
func loginHandler(deps ClientDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if !transport.Bind(c, &req) {
			return
		}
		u, token, err := deps.Service.Login(c.Request.Context(), req.Email, req.Password)
		if err != nil {
			transport.FailOf(c, mapUserError(err))
			return
		}
		transport.OK(c, gin.H{"access_token": token, "token_type": "Bearer",
			"expires_in": int(application.DefaultAccessTokenTTL.Seconds()), "username": u.Username})
	}
}

// @Summary 用户视图（PII 掩码示范）
// @Tags users-client
// @ID users.client.profile
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Router /client/v1/users/me [get]
func profileHandler(deps ClientDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		u, err := deps.Service.Profile(c.Request.Context(), p.ID)
		if err != nil {
			transport.FailOf(c, err)
			return
		}
		transport.OK(c, u)
	}
}

// updateUsernameRequest 改用户名请求。
type updateUsernameRequest struct {
	Username string `json:"username" validate:"required"`
}

// @Summary 修改用户名（登录后）
// @Tags users-client
// @ID users.client.update-username
// @Accept json
// @Param body body updateUsernameRequest true "新用户名"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /client/v1/users/username [put]
func updateUsernameHandler(deps ClientDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req updateUsernameRequest
		if !transport.Bind(c, &req) {
			return
		}
		p := transport.PrincipalFrom(c)
		if err := deps.Service.UpdateUsername(c.Request.Context(), p.ID, req.Username); err != nil {
			transport.FailOf(c, mapUserError(err))
			return
		}
		transport.OK(c, gin.H{"username": req.Username})
	}
}

// updatePhoneRequest 改手机号请求（可空=清除）。
type updatePhoneRequest struct {
	Phone string `json:"phone"`
}

// @Summary 修改手机号（登录后）
// @Tags users-client
// @ID users.client.update-phone
// @Accept json
// @Param body body updatePhoneRequest true "新手机号（空=清除）"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /client/v1/users/phone [put]
func updatePhoneHandler(deps ClientDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req updatePhoneRequest
		if !transport.Bind(c, &req) {
			return
		}
		p := transport.PrincipalFrom(c)
		if err := deps.Service.UpdatePhone(c.Request.Context(), p.ID, req.Phone); err != nil {
			transport.FailOf(c, mapUserError(err))
			return
		}
		transport.OK(c, gin.H{})
	}
}
