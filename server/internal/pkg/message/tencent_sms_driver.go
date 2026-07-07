package message

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tcSms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

// SmsConfig 腾讯云短信配置。
// 字段与 [config].SmsConfig (internal/config/config.go) 一一对应，
// 由 wire.go 从全局配置构造后传入 NewTencentSmsDriver。
type SmsConfig struct {
	SecretID  string
	SecretKey string
	AppID     string // SmsSdkAppId
	SignName  string
	Region    string // 接入地域，如 ap-guangzhou
}

// tencentSmsDriver 基于腾讯云 SMS SDK 实现 SmsDriver 接口。
//
// 配置优先级：ConfigProvider（运行时 sys_config group="sms_config" 热更） > 静态 cfg。
// 与 emailDriver.resolveConfig 保持一致的动态覆盖模式，便于运维在后台改配置而无需重启。
type tencentSmsDriver struct {
	cfg      SmsConfig
	provider ConfigProvider
}

// NewTencentSmsDriver 构造腾讯云短信驱动。
// cfg 是启动期从 config.toml 加载的静态配置；provider 可选，用于运行时热更。
func NewTencentSmsDriver(cfg SmsConfig, provider ConfigProvider) SmsDriver {
	return &tencentSmsDriver{cfg: cfg, provider: provider}
}

// resolveConfig 合并静态 cfg 与 ConfigProvider 运行时配置。
// ConfigProvider 返回 error 或空 map 时回退到静态 cfg（fail-safe，不阻断发送）。
func (d *tencentSmsDriver) resolveConfig(ctx context.Context) (SmsConfig, error) {
	cfg := d.cfg
	if d.provider == nil {
		return cfg, nil
	}
	vals, err := d.provider.GetByGroup(ctx, "sms_config")
	if err != nil {
		// 配置读取失败仅记录，回退静态配置（emailDriver 同款策略）
		slog.Warn("tencent sms: read dynamic config failed, fallback to static cfg", "err", err)
		return cfg, nil
	}
	if len(vals) == 0 {
		return cfg, nil
	}
	if v, ok := vals["secret_id"]; ok && v != "" {
		cfg.SecretID = v
	}
	if v, ok := vals["secret_key"]; ok && v != "" {
		cfg.SecretKey = v
	}
	if v, ok := vals["app_id"]; ok && v != "" {
		cfg.AppID = v
	}
	if v, ok := vals["sign_name"]; ok && v != "" {
		cfg.SignName = v
	}
	if v, ok := vals["region"]; ok && v != "" {
		cfg.Region = v
	}
	return cfg, nil
}

// newClient 构造腾讯云 SMS 客户端。
func (d *tencentSmsDriver) newClient(cfg SmsConfig) (*tcSms.Client, error) {
	if cfg.SecretID == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("tencent sms: secret_id/secret_key 未配置")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("tencent sms: region 未配置")
	}
	credential := common.NewCredential(cfg.SecretID, cfg.SecretKey)
	cpf := profile.NewClientProfile()
	return tcSms.NewClient(credential, cfg.Region, cpf)
}

// Send 实现基础 Driver.Send。
// 腾讯云 SMS 必须用模板发送（不支持纯文本），因此本方法约定：
//   - receiver: 手机号（带国际区号，如 "+8613800138000"）
//   - content:  解释为腾讯云模板 ID（业务约定）
//   - params:   模板参数
//   - title:    忽略（短信无标题）
func (d *tencentSmsDriver) Send(ctx context.Context, receiver, title, content string, params map[string]string) error {
	return d.SendWithTemplate(ctx, receiver, content, params)
}

// SendWithTemplate 发送模板短信。
//
// 参数：
//   - phone:       手机号，带国际区号（如 "+8613800138000"）
//   - templateID:  腾讯云短信模板 ID（如 "1234567"，需在控制台审核通过）
//   - params:      模板参数，腾讯云要求按模板变量顺序传 []string。
//                  本实现按 params 的 key 字典序排序后取 value，保证顺序稳定。
//                  业务侧模板变量命名建议用数字 key（如 "1","2","3"）以便排序。
func (d *tencentSmsDriver) SendWithTemplate(ctx context.Context, phone, templateID string, params map[string]string) error {
	if phone == "" {
		return fmt.Errorf("tencent sms: phone is empty")
	}
	if templateID == "" {
		return fmt.Errorf("tencent sms: templateID is empty")
	}

	cfg, _ := d.resolveConfig(ctx)
	if cfg.AppID == "" || cfg.SignName == "" {
		return fmt.Errorf("tencent sms: app_id/sign_name 未配置")
	}

	client, err := d.newClient(cfg)
	if err != nil {
		return err
	}

	// 构造模板参数（腾讯云要求 []*string，按 key 字典序保证顺序稳定）
	paramValues := make([]string, 0, len(params))
	for k := range params {
		paramValues = append(paramValues, k)
	}
	sort.Strings(paramValues)
	templateParamSet := make([]*string, 0, len(paramValues))
	for _, k := range paramValues {
		v := params[k]
		templateParamSet = append(templateParamSet, &v)
	}

	req := tcSms.NewSendSmsRequest()
	req.SmsSdkAppId = common.StringPtr(cfg.AppID)
	req.SignName = common.StringPtr(cfg.SignName)
	req.TemplateId = common.StringPtr(templateID)
	req.PhoneNumberSet = common.StringPtrs([]string{phone})
	req.TemplateParamSet = templateParamSet

	resp, err := client.SendSmsWithContext(ctx, req)
	if err != nil {
		return fmt.Errorf("tencent sms: SendSms 调用失败: %w", err)
	}

	// 检查 per-phone 发送结果
	if resp.Response == nil || len(resp.Response.SendStatusSet) == 0 {
		return fmt.Errorf("tencent sms: 响应无 SendStatusSet")
	}
	status := resp.Response.SendStatusSet[0]
	if status.Code == nil {
		return fmt.Errorf("tencent sms: 响应 Code 为空")
	}
	if *status.Code != "Ok" {
		msg := ""
		if status.Message != nil {
			msg = *status.Message
		}
		return fmt.Errorf("tencent sms: 发送失败 code=%s msg=%s", *status.Code, msg)
	}

	return nil
}
