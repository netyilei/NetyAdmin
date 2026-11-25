// repo.go：storage 持久化（PG——配置/上传记录）。
package adapters

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/netyilei/netyadmin/internal/kernel/database"
	"github.com/netyilei/netyadmin/internal/modules/storage/application"
	"github.com/netyilei/netyadmin/internal/modules/storage/domain"
)

// StorageConfigRepo 实现 application.StorageConfigRepo（PG）。
type StorageConfigRepo struct {
	pool *database.Pool
}

// NewStorageConfigRepo 创建。
func NewStorageConfigRepo(pool *database.Pool) *StorageConfigRepo {
	return &StorageConfigRepo{pool: pool}
}

// UploadRecordRepo 实现 application.UploadRecordRepo（PG）。
type UploadRecordRepo struct {
	pool *database.Pool
}

// NewUploadRecordRepo 创建。
func NewUploadRecordRepo(pool *database.Pool) *UploadRecordRepo {
	return &UploadRecordRepo{pool: pool}
}

// isUnique 判定唯一约束冲突（PG 23505）。
func isUnique(err error) bool { return database.IsUniqueViolation(err) }

// isNoRows 判定 pgx 无行错误（QueryRow.Scan 返回 pgx.ErrNoRows）。
func isNoRows(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no rows in result set")
}

const configCols = `id, name, provider, endpoint, region, bucket_lookup, bucket, access_key,
	secret_key, domain, path_prefix, is_default, status, max_file_size, allowed_types, created_at, updated_at`

func scanConfig(row interface{ Scan(...any) error }) (*domain.StorageConfig, error) {
	var c domain.StorageConfig
	if err := row.Scan(&c.ID, &c.Name, &c.Provider, &c.Endpoint, &c.Region, &c.BucketLookup,
		&c.Bucket, &c.AccessKey, &c.SecretKey, &c.Domain, &c.PathPrefix, &c.IsDefault,
		&c.Status, &c.MaxFileSize, &c.AllowedTypes, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// Create 实现 application.StorageConfigRepo（is_default 强制 false 落行——
// 「先建行后设默认」：直带 true 撞部分唯一索引 uk_storage_configs_default
// 必 409（联测审计 B1）；默认位由 service.CreateConfig 随后 SetDefault 事务设置）。
func (r *StorageConfigRepo) Create(ctx context.Context, c *domain.StorageConfig) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO storage_configs
		(id, name, provider, endpoint, region, bucket_lookup, bucket, access_key, secret_key,
		domain, path_prefix, status, max_file_size, allowed_types, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		c.ID, c.Name, c.Provider, c.Endpoint, c.Region, c.BucketLookup, c.Bucket,
		c.AccessKey, c.SecretKey, c.Domain, c.PathPrefix, c.Status,
		c.MaxFileSize, c.AllowedTypes, c.CreatedAt, c.UpdatedAt)
	if isUnique(err) {
		return domain.ErrConfigConflict
	}
	if err != nil {
		return fmt.Errorf("storage: create config: %w", err)
	}
	return nil
}

// Update 实现 application.StorageConfigRepo。
func (r *StorageConfigRepo) Update(ctx context.Context, c *domain.StorageConfig) error {
	tag, err := r.pool.Exec(ctx, `UPDATE storage_configs SET name=$2, provider=$3,
		endpoint=$4, region=$5, bucket_lookup=$6, bucket=$7, access_key=$8, secret_key=$9,
		domain=$10, path_prefix=$11, status=$12, max_file_size=$13, allowed_types=$14, updated_at=$15
		WHERE id=$1`, c.ID, c.Name, c.Provider, c.Endpoint, c.Region, c.BucketLookup,
		c.Bucket, c.AccessKey, c.SecretKey, c.Domain, c.PathPrefix, c.Status,
		c.MaxFileSize, c.AllowedTypes, c.UpdatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.ErrConfigConflict
		}
		return fmt.Errorf("storage: update config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConfigNotFound
	}
	return nil
}

// Delete 实现 application.StorageConfigRepo。
func (r *StorageConfigRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM storage_configs WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("storage: delete config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConfigNotFound
	}
	return nil
}

// Get 实现 application.StorageConfigRepo（pgx 无行 → 领域哨兵）。
func (r *StorageConfigRepo) Get(ctx context.Context, id string) (*domain.StorageConfig, error) {
	c, err := scanConfig(r.pool.QueryRow(ctx,
		`SELECT `+configCols+` FROM storage_configs WHERE id=$1`, id))
	if err != nil {
		if isNoRows(err) {
			return nil, application.ErrConfigNotFound
		}
		return nil, fmt.Errorf("storage: get config: %w", err)
	}
	return c, nil
}

// GetDefault 实现 application.StorageConfigRepo（部分唯一索引保证至多一行）。
func (r *StorageConfigRepo) GetDefault(ctx context.Context) (*domain.StorageConfig, error) {
	c, err := scanConfig(r.pool.QueryRow(ctx,
		`SELECT `+configCols+` FROM storage_configs WHERE is_default=true AND status=1 LIMIT 1`))
	if err != nil {
		if isNoRows(err) {
			return nil, application.ErrNoDefaultConfig
		}
		return nil, fmt.Errorf("storage: get default config: %w", err)
	}
	return c, nil
}

// SetDefault 实现 application.StorageConfigRepo（先清后设——单默认约束）。
func (r *StorageConfigRepo) SetDefault(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("storage: set default begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE storage_configs SET is_default=false WHERE is_default`); err != nil {
		return fmt.Errorf("storage: set default clear: %w", err)
	}
	tag, err := tx.Exec(ctx, `UPDATE storage_configs SET is_default=true, updated_at=now() WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("storage: set default: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConfigNotFound
	}
	return tx.Commit(ctx)
}

// List 实现 application.StorageConfigRepo。
func (r *StorageConfigRepo) List(ctx context.Context, offset, limit int) ([]*domain.StorageConfig, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM storage_configs`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("storage: count configs: %w", err)
	}
	rows, err := r.pool.Query(ctx, `SELECT `+configCols+` FROM storage_configs
		ORDER BY created_at DESC OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("storage: list configs: %w", err)
	}
	defer rows.Close()
	out := []*domain.StorageConfig{}
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

const recordCols = `id, config_id, object_key, file_name, file_size, mime_type,
	business_type, business_id, secret, status, file_url, uploader_id, uploader_ip,
	created_at, completed_at`

func scanRecord(row interface{ Scan(...any) error }) (*domain.UploadRecord, error) {
	var r domain.UploadRecord
	if err := row.Scan(&r.ID, &r.ConfigID, &r.ObjectKey, &r.FileName, &r.FileSize,
		&r.MimeType, &r.BusinessType, &r.BusinessID, &r.Secret, &r.Status,
		&r.FileURL, &r.UploaderID, &r.UploaderIP, &r.CreatedAt, &r.CompletedAt); err != nil {
		return nil, err
	}
	return &r, nil
}

// Create 实现 application.UploadRecordRepo。
func (r *UploadRecordRepo) Create(ctx context.Context, rec *domain.UploadRecord) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO storage_upload_records
		(id, config_id, object_key, file_name, file_size, mime_type, business_type,
		business_id, secret, status, uploader_id, uploader_ip, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		rec.ID, rec.ConfigID, rec.ObjectKey, rec.FileName, rec.FileSize, rec.MimeType,
		rec.BusinessType, rec.BusinessID, rec.Secret, rec.Status, rec.UploaderID,
		rec.UploaderIP, rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("storage: create record: %w", err)
	}
	return nil
}

// Get 实现 application.UploadRecordRepo（pgx 无行 → 领域哨兵）。
func (r *UploadRecordRepo) Get(ctx context.Context, id string) (*domain.UploadRecord, error) {
	rec, err := scanRecord(r.pool.QueryRow(ctx,
		`SELECT `+recordCols+` FROM storage_upload_records WHERE id=$1`, id))
	if err != nil {
		if isNoRows(err) {
			return nil, application.ErrRecordNotFound
		}
		return nil, fmt.Errorf("storage: get record: %w", err)
	}
	return rec, nil
}

// MarkCompleted 实现 application.UploadRecordRepo（CAS：pending→completed）。
func (r *UploadRecordRepo) MarkCompleted(ctx context.Context, id, fileURL string, completedAt time.Time) (bool, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE storage_upload_records SET status=2,
		file_url=$2, completed_at=$3 WHERE id=$1 AND status=1`, id, fileURL, completedAt)
	if err != nil {
		return false, fmt.Errorf("storage: mark completed: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// ExpireCandidates 实现 application.UploadRecordRepo（过期 pending 候选——
// cleanup 先删对象后置态，避免 S3 孤儿对象永久残留）。
func (r *UploadRecordRepo) ExpireCandidates(ctx context.Context, before time.Time) ([]*domain.UploadRecord, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+recordCols+` FROM storage_upload_records
		WHERE status=1 AND created_at < $1 ORDER BY created_at LIMIT 500`, before)
	if err != nil {
		return nil, fmt.Errorf("storage: expire candidates: %w", err)
	}
	defer rows.Close()
	var out []*domain.UploadRecord
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("storage: expire candidates scan: %w", err)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// ExpireOne 实现 application.UploadRecordRepo（单条 CAS pending→expired——
// 删除对象成功后逐条推进）。
func (r *UploadRecordRepo) ExpireOne(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE storage_upload_records SET status=3
		WHERE id=$1 AND status=1`, id)
	if err != nil {
		return fmt.Errorf("storage: expire one: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRecordNotFound
	}
	return nil
}

// List 实现 application.UploadRecordRepo。
func (r *UploadRecordRepo) List(ctx context.Context, offset, limit int) ([]*domain.UploadRecord, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM storage_upload_records`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("storage: count records: %w", err)
	}
	rows, err := r.pool.Query(ctx, `SELECT `+recordCols+` FROM storage_upload_records
		ORDER BY created_at DESC OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("storage: list records: %w", err)
	}
	defer rows.Close()
	out := []*domain.UploadRecord{}
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, rec)
	}
	return out, total, rows.Err()
}
