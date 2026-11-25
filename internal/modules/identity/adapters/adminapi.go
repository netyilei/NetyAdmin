// adminapi.go：管理员身份 HTTP 端点（V1 auth 7 端点对齐）——
// identity 模块的 HTTP 适配（gin handlers 调 application.Service）。模块自注册
// 模式：cmd 组装时调用 RegisterAdminRoutes 挂到 adminapi 版本组，
// adminapi 门不 import 本模块。
// 鉴权链（JWT 验签→吊销→TokenVersion→sid 提取）由装配层中间件完成，
// handlers 经 gin context 取 principal。登录审计：SecurityRecord 经 Bus 事件
// 直写（发布在 application，订阅在 audit——记录唯一归属）。
// 首个 swag 注解端点（TR-04 触发点）：swag init 生成随收尾（门禁 SKIP 态）。
package adapters

import (
	"errors"
	"net/mail"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/security/captcha"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/identity/api/keys"
	"github.com/netyilei/netyadmin/internal/modules/identity/application"
	"github.com/netyilei/netyadmin/internal/modules/identity/domain"
)

// Deps 是 handlers 依赖（cmd 装配注入）。
// authn 面=AuthMiddleware(verifier) 组级；authz 面=RegisterAuthedRoutes 的
// authorizer/onDeny 参数——本结构只承担 handler 直依赖。
type Deps struct {
	Service   *application.Service
	RateLimit gin.HandlerFunc  // 公开端点限流（login/refresh；nil=不限——装配缺失登记面）
	Captcha   *captcha.Captcha // 登录验证码面（补强——nil=装配缺失，登录 fail-closed 503）
}

// RegisterPublicRoutes 注册公开 auth 端点（login/refresh/captcha）。
// 限流由装配方经 Deps.RateLimit 注入（公开 API 必须限流——security.md §5；
// nil=直通，登记面）。
func RegisterPublicRoutes(public gin.IRouter, deps Deps) {
	mws := make([]gin.HandlerFunc, 0, 1)
	if deps.RateLimit != nil {
		mws = append(mws, deps.RateLimit)
	}
	auth := public.Group("", mws...)
	auth.POST("/login", loginHandler(deps))
	auth.POST("/refresh-token", refreshHandler(deps))
	auth.GET("/captcha", captchaHandler(deps))
}

// RegisterAuthedRoutes 注册受保护端点（自身面 + 管理面）。
// authed 组由装配方挂 AuthMiddleware（authn 组级——rbac/audit 端点同组同链，
// 公开端点不在此组）；权限链 = RequirePerm（声明）→ AuthzMiddleware（判定）。
func RegisterAuthedRoutes(authed gin.IRouter, deps Deps,
	authorizer authz.Authorizer, onDeny func(*gin.Context, authn.Principal, string, string)) {

	authzMW := transport.AuthzMiddleware(authorizer, transport.PrincipalFrom, onDeny)
	secured := func(perm string, h gin.HandlerFunc) []gin.HandlerFunc {
		return transport.Secured(authzMW, perm, h)
	}

	authed.POST("/auth/logout", logoutHandler(deps))
	authed.GET("/auth/user-info", profileHandler(deps))
	authed.GET("/auth/profile", profileHandler(deps))
	authed.PUT("/auth/profile", updateProfileHandler(deps))
	authed.POST("/auth/change-password", changePasswordHandler(deps))

	// 管理员账号管理（V1 /admins 端点集 + 重置密码）。
	admins := authed.Group("/admins")
	admins.GET("", secured(keys.PermAdminsList, listAdminsHandler(deps))...)
	admins.POST("", secured(keys.PermAdminsCreate, createAdminHandler(deps))...)
	admins.POST("/batch-delete", secured(keys.PermAdminsDelete, batchDeleteAdminsHandler(deps))...)
	admins.GET("/:id", secured(keys.PermAdminsGet, getAdminHandler(deps))...)
	admins.PUT("/:id", secured(keys.PermAdminsUpdate, updateAdminHandler(deps))...)
	admins.PATCH("/:id/status", secured(keys.PermAdminsStatus, setAdminStatusHandler(deps))...)
	admins.POST("/:id/reset-password", secured(keys.PermAdminsResetPassword, resetPasswordHandler(deps))...)
	admins.DELETE("/:id", secured(keys.PermAdminsDelete, deleteAdminHandler(deps))...)
}

// AuthMiddleware 是 adminapi authn 中间件（cmd 组级挂载——authed 子树统一链；
// rbac/audit 端点同组受益，公开端点不进该组）。JWT 验签→吊销→TokenVersion→
// Principal 注入 context；authn 哨兵细分映射（10101-10104——rpc server 同口径）。
func AuthMiddleware(verifier *authn.JWTVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, err := verifier.Authenticate(c.Request.Context(),
			authn.Credential{Token: bearerToken(c.GetHeader("Authorization"))})
		if err != nil {
			// FailOf 单一渲染面：HTTPStatusOf 与 mapAuthnError 同口径（10101-10104
			// →401010→503），顺带补 X-Request-ID/request_id/trace_id 与审计
			// biz_code 注入（原直写 c.JSON 缺这些——access 审计 BizCode 空根因）。
			transport.FailOf(c, mapAuthnError(err))
			return
		}
		c.Set("principal", principal)
		// ctx 注入（audit D②根修 2026-11-21）：request hook payload/rpc/bus 传播面
		// 读 ctx 版 authn.PrincipalFrom——gin key 仅 gin 侧可见，双面注入单一事实。
		c.Request = c.Request.WithContext(authn.WithPrincipal(c.Request.Context(), principal))
		c.Next()
	}
}

// mapAuthnError 映射验证链哨兵 → 业务码（HTTP 传输语义由
// errorx.HTTPStatusOf 统一给出——10101-10104→401010→503 同口径）。
func mapAuthnError(err error) *errorx.Error {
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

// mapIdentityError 把 identity 领域错误映射为 HTTP 语义（errorx 信封）：
// 防枚举面（invalid credentials）→ 10005；会话面 → 10104；禁用 → 10105；
// 强度 → 10002；不存在 → 10003；系统面 → 10010。
func mapIdentityError(err error) *errorx.Error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrInvalidCredentials):
		return errorx.New(errorx.CodeUnauthorized)
	case errors.Is(err, domain.ErrSessionDenied):
		return errorx.New(errorx.CodeUnauthorized)
	case errors.Is(err, domain.ErrAccountDisabled):
		return errorx.New(errorx.CodeAccountDisabled)
	case errors.Is(err, domain.ErrPasswordWeak), errors.Is(err, domain.ErrAvatarTooLarge), errors.Is(err, domain.ErrPhoneInvalid):
		return errorx.New(errorx.CodeValidationFailed)
	case errors.Is(err, domain.ErrUsernameTaken), errors.Is(err, domain.ErrNotFound):
		return errorx.New(errorx.CodeNotFound)
	case errors.Is(err, domain.ErrSelfOperation):
		return errorx.New(errorx.CodeForbidden)
	case errors.Is(err, domain.ErrStoreUnavailable):
		return errorx.New(errorx.CodeUnavailable)
	default:
		return errorx.New(errorx.CodeInternal)
	}
}

func writeIdentityError(c *gin.Context, err error) {
	// FailOf 单一渲染面（HTTPStatusOf 与映射表同口径——10005104→401、
	// 10105006→403、10002→400、10003→404、10010→503）；顺带补
	// X-Request-ID/request_id/trace_id 与审计 biz_code 注入（原直写缺失）。
	transport.FailOf(c, mapIdentityError(err))
}

func bearerToken(header string) string {
	const scheme = "Bearer "
	if len(header) > len(scheme) && header[:len(scheme)] == scheme {
		return header[len(scheme):]
	}
	return header
}

type loginRequest struct {
	Username    string `json:"username" validate:"required"`
	Password    string `json:"password" validate:"required"`
	CaptchaID   string `json:"captchaId" validate:"required"`
	CaptchaCode string `json:"captchaCode" validate:"required"`
}

// @Summary 管理员登录
// @ID admin.auth.login
// @Description 用户名密码换取 access/refresh token（login_ip 限流由接入面承担）
// @Tags auth
// @Accept json
// @Produce json
// @Param body body loginRequest true "登录凭证"
// @Success 200 {object} application.LoginResult
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 429 {object} errorx.Envelope
// @Router /admin/v1/auth/login [post]
func loginHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if !transport.Bind(c, &req) {
			return
		}
		// 验证码先于凭证校验（一次性 GETDEL 消费——填错即失效须刷新重取；
		// Captcha 未装配=登录 fail-closed 503，装配缺失不可静默放行）
		if deps.Captcha == nil {
			transport.Fail(c, errorx.New(errorx.CodeUnavailable, "reason", "captcha not wired"))
			return
		}
		if !deps.Captcha.Verify(c.Request.Context(), req.CaptchaID, req.CaptchaCode) {
			transport.Fail(c, errorx.New(errorx.CodeValidationFailed, "reason", "captcha invalid"))
			return
		}
		res, err := deps.Service.Login(c.Request.Context(), req.Username, req.Password)
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, res)
	}
}

// @Summary 获取登录验证码
// @ID admin.auth.captcha
// @Description 英数组合（剔除易混淆字符），一次性 3 分钟有效
// @Tags auth
// @Produce json
// @Success 200 {object} errorx.Envelope
// @Router /admin/v1/auth/captcha [get]
func captchaHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Captcha == nil {
			transport.Fail(c, errorx.New(errorx.CodeUnavailable, "reason", "captcha not wired"))
			return
		}
		id, imageB64, err := deps.Captcha.Generate(c.Request.Context())
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, gin.H{"captchaId": id, "image": imageB64})
	}
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// @Summary 刷新 token
// @ID admin.auth.refresh_token
// @Description refresh token 一次性轮换
// @Tags auth
// @Accept json
// @Produce json
// @Param body body refreshRequest true "刷新令牌"
// @Success 200 {object} application.LoginResult
// @Failure 401 {object} errorx.Envelope
// @Failure 429 {object} errorx.Envelope
// @Router /admin/v1/auth/refresh-token [post]
func refreshHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req refreshRequest
		if !transport.Bind(c, &req) {
			return
		}
		res, err := deps.Service.Refresh(c.Request.Context(), req.RefreshToken)
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, res)
	}
}

// @Summary 登出
// @ID admin.auth.logout
// @Description 吊销当前 access（jti）并杀会话（sid）
// @Tags auth
// @Produce json
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Router /admin/v1/auth/logout [post]
func logoutHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		if err := deps.Service.Logout(c.Request.Context(), p.JTI, p.Expires, p.SID); err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

// @Summary 当前管理员资料（user-info——登录后回填 userStore）
// @Tags auth
// @ID admin.auth.user-info
// @Produce json
// @Success 200 {object} errorx.Envelope{data=domain.Profile}
// @Failure 401 {object} errorx.Envelope
// @Router /admin/v1/auth/user-info [get]
func profileHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		profile, err := deps.Service.GetProfile(c.Request.Context(), p.ID)
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, profile)
	}
}

type updateProfileRequest struct {
	Name      string `json:"name" validate:"max=64"`
	Email     string `json:"email" validate:"omitempty,email,max=128"`
	Phone     string `json:"phone" validate:"omitempty,max=20"`
	AvatarURL string `json:"avatarUrl" validate:"omitempty,max=300000"` // data URL（≤200KB）或对象存储 URL
}

// @Summary 更新个人信息
// @Tags auth
// @ID admin.auth.profile_update
// @Accept json
// @Produce json
// @Param body body updateProfileRequest true "姓名与邮箱"
// @Success 200 {object} domain.Profile
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Router /admin/v1/auth/profile [put]
func updateProfileHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		var req updateProfileRequest
		if !transport.Bind(c, &req) {
			return
		}
		profile, err := deps.Service.UpdateProfile(c.Request.Context(), p.ID, req.Name, req.Email, req.Phone, req.AvatarURL)
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, profile)
	}
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=72"`
}

// @Summary 修改密码
// @ID admin.auth.change_password
// @Description 旧密码校验 + 新密码强度校验 + TokenVersion 全作废
// @Tags auth
// @Accept json
// @Produce json
// @Param body body changePasswordRequest true "新旧密码"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Router /admin/v1/auth/change-password [post]
func changePasswordHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		var req changePasswordRequest
		if !transport.Bind(c, &req) {
			return
		}
		if err := deps.Service.ChangePassword(c.Request.Context(),
			p.ID, req.OldPassword, req.NewPassword); err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

// —— 管理员账号管理——

// @Summary 管理员列表
// @ID admin.admins.list
// @Tags admins
// @Produce json
// @Param username query string false "用户名前缀过滤"
// @Param name query string false "姓名包含过滤"
// @Param status query int false "状态过滤（1=active 2=disabled）"
// @Param page query int false "页码（1 起）"
// @Param page_size query int false "页大小（默认 20 上限 100）"
// @Success 200 {object} transport.Page
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/admins [get]
func listAdminsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, size := transport.BindPage(c)
		status := 0
		if v, err := strconv.Atoi(c.Query("status")); err == nil {
			if st := domain.Status(v); v != 0 && !st.Valid() {
				transport.FailOf(c, errorx.New(errorx.CodeValidationFailed)) // 非法枚举=400
				return
			}
			status = v
		}
		list, total, err := deps.Service.ListAdmins(c.Request.Context(), domain.ListFilter{
			Username: c.Query("username"),
			Name:     c.Query("name"),
			Status:   domain.Status(status),
			Offset:   transport.Offset(page, size),
			Limit:    size,
		})
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, transport.NewPage(list, total, page, size))
	}
}

// @Summary 管理员详情
// @ID admin.admins.get
// @Tags admins
// @Produce json
// @Param id path string true "管理员 ID"
// @Success 200 {object} domain.Profile
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/admins/{id} [get]
func getAdminHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		profile, err := deps.Service.GetAdmin(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, profile)
	}
}

type createAdminRequest struct {
	Username string `json:"username" validate:"required,min=3,max=32"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	Name     string `json:"name" validate:"max=64"`
	Email    string `json:"email" validate:"omitempty,email,max=128"`
}

// @Summary 创建管理员
// @ID admin.admins.create
// @Description 初始密码一次性（must_change_password 强制首登改密）
// @Tags admins
// @Accept json
// @Produce json
// @Param body body createAdminRequest true "账号与初始密码"
// @Success 200 {object} domain.Profile
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /admin/v1/admins [post]
func createAdminHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createAdminRequest
		if !transport.Bind(c, &req) {
			return
		}
		profile, err := deps.Service.CreateAdmin(c.Request.Context(),
			req.Username, req.Password, req.Name, req.Email)
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, profile)
	}
}

type updateAdminRequest struct {
	Name  string  `json:"name" validate:"max=64"`
	Email *string `json:"email" validate:"omitempty,max=128"` // nil=不改该列（列表已掩码——未取原文不回写）；空串=清除；格式见 handler 校验
	Phone *string `json:"phone" validate:"omitempty,max=20"`  // nil=不改该列
}

// normalizeAdminEmail 校验/规范化管理面 email 入参（*string：nil=不改；空串=清除）。
// validator 的 omitempty 对非 nil 指针不跳过 email 标签（空串清除会误判 400），
// 故格式校验在此收口（net/mail 标准库——禁自写正则第二份）。
func normalizeAdminEmail(v *string) (*string, bool) {
	if v == nil {
		return nil, true
	}
	email := strings.TrimSpace(*v)
	if email == "" {
		return &email, true // 显式清除
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return nil, false
	}
	return &addr.Address, true
}

// @Summary 更新管理员资料
// @ID admin.admins.update
// @Tags admins
// @Accept json
// @Produce json
// @Param id path string true "管理员 ID"
// @Param body body updateAdminRequest true "姓名与邮箱"
// @Success 200 {object} domain.Profile
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/admins/{id} [put]
func updateAdminHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req updateAdminRequest
		if !transport.Bind(c, &req) {
			return
		}
		email, ok := normalizeAdminEmail(req.Email)
		if !ok {
			transport.FailOf(c, errorx.New(errorx.CodeValidationFailed))
			return
		}
		profile, err := deps.Service.UpdateAdminFace(c.Request.Context(),
			c.Param("id"), req.Name, email, req.Phone)
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, profile)
	}
}

type setStatusRequest struct {
	Status int `json:"status" validate:"required,eq=1|eq=2"`
}

// @Summary 管理员启停
// @ID admin.admins.status
// @Description 禁用 = TokenVersion++（该账号全部 token 即时作废）
// @Tags admins
// @Accept json
// @Produce json
// @Param id path string true "管理员 ID"
// @Param body body setStatusRequest true "目标状态"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/admins/{id}/status [patch]
func setAdminStatusHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		var req setStatusRequest
		if !transport.Bind(c, &req) {
			return
		}
		if err := deps.Service.SetAdminStatus(c.Request.Context(),
			p.ID, c.Param("id"), domain.Status(req.Status)); err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

type resetPasswordResult struct {
	Password string `json:"password"` // 一次性交付（响应通道唯一，不落日志）
}

// @Summary 重置密码
// @ID admin.admins.reset_password
// @Description 服务端生成随机密码一次性返回 + 强制首登改密 + 全会话作废
// @Tags admins
// @Produce json
// @Param id path string true "管理员 ID"
// @Success 200 {object} resetPasswordResult
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/admins/{id}/reset-password [post]
func resetPasswordHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		password, err := deps.Service.ResetAdminPassword(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, resetPasswordResult{Password: password})
	}
}

// @Summary 删除管理员
// @ID admin.admins.delete
// @Tags admins
// @Produce json
// @Param id path string true "管理员 ID"
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 404 {object} errorx.Envelope
// @Router /admin/v1/admins/{id} [delete]
func deleteAdminHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		if err := deps.Service.DeleteAdmins(c.Request.Context(),
			p.ID, []string{c.Param("id")}); err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}

type batchDeleteRequest struct {
	IDs []string `json:"ids" validate:"required,gt=0,dive,required"`
}

// @Summary 批量删除管理员
// @ID admin.admins.batch_delete
// @Tags admins
// @Accept json
// @Produce json
// @Param body body batchDeleteRequest true "管理员 ID 列表"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Router /admin/v1/admins/batch-delete [post]
func batchDeleteAdminsHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := transport.PrincipalFrom(c)
		var req batchDeleteRequest
		if !transport.Bind(c, &req) {
			return
		}
		if err := deps.Service.DeleteAdmins(c.Request.Context(), p.ID, req.IDs); err != nil {
			writeIdentityError(c, err)
			return
		}
		transport.OK(c, nil)
	}
}
