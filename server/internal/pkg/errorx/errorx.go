package errorx

import "fmt"

type Code int

const (
	CodeSuccess Code = 100000

	CodeInvalidParams   Code = 100001
	CodeUnauthorized    Code = 100002
	CodeForbidden       Code = 100003
	CodeNotFound        Code = 100004
	CodeInternalError   Code = 100005
	CodeTooManyRequest  Code = 100006
	CodeBadRequest      Code = 100007
	CodeAlreadyExists   Code = 100008
	CodeCaptchaInvalid  Code = 100009
	CodeCaptchaRequired Code = 100010

	CodeUserNotFound      Code = 101001
	CodeUserDisabled      Code = 101002
	CodePasswordWrong     Code = 101003
	CodeUserAlreadyExists Code = 101004
	CodeTokenExpired      Code = 101005
	CodeTokenInvalid      Code = 101006
	CodeUserLocked        Code = 101007
	CodeOldPasswordWrong  Code = 101008

	CodeRoleNotFound      Code = 102001
	CodeRoleInUse         Code = 102002
	CodeRoleAlreadyExists Code = 102003
	CodeRoleCodeDuplicate Code = 102004
	CodeCannotDeleteSuper Code = 102005
	CodeCannotModifySuper Code = 102006

	CodeMenuNotFound       Code = 103001
	CodeMenuHasChildren    Code = 103002
	CodeMenuAlreadyExists  Code = 103003
	CodeMenuRouteDuplicate Code = 103004

	CodeButtonNotFound      Code = 104001
	CodeButtonAlreadyExists Code = 104002
	CodeButtonCodeDuplicate Code = 104003

	CodeApiNotFound      Code = 105001
	CodeApiAlreadyExists Code = 105002
	CodeApiPathDuplicate Code = 105003

	// 用户模块 (1011xx)
	CodeClientUserNotFound      Code = 101101
	CodeClientUserAlreadyExists Code = 101102
	CodeEmailAlreadyExists      Code = 101103
	CodePhoneAlreadyExists      Code = 101104

	// 消息模块 (1012xx)
	CodeMsgTemplateNotFound   Code = 101201
	CodeMsgTemplateCodeExists Code = 101202
	CodeMsgSendFailed         Code = 101203
	CodeMsgRecordNotFound     Code = 101204
	CodeMsgDriverNotFound     Code = 101205

	// 开放平台 (1013xx)
	CodeAppKeyInvalid   Code = 101301
	CodeSignatureFailed Code = 101302
	CodeRequestExpired  Code = 101303
	CodeScopeMismatch   Code = 101304
	CodeRateLimited     Code = 101305

	// IP 访问控制 (1014xx)
	CodeIPBlocked     Code = 101401
	CodeIPInvalid     Code = 101402
	CodeWhitelistMode Code = 101403

	// 任务调度模块 (109xxx)
	CodeTaskNotFound       Code = 109005
	CodeTaskAlreadyRunning Code = 109006
	CodeTaskNotRunning     Code = 109007
	CodeEmailTestFailed    Code = 109008

	// 用户模块集成 (2006xx)
	CodeCaptchaExpired          Code = 200601
	CodeCaptchaSendTooFrequent  Code = 200604
	CodeVerifyTypeNotConfigured Code = 200605
)

var codeMessages = map[Code]string{
	CodeSuccess: "操作成功",

	CodeInvalidParams:   "参数错误",
	CodeUnauthorized:    "未授权",
	CodeForbidden:       "无权限",
	CodeNotFound:        "资源不存在",
	CodeInternalError:   "服务器内部错误",
	CodeTooManyRequest:  "请求过于频繁",
	CodeBadRequest:      "请求错误",
	CodeAlreadyExists:   "资源已存在",
	CodeCaptchaInvalid:  "验证码错误",
	CodeCaptchaRequired: "验证码必填",

	CodeUserNotFound:      "用户不存在",
	CodeUserDisabled:      "用户已禁用",
	CodePasswordWrong:     "密码错误",
	CodeUserAlreadyExists: "用户名已存在",
	CodeTokenExpired:      "令牌已过期",
	CodeTokenInvalid:      "令牌无效",
	CodeUserLocked:        "账户已锁定",
	CodeOldPasswordWrong:  "原密码错误",

	CodeRoleNotFound:      "角色不存在",
	CodeRoleInUse:         "角色正在使用中",
	CodeRoleAlreadyExists: "角色已存在",
	CodeRoleCodeDuplicate: "角色编码重复",
	CodeCannotDeleteSuper: "不能删除超级管理员",
	CodeCannotModifySuper: "不能修改超级管理员",

	CodeMenuNotFound:       "菜单不存在",
	CodeMenuHasChildren:    "菜单存在子菜单",
	CodeMenuAlreadyExists:  "菜单已存在",
	CodeMenuRouteDuplicate: "菜单路由重复",

	CodeButtonNotFound:      "按钮不存在",
	CodeButtonAlreadyExists: "按钮已存在",
	CodeButtonCodeDuplicate: "按钮编码重复",

	CodeApiNotFound:      "API不存在",
	CodeApiAlreadyExists: "API已存在",
	CodeApiPathDuplicate: "API路径重复",

	// 用户模块 (1011xx)
	CodeClientUserNotFound:      "用户不存在",
	CodeClientUserAlreadyExists: "用户名已存在",
	CodeEmailAlreadyExists:      "邮箱已存在",
	CodePhoneAlreadyExists:      "手机号已存在",

	// 消息模块 (1012xx)
	CodeMsgTemplateNotFound:   "消息模板不存在",
	CodeMsgTemplateCodeExists: "模板编码已存在",
	CodeMsgSendFailed:         "消息发送失败",
	CodeMsgRecordNotFound:     "消息记录不存在",
	CodeMsgDriverNotFound:     "消息驱动未配置或不存在",

	// 系统配置模块 (1090xx)
	CodeEmailTestFailed: "邮件测试发送失败",

	// 开放平台 (1013xx)
	CodeAppKeyInvalid:   "AppKey无效",
	CodeSignatureFailed: "签名验证失败",
	CodeRequestExpired:  "请求已过期",
	CodeScopeMismatch:   "权限不足 (Scope 不匹配)",
	CodeRateLimited:     "已触发流量限制",

	// IP 访问控制 (1014xx)
	CodeIPBlocked:     "访问受限 (您的 IP 已被封禁)",
	CodeIPInvalid:     "非法 IP/CIDR 格式",
	CodeWhitelistMode: "系统处于白名单模式，您的 IP 未被授权",

	// 用户模块集成 (2006xx)
	CodeCaptchaExpired:          "验证码已过期",
	CodeCaptchaSendTooFrequent:  "发送过于频繁，请稍后再试",
	CodeVerifyTypeNotConfigured: "未配置验证方式，请联系管理员",
}

func (c Code) Message() string {
	if msg, ok := codeMessages[c]; ok {
		return msg
	}
	return "未知错误"
}

func (c Code) String() string {
	return fmt.Sprintf("%04d", int(c))
}

type BizError struct {
	Code    Code
	Message string
}

func New(code Code, message ...string) *BizError {
	msg := code.Message()
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	return &BizError{Code: code, Message: msg}
}

func (e *BizError) Error() string {
	return e.Message
}

func Is(err error, code Code) bool {
	if bizErr, ok := err.(*BizError); ok {
		return bizErr.Code == code
	}
	return false
}
