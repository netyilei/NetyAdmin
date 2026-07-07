package message

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeConfigProvider 用于测试 resolveConfig 的动态配置覆盖。
type fakeConfigProvider struct {
	vals map[string]string
	err  error
}

func (f *fakeConfigProvider) GetByGroup(_ context.Context, _ string) (map[string]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.vals, nil
}

// TestTencentSms_ResolveConfig_Static 验证无 ConfigProvider 时回退静态 cfg。
func TestTencentSms_ResolveConfig_Static(t *testing.T) {
	d := NewTencentSmsDriver(SmsConfig{
		SecretID:  "static-id",
		SecretKey: "static-key",
		AppID:     "static-app",
		SignName:  "static-sign",
		Region:    "ap-shanghai",
	}, nil)

	cfg, err := d.(*tencentSmsDriver).resolveConfig(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "static-id", cfg.SecretID)
	assert.Equal(t, "ap-shanghai", cfg.Region)
}

// TestTencentSms_ResolveConfig_DynamicOverride 验证 ConfigProvider 覆盖静态配置。
func TestTencentSms_ResolveConfig_DynamicOverride(t *testing.T) {
	provider := &fakeConfigProvider{vals: map[string]string{
		"secret_id":  "dynamic-id",
		"region":     "ap-beijing",
		"sign_name":  "dynamic-sign",
		"empty_keep": "",
	}}
	d := NewTencentSmsDriver(SmsConfig{
		SecretID:  "static-id",
		SecretKey: "static-key",
		AppID:     "static-app",
		SignName:  "static-sign",
		Region:    "ap-shanghai",
	}, provider)

	cfg, err := d.(*tencentSmsDriver).resolveConfig(context.Background())
	require.NoError(t, err)
	// 动态覆盖
	assert.Equal(t, "dynamic-id", cfg.SecretID)
	assert.Equal(t, "ap-beijing", cfg.Region)
	assert.Equal(t, "dynamic-sign", cfg.SignName)
	// 动态为空时保留静态值
	assert.Equal(t, "static-key", cfg.SecretKey)
	assert.Equal(t, "static-app", cfg.AppID)
}

// TestTencentSms_ResolveConfig_ProviderError 验证 ConfigProvider 出错时回退静态配置（fail-safe）。
func TestTencentSms_ResolveConfig_ProviderError(t *testing.T) {
	provider := &fakeConfigProvider{err: assertError("config backend down")}
	d := NewTencentSmsDriver(SmsConfig{
		SecretID: "static-id",
		Region:   "ap-guangzhou",
	}, provider)

	cfg, err := d.(*tencentSmsDriver).resolveConfig(context.Background())
	// provider 出错不应返回 error（fail-safe 回退静态配置）
	require.NoError(t, err)
	assert.Equal(t, "static-id", cfg.SecretID)
	assert.Equal(t, "ap-guangzhou", cfg.Region)
}

// TestTencentSms_NewClient_MissingCredentials 验证缺少密钥时 newClient 返回明确错误。
func TestTencentSms_NewClient_MissingCredentials(t *testing.T) {
	d := NewTencentSmsDriver(SmsConfig{Region: "ap-guangzhou"}, nil)

	_, err := d.(*tencentSmsDriver).newClient(SmsConfig{Region: "ap-guangzhou"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret_id/secret_key")
}

// TestTencentSms_NewClient_MissingRegion 验证缺少 region 时返回明确错误。
func TestTencentSms_NewClient_MissingRegion(t *testing.T) {
	d := NewTencentSmsDriver(SmsConfig{
		SecretID:  "id",
		SecretKey: "key",
	}, nil)

	_, err := d.(*tencentSmsDriver).newClient(SmsConfig{SecretID: "id", SecretKey: "key"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "region")
}

// TestTencentSms_SendWithTemplate_Validation 验证参数校验（不发真实请求）。
func TestTencentSms_SendWithTemplate_Validation(t *testing.T) {
	d := NewTencentSmsDriver(SmsConfig{
		SecretID: "id", SecretKey: "key", AppID: "app", SignName: "sign", Region: "ap-guangzhou",
	}, nil)

	// 空手机号
	err := d.SendWithTemplate(context.Background(), "", "tpl", map[string]string{"1": "v"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "phone")

	// 空模板 ID
	err = d.SendWithTemplate(context.Background(), "+8613800138000", "", map[string]string{"1": "v"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "templateID")
}

// assertError 是 fakeConfigProvider 用的简单 error 类型。
type assertError string

func (e assertError) Error() string { return string(e) }
