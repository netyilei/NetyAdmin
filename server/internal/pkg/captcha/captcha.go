package captcha

import (
	"github.com/mojocn/base64Captcha"
)

// Manager 验证码管理器
type Manager struct {
	store base64Captcha.Store
}

func NewManager(store base64Captcha.Store) *Manager {
	return &Manager{store: store}
}

// Generate 生成指定类型的验证码
func (m *Manager) Generate(captchaType string, width, height, length int) (id, b64s string, err error) {
	var driver base64Captcha.Driver

	switch captchaType {
	case "string":
		driver = base64Captcha.NewDriverString(height, width, 0, 0, length, "23456789abcdefghjkmnpqrstuvwxyz", nil, nil, nil)
	case "math":
		driver = base64Captcha.NewDriverMath(height, width, 0, 0, nil, nil, nil)
	default: // digit
		driver = base64Captcha.NewDriverDigit(height, width, length, 0.7, 80)
	}

	cp := base64Captcha.NewCaptcha(driver, m.store)
	id, b64s, _, err = cp.Generate()
	return
}

// Verify 校验验证码
func (m *Manager) Verify(id, answer string, clear bool) bool {
	if id == "" || answer == "" {
		return false
	}
	return m.store.Verify(id, answer, clear)
}
