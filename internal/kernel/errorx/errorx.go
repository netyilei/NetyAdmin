package errorx

// Error 是业务错误：携带 5 位状态码与结构化上下文。
// 语义约定（api-style.md §4 / status-codes.md §8）：
//   - Code 为 5 位字符串常量（codes_gen.go 生成，禁止手写数字）；
//   - 文案不入 Error——messageKey 由前端 i18n 渲染；
//   - HTTPStatus 只表达传输语义，业务语义看 Code；
//   - 领域层错误映射到 Error 的职责在 application，domain 不 import 本包。
type Error struct {
	code    string
	details map[string]any
	cause   error
}

// New 构造业务错误；code 必须是已登记状态码常量。
// details 为键值对序列（奇数长度会 panic——编程错误，不静默）。
func New(code string, details ...any) *Error {
	assertRegistered(code)
	d := kv(details)
	return &Error{code: code, details: d}
}

// Wrap 在 cause 之上构造业务错误（用于包装底层错误同时给出对外状态码）。
func Wrap(code string, cause error, details ...any) *Error {
	assertRegistered(code)
	d := kv(details)
	return &Error{code: code, details: d, cause: cause}
}

// Code 返回 5 位状态码。
func (e *Error) Code() string { return e.code }

// MessageKey 返回 i18n 文案键（来自契约登记）。
func (e *Error) MessageKey() string { return codeMessageKey[e.code] }

// Details 返回结构化上下文（浅拷贝副本：防外层 map 赋值污染内部状态）。
func (e *Error) Details() map[string]any {
	if e.details == nil {
		return nil
	}
	out := make(map[string]any, len(e.details))
	for k, v := range e.details {
		out[k] = v
	}
	return out
}

// Unwrap 支持 errors.Is/As 链（保留底层错误）。
func (e *Error) Unwrap() error { return e.cause }

// Error 实现 error 接口：只含码与键，不含任何敏感数据。
func (e *Error) Error() string {
	s := "errorx: code=" + e.code + " message_key=" + e.MessageKey()
	if e.cause != nil {
		s += ": " + e.cause.Error()
	}
	return s
}

// HTTPStatus 该码对应的 HTTP 传输语义。
func (e *Error) HTTPStatus() int { return codeHTTP[e.code] }

// Retryable 该码是否可重试（reliability.md：只重试幂等）。
func (e *Error) Retryable() bool { return codeRetryable[e.code] }

// Deprecated 该码是否已弃用（禁止新用）。
func (e *Error) Deprecated() bool { return codeDeprecated[e.code] }

// —— 包级元数据访问器（对非 *Error 的码查询） ——

// HTTPStatusOf 查询状态码的 HTTP 语义；未登记码按内部错误（10009）语义返回。
func HTTPStatusOf(code string) int {
	if v, ok := codeHTTP[code]; ok {
		return v
	}
	return codeHTTP[CodeInternal]
}

// MessageKeyOf 查询状态码的 i18n 键；未登记返回空串。
func MessageKeyOf(code string) string { return codeMessageKey[code] }

// IsSuccess 判断是否成功码（10000）。
func IsSuccess(code string) bool { return code == CodeSuccess }

// assertRegistered 未登记码直接 panic——手写数字/未登记码是编程错误（status-codes.md 禁手写数字）。
func assertRegistered(code string) {
	if _, ok := codeHTTP[code]; !ok {
		panic("errorx: 未登记状态码 " + code + "（请走 contracts/status-codes.yaml 登记 + 重新生成）")
	}
}

// kv 把变长键值对转为 map；奇数长度是调用方编程错误，panic 而非静默。
func kv(pairs []any) map[string]any {
	if len(pairs) == 0 {
		return nil
	}
	if len(pairs)%2 != 0 {
		panic("errorx: details 必须是 key,value 成对出现")
	}
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		k, ok := pairs[i].(string)
		if !ok {
			panic("errorx: details key 必须是 string")
		}
		m[k] = pairs[i+1]
	}
	return m
}
