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
	CodeOldPasswordWrong  Code = 101007

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
