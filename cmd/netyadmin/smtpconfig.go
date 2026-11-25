// smtpconfig.go：SMTP 运行期配置（admin「系统管理 → 邮件设置」）。
//
// 存储=configcenter（group=message，键契约见 configcenter/api/keys）：复用既有
// 版本/历史/回滚/事务/广播/操作审计面；启动期 config.yaml 为回退默认值；密码
// 始终 env-only（NETYADMIN_MESSAGE_SMTP_PASSWORD，不进 configcenter）。
// 生效=configcenter reload 回调重建 SMTP 驱动（wireMessageSMTPConfig）——
// 多节点经广播各自收敛；无需重启。
package main

import (
	"context"
	"errors"
	"net/mail"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/netyilei/netyadmin/internal/kernel/config"
	"github.com/netyilei/netyadmin/internal/kernel/errorx"
	"github.com/netyilei/netyadmin/internal/kernel/log"
	"github.com/netyilei/netyadmin/internal/kernel/security/authn"
	"github.com/netyilei/netyadmin/internal/kernel/security/authz"
	"github.com/netyilei/netyadmin/internal/kernel/transport"
	ckeys "github.com/netyilei/netyadmin/internal/modules/configcenter/api/keys"
	ccapplication "github.com/netyilei/netyadmin/internal/modules/configcenter/application"
	ccdomain "github.com/netyilei/netyadmin/internal/modules/configcenter/domain"
	"github.com/netyilei/netyadmin/internal/modules/message"
	msgapp "github.com/netyilei/netyadmin/internal/modules/message/application"
)

// SMTP 配置权限码（与 rbac_apis 种子对偶——check-rbac-perms）。
const (
	smtpPermView    = "admin.settings.smtp.view"
	smtpPermOperate = "admin.settings.smtp.operate"
)

// smtpValues 是生效 SMTP 参数（密码不在内——env-only）。
type smtpValues struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	From     string `json:"from"`
}

// smtpView 是 GET/PUT 响应。
type smtpView struct {
	smtpValues
	PasswordSet  bool   `json:"password_set"`  // 密码是否已由 env 注入
	Source       string `json:"source"`        // config（启动值）| configcenter（有覆盖）
	ActiveDriver string `json:"active_driver"` // email 通道当前驱动（smtp）
}

// SMTPDeps 是 SMTP 配置面依赖（cmd 组合根持有；ccSvc=configcenter 写面，
// 与 taskcenter 暂停态同惯例）。
type SMTPDeps struct {
	Config  *config.Config
	Configs *ccapplication.Service
	Message *message.Module
}

// smtpTestRequest 是测试邮件请求体。
type smtpTestRequest struct {
	To string `json:"to"`
}

// RegisterSMTPConfigRoutes 注册 SMTP 配置端点（authed 组；message 模块启用才挂）。
func RegisterSMTPConfigRoutes(authed gin.IRouter, deps SMTPDeps,
	authorizer authz.Authorizer, onDeny func(*gin.Context, authn.Principal, string, string)) {

	authzMW := transport.AuthzMiddleware(authorizer, transport.PrincipalFrom, onDeny)
	secured := func(perm string, h gin.HandlerFunc) []gin.HandlerFunc {
		return transport.Secured(authzMW, perm, h)
	}
	grp := authed.Group("/system/smtp")
	grp.GET("", secured(smtpPermView, smtpGetHandler(deps))...)
	grp.PUT("", secured(smtpPermOperate, smtpPutHandler(deps))...)
	grp.DELETE("", secured(smtpPermOperate, smtpResetHandler(deps))...)
	grp.POST("/test", secured(smtpPermOperate, smtpTestHandler(deps))...)
}

// smtpGetHandler 读取生效配置（configcenter 覆盖 > 启动期 config）。
// @Summary 读取 SMTP 配置（生效值：configcenter 覆盖优先，回退启动配置）
// @Tags message
// @ID admin.system.smtp.get
// @Produce json
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /admin/v1/system/smtp [get]
func smtpGetHandler(deps SMTPDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		vals, overridden, err := effectiveSMTP(c.Request.Context(), deps.Configs, deps.Config.Message)
		if err != nil {
			transport.FailOf(c, err)
			return
		}
		transport.OK(c, smtpViewOf(deps, vals, overridden))
	}
}

// smtpPutHandler 保存配置（configcenter 单事务批量写；提交后广播→各节点重建驱动）。
// @Summary 保存 SMTP 配置（configcenter 写入 + 热更生效；密码 env-only 不入库）
// @Tags message
// @ID admin.system.smtp.update
// @Accept json
// @Produce json
// @Param body body smtpValues true "SMTP 参数（host/port/username/from）"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /admin/v1/system/smtp [put]
func smtpPutHandler(deps SMTPDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req smtpValues
		if err := c.ShouldBindJSON(&req); err != nil {
			transport.FailOf(c, errorx.New(errorx.CodeInvalidParams, "reason", "invalid json body"))
			return
		}
		vals, verr := normalizeSMTP(req)
		if verr != nil {
			transport.FailOf(c, verr)
			return
		}
		op := transport.PrincipalFrom(c).ID
		if err := deps.Configs.BatchUpsert(c.Request.Context(), smtpConfigItems(vals), op, "admin smtp settings"); err != nil {
			transport.FailOf(c, err)
			return
		}
		// 本节点即时生效（广播回调幂等——免等 Bus 往返）；其他节点由广播收敛。
		applySMTP(deps.Config.Message, vals, deps.Message)
		transport.OK(c, smtpViewOf(deps, vals, true))
	}
}

// smtpResetHandler 清除覆盖（删除 4 个键→回退启动期 config）。
// @Summary 重置 SMTP 配置（删除覆盖键→回退启动配置）
// @Tags message
// @ID admin.system.smtp.reset
// @Produce json
// @Success 200 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /admin/v1/system/smtp [delete]
func smtpResetHandler(deps SMTPDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		op := transport.PrincipalFrom(c).ID
		for _, key := range ckeys.SMTPKeys() {
			err := deps.Configs.Delete(c.Request.Context(), ckeys.GroupMessage, key, op, "admin smtp settings reset")
			if err != nil && !errors.Is(err, ccdomain.ErrNotFound) {
				transport.FailOf(c, err)
				return
			}
		}
		vals := smtpValuesFromConfig(deps.Config.Message)
		applySMTP(deps.Config.Message, vals, deps.Message)
		transport.OK(c, smtpViewOf(deps, vals, false))
	}
}

// smtpTestHandler 用当前生效配置发送测试邮件（保存后可即时验证联通）。
// @Summary 发送测试邮件（使用当前生效 SMTP 配置）
// @Tags message
// @ID admin.system.smtp.test
// @Accept json
// @Produce json
// @Param body body smtpTestRequest true "收件人邮箱（to）"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /admin/v1/system/smtp/test [post]
func smtpTestHandler(deps SMTPDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req smtpTestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			transport.FailOf(c, errorx.New(errorx.CodeInvalidParams, "reason", "invalid json body"))
			return
		}
		to, err := validEmail(req.To)
		if err != nil {
			transport.FailOf(c, err)
			return
		}
		// 以配置中心最新值为准（不依赖广播时序——保存后立即测试也正确）。
		vals, _, err := effectiveSMTP(c.Request.Context(), deps.Configs, deps.Config.Message)
		if err != nil {
			transport.FailOf(c, err)
			return
		}
		applySMTP(deps.Config.Message, vals, deps.Message)
		if err := deps.Message.Service().SendTest(c.Request.Context(), to); err != nil {
			transport.FailOf(c, errorx.Wrap(errorx.CodeUnavailable, err, "reason", "smtp send failed"))
			return
		}
		transport.OK(c, gin.H{"sent": true, "to": to})
	}
}

// smtpViewOf 组装响应视图。
func smtpViewOf(deps SMTPDeps, vals smtpValues, overridden bool) smtpView {
	source := "config"
	if overridden {
		source = "configcenter"
	}
	driver := ""
	if deps.Message != nil && deps.Message.Service() != nil {
		driver = deps.Message.Service().ActiveEmailDriver()
	}
	return smtpView{
		smtpValues:   vals,
		PasswordSet:  deps.Config.Message.SMTPPassword.IsSet(),
		Source:       source,
		ActiveDriver: driver,
	}
}

// smtpValuesFromConfig 启动期回退值（config.yaml message.smtp）。
func smtpValuesFromConfig(cfg config.Message) smtpValues {
	return smtpValues{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		Username: cfg.SMTP.Username,
		From:     cfg.SMTP.From,
	}
}

// effectiveSMTP 从 configcenter 读覆盖值（整组一次查询），缺省回退启动值。
// overridden 表示至少一个键来自 configcenter（页面显示生效源）。
func effectiveSMTP(ctx context.Context, cc *ccapplication.Service, startup config.Message) (smtpValues, bool, error) {
	all, err := cc.All(ctx)
	if err != nil {
		return smtpValues{}, false, err
	}
	vals, overridden := effectiveSMTPFromConfigs(all, startup)
	return vals, overridden, nil
}

// effectiveSMTPFromConfigs 纯函数：合并 configcenter 行与启动值（测试面）。
func effectiveSMTPFromConfigs(all []ccdomain.Config, startup config.Message) (smtpValues, bool) {
	vals := smtpValuesFromConfig(startup)
	overridden := false
	for _, row := range all {
		if row.GroupName != ckeys.GroupMessage {
			continue
		}
		switch row.ConfigKey {
		case ckeys.SMTPHostKey:
			vals.Host, overridden = row.ConfigValue, true
		case ckeys.SMTPPortKey:
			if p, err := strconv.Atoi(row.ConfigValue); err == nil {
				vals.Port, overridden = p, true
			}
		case ckeys.SMTPUsernameKey:
			vals.Username, overridden = row.ConfigValue, true
		case ckeys.SMTPFromKey:
			vals.From, overridden = row.ConfigValue, true
		}
	}
	return vals, overridden
}

// effectiveFromSnapshot 从 Watcher 快照合并（热更回调路径——不查 DB）。
func effectiveFromSnapshot(snap map[string]ccdomain.Config, startup config.Message) (smtpValues, bool) {
	rows := make([]ccdomain.Config, 0, len(ckeys.SMTPKeys()))
	for _, key := range ckeys.SMTPKeys() {
		if row, ok := snap[ckeys.SnapshotKey(ckeys.GroupMessage, key)]; ok {
			rows = append(rows, row)
		}
	}
	return effectiveSMTPFromConfigs(rows, startup)
}

// smtpConfigItems 构造批量写入行（单事务/单版本/单广播）。
func smtpConfigItems(v smtpValues) []ccdomain.Config {
	items := []ccdomain.Config{
		{GroupName: ckeys.GroupMessage, ConfigKey: ckeys.SMTPHostKey, ConfigValue: v.Host,
			ValueType: ccdomain.TypeString, Description: "SMTP 服务器地址（admin 邮件设置）"},
		{GroupName: ckeys.GroupMessage, ConfigKey: ckeys.SMTPPortKey, ConfigValue: strconv.Itoa(v.Port),
			ValueType: ccdomain.TypeString, Description: "SMTP 端口（admin 邮件设置）"},
		{GroupName: ckeys.GroupMessage, ConfigKey: ckeys.SMTPUsernameKey, ConfigValue: v.Username,
			ValueType: ccdomain.TypeString, Description: "SMTP 用户名（密码由 env NETYADMIN_MESSAGE_SMTP_PASSWORD 提供）"},
		{GroupName: ckeys.GroupMessage, ConfigKey: ckeys.SMTPFromKey, ConfigValue: v.From,
			ValueType: ccdomain.TypeString, Description: "发件人地址（admin 邮件设置）"},
	}
	return items
}

// normalizeSMTP 校验并归一化输入（host/port/from 必填；from 必须为纯邮箱地址）。
func normalizeSMTP(in smtpValues) (smtpValues, error) {
	v := smtpValues{
		Host:     strings.TrimSpace(in.Host),
		Port:     in.Port,
		Username: strings.TrimSpace(in.Username),
		From:     strings.TrimSpace(in.From),
	}
	if v.Host == "" {
		return smtpValues{}, errorx.New(errorx.CodeInvalidParams, "reason", "smtp host required")
	}
	if v.Port < 1 || v.Port > 65535 {
		return smtpValues{}, errorx.New(errorx.CodeInvalidParams, "reason", "smtp port must be 1..65535")
	}
	from, err := validEmail(v.From)
	if err != nil {
		return smtpValues{}, err
	}
	v.From = from
	return v, nil
}

// validEmail 解析并归一化邮箱（只接受纯地址，拒绝显示名——SMTP From 头安全）。
func validEmail(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	addr, err := mail.ParseAddress(raw)
	if err != nil || addr.Address != raw {
		return "", errorx.New(errorx.CodeInvalidParams, "reason", "invalid email address")
	}
	return addr.Address, nil
}

// applySMTP 用启动值（密码 env-only）与生效参数重建 SMTP 驱动。
func applySMTP(startup config.Message, vals smtpValues, msg *message.Module) {
	if msg == nil || msg.Service() == nil {
		return
	}
	msg.Service().ApplySMTP(msgapp.SMTPConfig{
		Host:     vals.Host,
		Port:     vals.Port,
		Username: vals.Username,
		From:     vals.From,
		Password: startup.SMTPPassword.Value(),
	})
}

// wireMessageSMTPConfig 热更消费：configcenter reload（启动装载/广播/回滚）后
// 按快照重建 SMTP 驱动——各节点一致；无覆盖键=回退启动配置。
func wireMessageSMTPConfig(w *ccapplication.Watcher, msg *message.Module, cfg *config.Config, logMgr *log.Manager) {
	if w == nil || msg == nil {
		return
	}
	logger := logMgr.Logger("message", "app")
	w.OnChange(func() {
		vals, overridden := effectiveFromSnapshot(w.Snapshot(), cfg.Message)
		applySMTP(cfg.Message, vals, msg)
		if overridden {
			logger.Info("message: smtp config applied from configcenter",
				"host", vals.Host, "port", vals.Port)
		}
	})
}
