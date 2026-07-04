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
	CodeRequestTimeout  Code = 100011

	CodeUserNotFound      Code = 101001
	CodeUserDisabled      Code = 101002
	CodePasswordWrong     Code = 101003
	CodeUserAlreadyExists Code = 101004
	CodeTokenExpired      Code = 101005
	CodeTokenInvalid      Code = 101006
	CodeUserLocked        Code = 101007
	CodeOldPasswordWrong  Code = 101008
	CodePasswordTooWeak   Code = 101009

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
	CodeAppDisabled     Code = 101306

	// IP 访问控制 (1014xx)
	CodeIPBlocked     Code = 101401
	CodeIPInvalid     Code = 101402
	CodeWhitelistMode Code = 101403

	// 存储上传 (1015xx)
	CodeUploadRecordNotFound   Code = 101501
	CodeUploadSignatureInvalid Code = 101502
	CodeUploadRecordCompleted  Code = 101503
	CodeUploadRecordExpired    Code = 101504
	CodeUploadRecordMismatch   Code = 101505

	// 任务调度模块 (109xxx)
	CodeTaskNotFound       Code = 109005
	CodeTaskAlreadyRunning Code = 109006
	CodeTaskNotRunning     Code = 109007
	CodeEmailTestFailed    Code = 109008

	// 内容模块 (1016xx)
	CodeCategoryHasArticles Code = 101601
	CodeCategoryHasChildren Code = 101602
	CodeBannerGroupHasItems Code = 101603

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
	CodeRequestTimeout:  "请求超时",

	CodeUserNotFound:      "用户不存在",
	CodeUserDisabled:      "用户已禁用",
	CodePasswordWrong:     "密码错误",
	CodeUserAlreadyExists: "用户名已存在",
	CodeTokenExpired:      "令牌已过期",
	CodeTokenInvalid:      "令牌无效",
	CodeUserLocked:        "账户已锁定",
	CodeOldPasswordWrong:  "原密码错误",
	CodePasswordTooWeak:   "密码强度不足",

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

	// 消息模块 (1012xx)
	CodeMsgTemplateNotFound:   "消息模板不存在",
	CodeMsgTemplateCodeExists: "模板编码已存在",
	CodeMsgSendFailed:         "消息发送失败",
	CodeMsgRecordNotFound:     "消息记录不存在",
	CodeMsgDriverNotFound:     "消息驱动未配置或不存在",

	// 系统配置模块 (1090xx)
	CodeEmailTestFailed: "邮件测试发送失败",

	// 任务调度模块 (109xxx)
	CodeTaskNotFound:       "任务不存在",
	CodeTaskAlreadyRunning: "任务已在运行中",
	CodeTaskNotRunning:     "任务未在运行",

	// 开放平台 (1013xx)
	CodeAppKeyInvalid:   "AppKey无效",
	CodeSignatureFailed: "签名验证失败",
	CodeRequestExpired:  "请求已过期",
	CodeScopeMismatch:   "权限不足 (Scope 不匹配)",
	CodeRateLimited:     "已触发流量限制",
	CodeAppDisabled:     "应用已被禁用",

	// IP 访问控制 (1014xx)
	CodeIPBlocked:     "访问受限 (您的 IP 已被封锁)",
	CodeIPInvalid:     "非法 IP/CIDR 格式",
	CodeWhitelistMode: "系统处于白名单模式，您的 IP 未被授权",

	// 存储上传 (1015xx)
	CodeUploadRecordNotFound:   "上传记录不存在",
	CodeUploadSignatureInvalid: "上传凭证校验失败",
	CodeUploadRecordCompleted:  "该上传记录已完成，不可重复提交",
	CodeUploadRecordExpired:    "上传凭证已过期",
	CodeUploadRecordMismatch:   "上传记录与请求不匹配",

	// 内容模块 (1016xx)
	CodeCategoryHasArticles: "该分类下存在文章，请先移走或删除文章后再删除分类",
	CodeCategoryHasChildren: "该分类下存在子分类，请先删除子分类后再删除父分类",
	CodeBannerGroupHasItems: "该 Banner 组下存在 Banner 项，请先移走或删除 Banner 项后再删除组",

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
	return fmt.Sprintf("%06d", int(c))
}

// BizError 表示业务错误，携带业务码 + 用户可见消息 + 可选的原始错误链。
// Err 字段用于保留 Repository / 第三方库返回的原始错误（如 gorm.ErrRecordNotFound），
// 便于 Sentry 上报与日志排查，同时不污染返回给前端的 Message。
type BizError struct {
	Code    Code
	Message string
	Err     error
}

func New(code Code, message ...string) *BizError {
	msg := code.Message()
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	return &BizError{Code: code, Message: msg}
}

// NewWithErr 构造一个携带原始错误链的 BizError。
// 用于 Service 层在捕获 Repository 错误时，既要映射业务码，
// 又要保留原始错误（如 gorm.ErrRecordNotFound）以便 Sentry/日志排查。
// 当 err 为 nil 时等价于 New(code, message...)。
func NewWithErr(code Code, err error, message ...string) *BizError {
	bizErr := New(code, message...)
	bizErr.Err = err
	return bizErr
}

func (e *BizError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Is 支持 errors.Is 比较：当 target 也是 *BizError 且业务码相同时返回 true。
// 这使得 errors.Is(wrappedBizErr, errorx.New(CodeXxx)) 可以穿透 fmt.Errorf("%w") 包装链。
func (e *BizError) Is(target error) bool {
	if t, ok := target.(*BizError); ok {
		return e.Code == t.Code
	}
	return false
}

// Unwrap 返回被包装的原始错误，支持 errors.As / errors.Is 穿透 BizError 链路。
func (e *BizError) Unwrap() error {
	return e.Err
}
