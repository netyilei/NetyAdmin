// repo.go：identity 存储适配（adapters 层）——admins CRUD + DB 回退读源
// （admin_token_revocations / admin_sessions）。SQL 一律参数化；表名/列名常量。
package adapters

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/modules/identity/domain"
)

// ErrNoRow 是查询无结果的内部哨兵（repo→application 转 domain.ErrNotFound）。
// Repo 是 identity 存储访问（pool 构造期注入）。
type Repo struct {
	pool *database.Pool
}

// NewRepo 创建。
func NewRepo(pool *database.Pool) *Repo { return &Repo{pool: pool} }

const adminCols = `id, username, password_hash, name, email, phone, avatar_url, status,
	must_change_password, token_version, last_login_at, created_at, updated_at`

func scanAdmin(row pgx.Row) (domain.Admin, error) {
	var a domain.Admin
	var status int16
	err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.Name, &a.Email, &a.Phone, &a.AvatarURL, &status,
		&a.MustChangePassword, &a.TokenVersion, &a.LastLoginAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return domain.Admin{}, err
	}
	a.Status = domain.Status(status)
	return a, nil
}

// FindByUsername 登录用（唯一约束）。
func (r *Repo) FindByUsername(ctx context.Context, username string) (domain.Admin, error) {
	a, err := scanAdmin(r.pool.QueryRow(ctx,
		`SELECT `+adminCols+` FROM admins WHERE username = $1`, username))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Admin{}, domain.ErrNotFound
	}
	return a, err
}

// FindByID profile/改密用。
func (r *Repo) FindByID(ctx context.Context, id string) (domain.Admin, error) {
	a, err := scanAdmin(r.pool.QueryRow(ctx,
		`SELECT `+adminCols+` FROM admins WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Admin{}, domain.ErrNotFound
	}
	return a, err
}

// TouchLastLogin 更新最后登录时间。
func (r *Repo) TouchLastLogin(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE admins SET last_login_at = $1 WHERE id = $2`, at, id)
	if err != nil {
		return fmt.Errorf("identity: touch last_login: %w", err)
	}
	return nil
}

// UpdatePassword 改密/重置：新 hash + TokenVersion++（用户级全作废—— tv
// 面驱动）+ must_change_password 置位（重置/强制改密场景）或清除（自助改密）。
func (r *Repo) UpdatePassword(ctx context.Context, id, newHash string, bumpVersion, mustChange bool) error {
	q := `UPDATE admins SET password_hash = $1, must_change_password = $2, updated_at = now()`
	if bumpVersion {
		q += `, token_version = token_version + 1`
	}
	q += ` WHERE id = $3`
	tag, err := r.pool.Exec(ctx, q, newHash, mustChange, id)
	if err != nil {
		return fmt.Errorf("identity: update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SetStatus 管理面启停：bumpVersion=true 时 TokenVersion++（禁用全作废——
// access 在 TTL 内即时拒绝，与改密同语义面）。
func (r *Repo) SetStatus(ctx context.Context, id string, status domain.Status, bumpVersion bool) error {
	q := `UPDATE admins SET status = $1, updated_at = now()`
	if bumpVersion {
		q += `, token_version = token_version + 1`
	}
	q += ` WHERE id = $2`
	tag, err := r.pool.Exec(ctx, q, int16(status), id)
	if err != nil {
		return fmt.Errorf("identity: set status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// List 管理面分页列表（过滤条件零值=不过滤；LIKE 输入已由 Service 层转义）。
func (r *Repo) List(ctx context.Context, f domain.ListFilter) ([]domain.Admin, int64, error) {
	where, args := listWhere(f)
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM admins`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("identity: count list: %w", err)
	}
	q := `SELECT ` + adminCols + ` FROM admins` + where +
		` ORDER BY created_at DESC, id LIMIT $` + strconv.Itoa(len(args)+1) +
		` OFFSET $` + strconv.Itoa(len(args)+2)
	rows, err := r.pool.Query(ctx, q, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("identity: list admins: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Admin, 0, f.Limit)
	for rows.Next() {
		a, err := scanAdmin(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("identity: scan admin: %w", err)
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// listWhere 组装过滤子句（条件增量追加——参数位随 args 长度推导）。
func listWhere(f domain.ListFilter) (string, []any) {
	where := ` WHERE 1=1`
	var args []any
	if f.Username != "" {
		args = append(args, f.Username+"%")
		where += fmt.Sprintf(` AND username LIKE $%d`, len(args))
	}
	if f.Name != "" {
		args = append(args, "%"+f.Name+"%")
		where += fmt.Sprintf(` AND name LIKE $%d`, len(args))
	}
	if f.Status != 0 {
		args = append(args, int16(f.Status))
		where += fmt.Sprintf(` AND status = $%d`, len(args))
	}
	return where, args
}

// Delete 删除（单/批统一 IN 面 Return 数防误删 0 行误判成功）。
func (r *Repo) Delete(ctx context.Context, ids []string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM admins WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("identity: delete admins: %w", err)
	}
	if tag.RowsAffected() != int64(len(ids)) {
		return domain.ErrNotFound // 有 ID 不存在：整批拒绝（管理面幂等删除由前端先查）
	}
	return nil
}

// UpdateAdminFace 管理面编辑（name/email——email/phone 指针语义：nil=不改该列，
// 空串=显式清除。 根修：此前全量覆盖，编辑表单不含 phone 时把本人
// 手机号清空；email 同理——管理面列表已掩码，未取到原文时不得回写掩码串）。
func (r *Repo) UpdateAdminFace(ctx context.Context, id, name string, email *string, phone *string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE admins SET name = $2, email = COALESCE($3, email), phone = COALESCE($4, phone), updated_at = now() WHERE id = $1`,
		id, name, email, phone)
	if err != nil {
		return fmt.Errorf("identity: update admin face: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UpdateProfile 更新展示字段（本人资料面——name/email/phone/avatar 全量）。
func (r *Repo) UpdateProfile(ctx context.Context, id, name, email, phone, avatarURL string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE admins SET name = $1, email = $2, phone = $3, avatar_url = $4, updated_at = now() WHERE id = $5`, name, email, phone, avatarURL, id)
	if err != nil {
		return fmt.Errorf("identity: update profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// CountAdmins 首启 bootstrap 判定（0 = 需要创建）。
func (r *Repo) CountAdmins(ctx context.Context) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM admins`).Scan(&n); err != nil {
		return 0, fmt.Errorf("identity: count admins: %w", err)
	}
	return n, nil
}

// InsertAdmin 创建管理员（bootstrap/管理面共用）；username 唯一冲突 →
// ErrUsernameTaken（管理面创建的业务判定面；bootstrap 前有 CountAdmins 兜底）。
func (r *Repo) InsertAdmin(ctx context.Context, a domain.Admin) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO admins (id, username, password_hash, name, email, phone, avatar_url, status, must_change_password, token_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		a.ID, a.Username, a.PasswordHash, a.Name, a.Email, a.Phone, a.AvatarURL, int16(a.Status),
		a.MustChangePassword, a.TokenVersion)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrUsernameTaken
		}
		return fmt.Errorf("identity: insert admin: %w", err)
	}
	return nil
}

// —— DB 回退读源（revocation / session；读向语义） ——

// RevokePersisted 吊销落库（写向：Redis 写成功后同步落库，供回退读）。
func (r *Repo) RevokePersisted(ctx context.Context, jti string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO admin_token_revocations (jti, expires_at) VALUES ($1, $2)
		ON CONFLICT (jti) DO NOTHING`, jti, expiresAt)
	if err != nil {
		return fmt.Errorf("identity: persist revocation: %w", err)
	}
	return nil
}

// IsRevokedFallback RevocationFallback：Redis 不可用时读 DB。
func (r *Repo) IsRevokedFallback(ctx context.Context, jti string) (bool, error) {
	var revoked bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM admin_token_revocations WHERE jti = $1)`, jti).Scan(&revoked)
	if err != nil {
		return false, fmt.Errorf("identity: revocation fallback: %w", err)
	}
	return revoked, nil
}

// CleanExpiredRevocations 分批删除过期吊销行（expires_at < before——TTL 已过，
// 读源恒 miss；周期 Job 调用，audit RetentionJob 同款分批防长事务）。
func (r *Repo) CleanExpiredRevocations(ctx context.Context, before time.Time) (int64, error) {
	const batchLimit = 5000
	var total int64
	for {
		tag, err := r.pool.Exec(ctx,
			`DELETE FROM admin_token_revocations WHERE jti IN (
				SELECT jti FROM admin_token_revocations WHERE expires_at < $1 LIMIT $2
			)`, before, batchLimit)
		if err != nil {
			return total, fmt.Errorf("identity: clean expired revocations: %w", err)
		}
		n := tag.RowsAffected()
		total += n
		if n < batchLimit {
			break
		}
	}
	return total, nil
}

// PutSessionFallback 写会话回退副本（payload = admin 快照 JSON）。
func (r *Repo) PutSessionFallback(ctx context.Context, sessionID string, adminID string, payload []byte, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO admin_sessions (session_id, admin_id, payload, expires_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (session_id) DO UPDATE SET payload = $3, expires_at = $4`,
		sessionID, adminID, payload, expiresAt)
	if err != nil {
		return fmt.Errorf("identity: persist session: %w", err)
	}
	return nil
}

// DeleteSessionFallback 登出时删除回退副本（与 Redis DropSession 同步——
// 否则 DB 副本让已登出会话继续可用）。
func (r *Repo) DeleteSessionFallback(ctx context.Context, sessionID string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM admin_sessions WHERE session_id = $1`, sessionID); err != nil {
		return fmt.Errorf("identity: delete session fallback: %w", err)
	}
	return nil
}

// GetSessionFallback SessionFallback：读回退（payload JSON 原样）。
func (r *Repo) GetSessionFallback(ctx context.Context, sessionID string) ([]byte, bool, error) {
	var payload []byte
	err := r.pool.QueryRow(ctx,
		`SELECT payload FROM admin_sessions WHERE session_id = $1 AND expires_at > now()`, sessionID).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("identity: session fallback: %w", err)
	}
	return payload, true, nil
}

// SessionSnapshot 是会话负载结构（jsonb 序列化契约）。
type SessionSnapshot struct {
	AdminID  string   `json:"admin_id"`
	Username string   `json:"username"`
	TokenVer int64    `json:"token_version"`
	Roles    []string `json:"roles"`
}
