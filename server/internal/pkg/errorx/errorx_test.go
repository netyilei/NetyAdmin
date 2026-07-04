package errorx_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"NetyAdmin/internal/pkg/errorx"
)

// customSentinelErr 是测试用的自定义错误类型，用于验证 errors.As 能穿透 BizError 的 Unwrap 链。
type customSentinelErr struct {
	msg string
}

func (e *customSentinelErr) Error() string { return e.msg }

func TestCode_Message(t *testing.T) {
	tests := []struct {
		name string
		code errorx.Code
		want string
	}{
		{"success", errorx.CodeSuccess, "操作成功"},
		{"invalid params", errorx.CodeInvalidParams, "参数错误"},
		{"unauthorized", errorx.CodeUnauthorized, "未授权"},
		{"forbidden", errorx.CodeForbidden, "无权限"},
		{"not found", errorx.CodeNotFound, "资源不存在"},
		{"internal error", errorx.CodeInternalError, "服务器内部错误"},
		{"too many requests", errorx.CodeTooManyRequest, "请求过于频繁"},
		{"user not found", errorx.CodeUserNotFound, "用户不存在"},
		{"password wrong", errorx.CodePasswordWrong, "密码错误"},
		{"token expired", errorx.CodeTokenExpired, "令牌已过期"},
		{"role not found", errorx.CodeRoleNotFound, "角色不存在"},
		{"app key invalid", errorx.CodeAppKeyInvalid, "AppKey无效"},
		{"upload record not found", errorx.CodeUploadRecordNotFound, "上传记录不存在"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.code.Message())
		})
	}
}

func TestCode_Message_UnknownCode(t *testing.T) {
	unknownCode := errorx.Code(999999)
	assert.Equal(t, "未知错误", unknownCode.Message())
}

func TestCode_String(t *testing.T) {
	tests := []struct {
		code errorx.Code
		want string
	}{
		{errorx.CodeSuccess, "100000"},
		{errorx.CodeInvalidParams, "100001"},
		{errorx.CodeUnauthorized, "100002"},
		{errorx.CodeUserNotFound, "101001"},
		{errorx.CodeAppKeyInvalid, "101301"},
		// Verify the %06d format pads codes < 10000 with leading zeros.
		// Previously %04d was used, which would produce a 4-char string here.
		{errorx.Code(1), "000001"},
		{errorx.Code(9999), "009999"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.code.String())
		})
	}
}

func TestNew_DefaultMessage(t *testing.T) {
	err := errorx.New(errorx.CodeUserNotFound)
	assert.Equal(t, errorx.CodeUserNotFound, err.Code)
	assert.Equal(t, "用户不存在", err.Message)
	assert.Equal(t, "用户不存在", err.Error())
}

func TestNew_CustomMessage(t *testing.T) {
	err := errorx.New(errorx.CodeInvalidParams, "自定义错误消息")
	assert.Equal(t, errorx.CodeInvalidParams, err.Code)
	assert.Equal(t, "自定义错误消息", err.Message)
	assert.Equal(t, "自定义错误消息", err.Error())
}

func TestNew_EmptyCustomMessageFallsBack(t *testing.T) {
	err := errorx.New(errorx.CodeNotFound, "")
	assert.Equal(t, "资源不存在", err.Message)
}

func TestBizError_Error(t *testing.T) {
	err := errorx.New(errorx.CodeForbidden)
	assert.Equal(t, "无权限", err.Error())

	// Verify it satisfies the error interface
	var _ error = err
}

// TestNewWithErr 验证 NewWithErr 构造的 BizError 正确携带原始错误链。
func TestNewWithErr(t *testing.T) {
	original := errors.New("gorm: record not found")

	t.Run("Err field preserved", func(t *testing.T) {
		err := errorx.NewWithErr(errorx.CodeUserNotFound, original)
		assert.Equal(t, errorx.CodeUserNotFound, err.Code)
		assert.Equal(t, "用户不存在", err.Message)
		assert.Same(t, original, err.Err)
	})

	t.Run("custom message overrides default", func(t *testing.T) {
		err := errorx.NewWithErr(errorx.CodeUserNotFound, original, "用户不存在（自定义）")
		assert.Equal(t, "用户不存在（自定义）", err.Message)
		assert.Same(t, original, err.Err)
	})

	t.Run("nil err behaves like New", func(t *testing.T) {
		err := errorx.NewWithErr(errorx.CodeUserNotFound, nil)
		assert.Equal(t, errorx.CodeUserNotFound, err.Code)
		assert.Nil(t, err.Err)
		assert.Equal(t, "用户不存在", err.Error())
	})

	t.Run("Error() includes wrapped err", func(t *testing.T) {
		err := errorx.NewWithErr(errorx.CodeUserNotFound, original)
		assert.Equal(t, "用户不存在: gorm: record not found", err.Error())
	})

	t.Run("empty custom message falls back to default", func(t *testing.T) {
		err := errorx.NewWithErr(errorx.CodeUserNotFound, original, "")
		assert.Equal(t, "用户不存在", err.Message)
	})
}

// TestBizError_Unwrap 验证 Unwrap 返回原始错误，支持 errors.As / errors.Is 穿透。
func TestBizError_Unwrap(t *testing.T) {
	t.Run("Unwrap returns wrapped err", func(t *testing.T) {
		original := errors.New("db connection lost")
		err := errorx.NewWithErr(errorx.CodeInternalError, original)
		assert.Same(t, original, err.Unwrap())
	})

	t.Run("Unwrap returns nil when no wrapped err", func(t *testing.T) {
		err := errorx.New(errorx.CodeUserNotFound)
		assert.Nil(t, err.Unwrap())
	})

	t.Run("errors.As can reach wrapped error through BizError", func(t *testing.T) {
		sentinel := &customSentinelErr{msg: "sentinel"}
		err := errorx.NewWithErr(errorx.CodeInternalError, sentinel)

		var target *customSentinelErr
		assert.True(t, errors.As(err, &target))
		assert.Equal(t, "sentinel", target.msg)
	})
}

// TestBizError_Is_Method 验证 BizError.Is 方法支持 errors.Is 穿透 fmt.Errorf("%w") 包装链。
func TestBizError_Is_Method(t *testing.T) {
	t.Run("errors.Is matches same code (direct)", func(t *testing.T) {
		err := errorx.New(errorx.CodeUserNotFound)
		assert.True(t, errors.Is(err, errorx.New(errorx.CodeUserNotFound)))
	})

	t.Run("errors.Is does not match different code", func(t *testing.T) {
		err := errorx.New(errorx.CodeUserNotFound)
		assert.False(t, errors.Is(err, errorx.New(errorx.CodeForbidden)))
	})

	t.Run("errors.Is matches through fmt.Errorf %w wrap", func(t *testing.T) {
		bizErr := errorx.New(errorx.CodeUserNotFound)
		wrapped := fmt.Errorf("service layer: %w", bizErr)
		assert.True(t, errors.Is(wrapped, errorx.New(errorx.CodeUserNotFound)))
	})

	t.Run("errors.Is matches through NewWithErr wrap", func(t *testing.T) {
		original := errors.New("gorm: record not found")
		bizErr := errorx.NewWithErr(errorx.CodeUserNotFound, original)
		// errors.Is(bizErr, errorx.New(CodeUserNotFound)) 通过 Is 方法匹配
		assert.True(t, errors.Is(bizErr, errorx.New(errorx.CodeUserNotFound)))
		// errors.Is(bizErr, original) 通过 Unwrap 链匹配
		assert.True(t, errors.Is(bizErr, original))
	})

	t.Run("errors.Is does not match non-BizError target", func(t *testing.T) {
		err := errorx.New(errorx.CodeUserNotFound)
		assert.False(t, errors.Is(err, errors.New("plain error")))
	})

	t.Run("errors.Is matches through nested wrap chain", func(t *testing.T) {
		bizErr := errorx.NewWithErr(errorx.CodeRoleNotFound, errors.New("db err"))
		middle := fmt.Errorf("repo: %w", bizErr)
		outer := fmt.Errorf("service: %w", middle)
		assert.True(t, errors.Is(outer, errorx.New(errorx.CodeRoleNotFound)))
	})
}

// TestErrorsAs_Pattern 验证推荐的 errors.As 模式可正确取出 BizError 业务码。
func TestErrorsAs_Pattern(t *testing.T) {
	t.Run("errors.As extracts BizError through wrap", func(t *testing.T) {
		bizErr := errorx.New(errorx.CodeUserNotFound, "用户不存在")
		wrapped := fmt.Errorf("service: %w", bizErr)

		var target *errorx.BizError
		assert.True(t, errors.As(wrapped, &target))
		assert.Equal(t, errorx.CodeUserNotFound, target.Code)
		assert.Equal(t, "用户不存在", target.Message)
	})

	t.Run("errors.As returns false for plain error", func(t *testing.T) {
		plainErr := errors.New("plain")
		var target *errorx.BizError
		assert.False(t, errors.As(plainErr, &target))
		assert.Nil(t, target)
	})
}
