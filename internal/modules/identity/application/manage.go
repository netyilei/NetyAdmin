// manage.go：管理员账号管理面（V1 /admins 6 端点对齐 + 重置密码）。
// 安全规则：操作者对自身禁用/删除拒绝（防自锁 ErrSelfOperation）；禁用 =
// TokenVersion++（access TTL 内即时拒绝，与改密同作废语义面）；重置密码 =
// 随机密码一次性返回 + must_change_password 强制首登改密（bootstrap 同语义）。
package application

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/identity/domain"
)

// 随机密码参数（bootstrap/重置共用；URL 安全字母表剔除易混淆字符）。
const (
	passwordRandomLen = 15
	passwordAlphabet  = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

// GeneratePassword 生成随机密码（bootstrap 与管理员重置共用——G1-06 收拢；
// 一次生成只经一次响应通道交付，不落日志/DB）。
func GeneratePassword() (string, error) {
	buf := make([]byte, passwordRandomLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("identity: generate password: %w", err)
	}
	out := make([]byte, len(buf))
	for i, b := range buf {
		out[i] = passwordAlphabet[int(b)%len(passwordAlphabet)]
	}
	return string(out), nil
}

// CreateAdmin 管理面创建：密码强度校验 + username 唯一（adapters 唯一冲突映射）
// + must_change_password 置位（初始密码一次性——首登强制自改）。
func (s *Service) CreateAdmin(ctx context.Context, username, password, name, email string) (domain.Profile, error) {
	if err := s.ready(); err != nil {
		return domain.Profile{}, err
	}
	if len(password) < passwordMinLen || len(password) > passwordMaxBytes {
		return domain.Profile{}, domain.ErrPasswordWeak
	}
	hash, err := primitives.HashPassword(password, s.cfg.BcryptCost)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("identity: hash password: %w", err)
	}
	now := s.clock.Now().UTC()
	admin := domain.Admin{
		ID:                 primitives.NewULID(s.clock),
		Username:           username,
		PasswordHash:       hash,
		Name:               name,
		Email:              email,
		Status:             domain.StatusActive,
		MustChangePassword: true,
		TokenVersion:       1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.repo.InsertAdmin(ctx, admin); err != nil {
		return domain.Profile{}, err
	}
	return maskProfile(admin.ToProfile()), nil
}

// maskProfile 是管理面他人记录出口掩码（email/phone PII；用户口径：手机号/
// 真实姓名/邮箱需 PII 处理）。登录者看自己（GetProfile/UpdateProfile/LoginResult）
// 不掩码——管理端列表/创建/编辑响应必须掩码。
func maskProfile(p domain.Profile) domain.Profile {
	if p.Email != "" {
		p.Email = primitives.Mask("email", p.Email)
	}
	if p.Phone != "" {
		p.Phone = primitives.Mask("phone", p.Phone)
	}
	return p
}

// ListAdmins 管理面分页列表（过滤输入 LIKE 转义在此层收口；email/phone 掩码输出）。
func (s *Service) ListAdmins(ctx context.Context, f domain.ListFilter) ([]domain.Profile, int64, error) {
	if err := s.ready(); err != nil {
		return nil, 0, err
	}
	f.Username = primitives.EscapeLike(f.Username)
	f.Name = primitives.EscapeLike(f.Name)
	admins, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.Profile, 0, len(admins))
	for _, a := range admins {
		out = append(out, maskProfile(a.ToProfile()))
	}
	return out, total, nil
}

// GetAdmin 管理面详情（Profile 投影同出口）。
// 详情按权限（admin.admins.get 专权限）返回原文：编辑表单需要原始 email/phone，
// 否则回写会落掩码串；批量暴露面已由列表掩码收口（范围列表/审计/导出必须
// 掩码或按权限——详情=按权限）。
func (s *Service) GetAdmin(ctx context.Context, id string) (domain.Profile, error) {
	if err := s.ready(); err != nil {
		return domain.Profile{}, err
	}
	admin, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.Profile{}, err
	}
	return admin.ToProfile(), nil
}

// SetAdminStatus 启停：禁用 bump TokenVersion（即时全作废）；自身禁用拒绝。
func (s *Service) SetAdminStatus(ctx context.Context, operatorID, id string, status domain.Status) error {
	if err := s.ready(); err != nil {
		return err
	}
	if operatorID == id && status != domain.StatusActive {
		return domain.ErrSelfOperation
	}
	disabled := status == domain.StatusDisabled
	if err := s.repo.SetStatus(ctx, id, status, disabled); err != nil {
		return err
	}
	if disabled { // bumpVersion=全会话作废——事件化通知
		s.publishTokenRevoked(ctx, id, reasonAccountDisabled)
	}
	return nil
}

// ResetAdminPassword 管理员重置密码：随机密码一次性返回 + must_change_password
// + TokenVersion++（旧 access/refresh 全作废——被重置者立即下线）。
func (s *Service) ResetAdminPassword(ctx context.Context, id string) (string, error) {
	if err := s.ready(); err != nil {
		return "", err
	}
	password, err := GeneratePassword()
	if err != nil {
		return "", err
	}
	hash, err := primitives.HashPassword(password, s.cfg.BcryptCost)
	if err != nil {
		return "", fmt.Errorf("identity: hash password: %w", err)
	}
	if err := s.repo.UpdatePassword(ctx, id, hash, true, true); err != nil {
		return "", err
	}
	s.publishTokenRevoked(ctx, id, reasonPasswordReset)
	return password, nil
}

// DeleteAdmins 删除（单/批统一；操作者在删除列 = 拒绝防自锁）。
func (s *Service) DeleteAdmins(ctx context.Context, operatorID string, ids []string) error {
	if err := s.ready(); err != nil {
		return err
	}
	if len(ids) == 0 {
		return domain.ErrNotFound
	}
	for _, id := range ids {
		if id == operatorID {
			return domain.ErrSelfOperation
		}
	}
	return s.repo.Delete(ctx, ids)
}
