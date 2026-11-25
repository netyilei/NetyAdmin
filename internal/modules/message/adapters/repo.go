// repo.go：message 持久化（PG——模板/发送记录；CopyFrom 无需——行级 CRUD）。
package adapters

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/kernel/security/primitives"
	"github.com/netyilei/netyadmin/internal/modules/message/application"
	"github.com/netyilei/netyadmin/internal/modules/message/domain"
)

// TemplateRepo 实现 application.TemplateRepo（PG）。
type TemplateRepo struct {
	pool *database.Pool
}

// RecordRepo 实现 application.RecordRepo（PG）。
type RecordRepo struct {
	pool *database.Pool
}

// NewTemplateRepo 创建。
func NewTemplateRepo(pool *database.Pool) *TemplateRepo { return &TemplateRepo{pool: pool} }

// NewRecordRepo 创建。
func NewRecordRepo(pool *database.Pool) *RecordRepo { return &RecordRepo{pool: pool} }

// —— TemplateRepo ——

const templateCols = `id, name, channel, subject, body, status, created_at, updated_at`

func scanTemplate(row interface{ Scan(...any) error }) (*domain.Template, error) {
	var t domain.Template
	if err := row.Scan(&t.ID, &t.Name, &t.Channel, &t.Subject, &t.Body, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateTemplate 实现 application.TemplateRepo。
func (r *TemplateRepo) Create(ctx context.Context, t *domain.Template) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO message_templates
		(id, name, channel, subject, body, status) VALUES ($1,$2,$3,$4,$5,$6)`,
		t.ID, t.Name, t.Channel, t.Subject, t.Body, t.Status)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrTemplateConflict
		}
		return fmt.Errorf("message: create template: %w", err)
	}
	return nil
}

// UpdateTemplate 实现 application.TemplateRepo。
func (r *TemplateRepo) Update(ctx context.Context, t *domain.Template) error {
	tag, err := r.pool.Exec(ctx, `UPDATE message_templates SET name=$2, channel=$3, subject=$4,
		body=$5, status=$6, updated_at=now() WHERE id=$1`, t.ID, t.Name, t.Channel, t.Subject, t.Body, t.Status)
	if err != nil {
		if database.IsUniqueViolation(err) {
			return domain.ErrTemplateConflict // 撞 name 唯一约束=409（非 500）
		}
		return fmt.Errorf("message: update template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTemplateNotFound
	}
	return nil
}

// DeleteTemplate 实现 application.TemplateRepo。
func (r *TemplateRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM message_templates WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("message: delete template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTemplateNotFound
	}
	return nil
}

// GetTemplate 实现 application.TemplateRepo（pgx.ErrNoRows 精确分流——R3/D1：
// DB 故障不得伪装 NotFound）。
func (r *TemplateRepo) Get(ctx context.Context, id string) (*domain.Template, error) {
	t, err := scanTemplate(r.pool.QueryRow(ctx,
		`SELECT `+templateCols+` FROM message_templates WHERE id=$1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// 返回 domain 哨兵（单一源）：此前 adapter 私有哨兵导致
			// errors.Is(domain.ErrTemplateNotFound) 恒 false——404 映射失效。
			return nil, domain.ErrTemplateNotFound
		}
		return nil, fmt.Errorf("message: get template: %w", err)
	}
	return t, nil
}

// GetByName 实现 application.TemplateRepo（name 唯一键查找——自举幂等面）。
func (r *TemplateRepo) GetByName(ctx context.Context, name string) (*domain.Template, error) {
	t, err := scanTemplate(r.pool.QueryRow(ctx,
		`SELECT `+templateCols+` FROM message_templates WHERE name=$1`, name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTemplateNotFound
		}
		return nil, fmt.Errorf("message: get template by name: %w", err)
	}
	return t, nil
}

// ListTemplates 实现 application.TemplateRepo（name 包含过滤）。
func (r *TemplateRepo) List(ctx context.Context, name string, offset, limit int) ([]*domain.Template, int, error) {
	// LIKE 通配转义收口（F5——用户输入 %/_ 直拼可越权通配；identity/rbac 同惯例）
	pattern := "%" + primitives.EscapeLike(name) + "%"
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM message_templates WHERE ($1='' OR name LIKE $2)`,
		name, pattern).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("message: count templates: %w", err)
	}
	rows, err := r.pool.Query(ctx, `SELECT `+templateCols+` FROM message_templates
		WHERE ($1='' OR name LIKE $2) ORDER BY created_at DESC OFFSET $3 LIMIT $4`,
		name, pattern, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("message: list templates: %w", err)
	}
	defer rows.Close()
	var out []*domain.Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}

// —— RecordRepo ——

const recordCols = `id, template_id, channel, target, subject, body, driver, status, retries, error, created_at, sent_at`

func scanRecord(row interface{ Scan(...any) error }) (*domain.SendRecord, error) {
	var r domain.SendRecord
	if err := row.Scan(&r.ID, &r.TemplateID, &r.Channel, &r.Target, &r.Subject, &r.Body,
		&r.Driver, &r.Status, &r.Retries, &r.Error, &r.CreatedAt, &r.SentAt); err != nil {
		return nil, err
	}
	return &r, nil
}

// CreateRecord 实现 application.RecordRepo。
func (r *RecordRepo) Create(ctx context.Context, rec *domain.SendRecord) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO message_send_records
		(id, template_id, channel, target, subject, body, driver, status, retries, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		rec.ID, rec.TemplateID, rec.Channel, rec.Target, rec.Subject, rec.Body,
		rec.Driver, rec.Status, rec.Retries, rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("message: create record: %w", err)
	}
	return nil
}

// UpdateStatusRecord 实现 application.RecordRepo。
func (r *RecordRepo) UpdateStatus(ctx context.Context, id string, status int16, retries int, sendErr string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE message_send_records SET status=$2::smallint, retries=$3,
		error=$4, sent_at=CASE WHEN $2=2 THEN now() ELSE sent_at END WHERE id=$1`,
		id, status, retries, sendErr)
	if err != nil {
		return fmt.Errorf("message: update record: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRecordNotFound
	}
	return nil
}

// GetRecord 实现 application.RecordRepo。
func (r *RecordRepo) Get(ctx context.Context, id string) (*domain.SendRecord, error) {
	rec, err := scanRecord(r.pool.QueryRow(ctx,
		`SELECT `+recordCols+` FROM message_send_records WHERE id=$1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRecordNotFound // domain 哨兵单一源（同模板面）
		}
		return nil, fmt.Errorf("message: get record: %w", err)
	}
	return rec, nil
}

// ListRecords 实现 application.RecordRepo（channel/status 过滤）。
func (r *RecordRepo) List(ctx context.Context, channel string, status int16, offset, limit int) ([]*domain.SendRecord, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM message_send_records
		WHERE ($1='' OR channel=$1) AND ($2=0 OR status=$2)`,
		channel, status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("message: count records: %w", err)
	}
	rows, err := r.pool.Query(ctx, `SELECT `+recordCols+` FROM message_send_records
		WHERE ($1='' OR channel=$1) AND ($2=0 OR status=$2)
		ORDER BY created_at DESC OFFSET $3 LIMIT $4`, channel, status, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("message: list records: %w", err)
	}
	defer rows.Close()
	var out []*domain.SendRecord
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, rec)
	}
	return out, total, rows.Err()
}

// ClaimRecord 是发送权 CAS 抢占（修正：worker 与 RetryJob 并发 attempt
// 同一记录=双投——UPDATE 条件推进 retries，抢到者才 Send）。
func (r *RecordRepo) Claim(ctx context.Context, id string, expectedRetries int) (bool, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE message_send_records SET retries=retries+1
		WHERE id=$1 AND retries=$2 AND status IN ($3,$4)`,
		id, expectedRetries, domain.RecordPending, domain.RecordFailed)
	if err != nil {
		return false, fmt.Errorf("message: claim record: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// RetryableRecords 实现 application.RecordRepo（pending/failed 且未穷尽）。
func (r *RecordRepo) Retryable(ctx context.Context, limit int) ([]*domain.SendRecord, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+recordCols+` FROM message_send_records
		WHERE status IN (1,3) AND retries < $1 ORDER BY created_at LIMIT $2`,
		domain.MaxRetries, limit)
	if err != nil {
		return nil, fmt.Errorf("message: retryable records: %w", err)
	}
	defer rows.Close()
	var out []*domain.SendRecord
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// CountBacklogByChannel 实现 application.RecordRepo（观测面：待发送/可重试
// 记录数按通道——status=pending 或 failed 且未达重试上限；走 (channel,status)
// 索引，scrape 时一次聚合查询）。
func (r *RecordRepo) CountBacklogByChannel(ctx context.Context) (map[string]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT channel, count(*) FROM message_send_records
		WHERE status=$1 OR (status=$2 AND retries < $3) GROUP BY channel`,
		domain.RecordPending, domain.RecordFailed, domain.MaxRetries)
	if err != nil {
		return nil, fmt.Errorf("message: count backlog: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var channel string
		var n int64
		if err := rows.Scan(&channel, &n); err != nil {
			return nil, fmt.Errorf("message: count backlog scan: %w", err)
		}
		out[channel] = n
	}
	return out, rows.Err()
}

// —— InboxRepo（站内信）——

const inboxCols = `id, user_id, title, body, is_read, created_at, read_at`

func scanInbox(row interface{ Scan(...any) error }) (*application.InboxMessage, error) {
	var m application.InboxMessage
	if err := row.Scan(&m.ID, &m.UserID, &m.Title, &m.Body, &m.IsRead, &m.CreatedAt, &m.ReadAt); err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateInbox 实现 application.InboxRepo。
func (r *RecordRepo) CreateInbox(ctx context.Context, m *application.InboxMessage) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO message_inbox (id, user_id, title, body)
		VALUES ($1,$2,$3,$4)`, m.ID, m.UserID, m.Title, m.Body)
	if err != nil {
		return fmt.Errorf("message: create inbox: %w", err)
	}
	return nil
}

// ListInbox 实现 application.InboxRepo。
func (r *RecordRepo) ListInbox(ctx context.Context, userID string, offset, limit int) ([]*application.InboxMessage, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM message_inbox WHERE user_id=$1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("message: count inbox: %w", err)
	}
	rows, err := r.pool.Query(ctx, `SELECT `+inboxCols+` FROM message_inbox
		WHERE user_id=$1 ORDER BY created_at DESC OFFSET $2 LIMIT $3`, userID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("message: list inbox: %w", err)
	}
	defer rows.Close()
	out := []*application.InboxMessage{}
	for rows.Next() {
		m, err := scanInbox(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}

// MarkRead 实现 application.InboxRepo（user_id 双条件防越权）。
func (r *RecordRepo) MarkRead(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE message_inbox SET is_read=true, read_at=now()
		WHERE id=$1 AND user_id=$2 AND is_read=false`, id, userID)
	if err != nil {
		return fmt.Errorf("message: mark read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRecordNotFound
	}
	return nil
}

// MarkAllRead 实现 application.InboxRepo。
func (r *RecordRepo) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE message_inbox SET is_read=true, read_at=now()
		WHERE user_id=$1 AND is_read=false`, userID)
	if err != nil {
		return 0, fmt.Errorf("message: mark all read: %w", err)
	}
	return tag.RowsAffected(), nil
}

// UnreadCount 实现 application.InboxRepo。
func (r *RecordRepo) UnreadCount(ctx context.Context, userID string) (int64, error) {
	var n int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM message_inbox WHERE user_id=$1 AND is_read=false`, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("message: unread count: %w", err)
	}
	return n, nil
}
