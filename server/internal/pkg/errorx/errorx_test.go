package errorx_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"NetyAdmin/internal/pkg/errorx"
)

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

func TestIs(t *testing.T) {
	bizErr := errorx.New(errorx.CodeUserNotFound)

	t.Run("matching code returns true", func(t *testing.T) {
		assert.True(t, errorx.Is(bizErr, errorx.CodeUserNotFound))
	})

	t.Run("different code returns false", func(t *testing.T) {
		assert.False(t, errorx.Is(bizErr, errorx.CodeForbidden))
	})

	t.Run("non-BizError returns false", func(t *testing.T) {
		plainErr := errors.New("plain error")
		assert.False(t, errorx.Is(plainErr, errorx.CodeUserNotFound))
	})

	t.Run("nil returns false", func(t *testing.T) {
		assert.False(t, errorx.Is(nil, errorx.CodeUserNotFound))
	})
}
