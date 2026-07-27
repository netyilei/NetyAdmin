package open_platform

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	"NetyAdmin/internal/domain/entity/open_platform"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/pagination"
)

type OpenLogRepository interface {
	Create(ctx context.Context, log *open_platform.OpenPlatformLog) error
	BatchCreate(ctx context.Context, logs []*open_platform.OpenPlatformLog) error
	List(ctx context.Context, query *OpenLogRepoQuery) ([]*open_platform.OpenPlatformLog, int64, error)
	GetByID(ctx context.Context, id uint64) (*open_platform.OpenPlatformLog, error)
	DeleteBatch(ctx context.Context, ids []uint64) error
	Clear(ctx context.Context, days int) error
	GetTrendStats(ctx context.Context, query *StatisticsRepoQuery) ([]*open_platform.TrendItem, error)
	GetTopAppsStats(ctx context.Context, query *StatisticsRepoQuery) ([]*open_platform.AppStatItem, error)
	GetTopApisStats(ctx context.Context, query *StatisticsRepoQuery) ([]*open_platform.ApiStatItem, error)
	GetStatusDistributionStats(ctx context.Context, query *StatisticsRepoQuery) ([]*open_platform.StatusDistItem, error)
	GetLatencyStats(ctx context.Context, query *StatisticsRepoQuery) (*open_platform.LatencyStats, error)
	GetOverviewStats(ctx context.Context, query *StatisticsRepoQuery) (*open_platform.OverviewStats, error)
}

type OpenLogRepoQuery struct {
	Page       int
	PageSize   int
	AppID      string
	AppKey     string
	ApiPath    string
	StatusCode *int
	StartTime  string
	EndTime    string
}

// StatisticsRepoQuery is the repository-level query input for statistics,
// decoupled from the interface-layer DTO (mirrors OpenLogRepoQuery pattern).
type StatisticsRepoQuery struct {
	Type        string
	StartTime   string
	EndTime     string
	AppID       string
	Granularity string
}

type openLogRepository struct {
	db *gorm.DB
}

func NewOpenLogRepository(db *gorm.DB) OpenLogRepository {
	return &openLogRepository{db: db}
}

// getDB 根据 context 中是否携带事务，返回事务内的 *gorm.DB 或回退到 r.db
func (r *openLogRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *openLogRepository) Create(ctx context.Context, log *open_platform.OpenPlatformLog) error {
	return r.getDB(ctx).Create(log).Error
}

func (r *openLogRepository) BatchCreate(ctx context.Context, logs []*open_platform.OpenPlatformLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.getDB(ctx).Create(&logs).Error
}

func (r *openLogRepository) List(ctx context.Context, query *OpenLogRepoQuery) ([]*open_platform.OpenPlatformLog, int64, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = entity.DefaultPageSize
	}

	var list []*open_platform.OpenPlatformLog
	var total int64
	db := r.getDB(ctx).Model(&open_platform.OpenPlatformLog{})

	if query.AppID != "" {
		db = db.Where("app_id = ?", query.AppID)
	}
	if query.AppKey != "" {
		db = db.Where("app_key = ?", query.AppKey)
	}
	if query.ApiPath != "" {
		db = db.Where("api_path LIKE ?", "%"+query.ApiPath+"%")
	}
	if query.StatusCode != nil {
		db = db.Where("status_code = ?", *query.StatusCode)
	}
	if query.StartTime != "" {
		db = db.Where("created_at >= ?", query.StartTime)
	}
	if query.EndTime != "" {
		db = db.Where("created_at <= ?", query.EndTime)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := db.Order("created_at DESC").Scopes(pagination.Paginate(query.Page, query.PageSize)).Find(&list).Error
	return list, total, err
}

func (r *openLogRepository) GetByID(ctx context.Context, id uint64) (*open_platform.OpenPlatformLog, error) {
	var log open_platform.OpenPlatformLog
	if err := r.getDB(ctx).First(&log, id).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *openLogRepository) DeleteBatch(ctx context.Context, ids []uint64) error {
	return r.getDB(ctx).Delete(&open_platform.OpenPlatformLog{}, ids).Error
}

func (r *openLogRepository) Clear(ctx context.Context, days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	return r.getDB(ctx).Where("created_at < ?", cutoff).Delete(&open_platform.OpenPlatformLog{}).Error
}

// buildWhereClause builds WHERE clause with parameterized args from StatisticsRepoQuery.
func (r *openLogRepository) buildWhereClause(query *StatisticsRepoQuery) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	if query.AppID != "" {
		conditions = append(conditions, "app_id = ?")
		args = append(args, query.AppID)
	}
	if query.StartTime != "" {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, query.StartTime)
	}
	if query.EndTime != "" {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, query.EndTime)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}
	return whereClause, args
}

// getTotalCount returns the total row count matching the WHERE clause.
func (r *openLogRepository) getTotalCount(db *gorm.DB, whereClause string, args []interface{}) (int64, error) {
	var total int64
	if err := db.Raw("SELECT COUNT(*) FROM sys_open_platform_logs "+whereClause, args...).Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// resolveGranularity returns the validated PostgreSQL DATE_TRUNC unit.
func resolveGranularity(g string) string {
	switch g {
	case "week":
		return "week"
	case "month":
		return "month"
	default:
		return "day"
	}
}

func (r *openLogRepository) GetTrendStats(ctx context.Context, query *StatisticsRepoQuery) ([]*open_platform.TrendItem, error) {
	whereClause, args := r.buildWhereClause(query)
	granularity := resolveGranularity(query.Granularity)

	// granularity validated to {day|week|month}, safe to interpolate as a fixed SQL keyword.
	sql := `
		SELECT
			to_char(DATE_TRUNC('` + granularity + `', created_at), 'YYYY-MM-DD') AS date,
			COUNT(*) AS total_calls,
			COUNT(*) FILTER (WHERE status_code >= 200 AND status_code < 300) AS success_calls,
			COUNT(*) FILTER (WHERE status_code < 200 OR status_code >= 300) AS fail_calls,
			COALESCE(AVG(latency) / 1000000.0, 0) AS avg_latency
		FROM sys_open_platform_logs
		` + whereClause + `
		GROUP BY DATE_TRUNC('` + granularity + `', created_at)
		ORDER BY date
	`

	var items []*open_platform.TrendItem
	if err := r.getDB(ctx).Raw(sql, args...).Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *openLogRepository) GetTopAppsStats(ctx context.Context, query *StatisticsRepoQuery) ([]*open_platform.AppStatItem, error) {
	db := r.getDB(ctx)
	whereClause, args := r.buildWhereClause(query)

	total, err := r.getTotalCount(db, whereClause, args)
	if err != nil {
		return nil, err
	}

	sql := `
		SELECT
			l.app_id,
			COALESCE(a.name, l.app_id) AS app_name,
			COUNT(*) AS calls
		FROM sys_open_platform_logs l
		LEFT JOIN sys_apps a ON a.id = l.app_id
		` + whereClause + `
		GROUP BY l.app_id, a.name
		ORDER BY calls DESC
		LIMIT 20
	`

	type appRow struct {
		AppID   string `json:"appId"`
		AppName string `json:"appName"`
		Calls   int64  `json:"calls"`
	}

	var rows []appRow
	if err := db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]*open_platform.AppStatItem, 0, len(rows))
	for _, row := range rows {
		percent := 0.0
		if total > 0 {
			percent = float64(row.Calls) / float64(total) * 100
		}
		items = append(items, &open_platform.AppStatItem{
			AppID:   row.AppID,
			AppName: row.AppName,
			Calls:   row.Calls,
			Percent: percent,
		})
	}
	return items, nil
}

func (r *openLogRepository) GetTopApisStats(ctx context.Context, query *StatisticsRepoQuery) ([]*open_platform.ApiStatItem, error) {
	db := r.getDB(ctx)
	whereClause, args := r.buildWhereClause(query)

	total, err := r.getTotalCount(db, whereClause, args)
	if err != nil {
		return nil, err
	}

	sql := `
		SELECT
			api_path,
			api_method,
			COUNT(*) AS calls
		FROM sys_open_platform_logs
		` + whereClause + `
		GROUP BY api_path, api_method
		ORDER BY calls DESC
		LIMIT 20
	`

	type apiRow struct {
		ApiPath   string `json:"apiPath"`
		ApiMethod string `json:"apiMethod"`
		Calls     int64  `json:"calls"`
	}

	var rows []apiRow
	if err := db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]*open_platform.ApiStatItem, 0, len(rows))
	for _, row := range rows {
		percent := 0.0
		if total > 0 {
			percent = float64(row.Calls) / float64(total) * 100
		}
		items = append(items, &open_platform.ApiStatItem{
			ApiPath:   row.ApiPath,
			ApiMethod: row.ApiMethod,
			Calls:     row.Calls,
			Percent:   percent,
		})
	}
	return items, nil
}

func (r *openLogRepository) GetStatusDistributionStats(ctx context.Context, query *StatisticsRepoQuery) ([]*open_platform.StatusDistItem, error) {
	db := r.getDB(ctx)
	whereClause, args := r.buildWhereClause(query)

	total, err := r.getTotalCount(db, whereClause, args)
	if err != nil {
		return nil, err
	}

	sql := `
		SELECT
			status_code,
			COUNT(*) AS calls
		FROM sys_open_platform_logs
		` + whereClause + `
		GROUP BY status_code
		ORDER BY status_code
	`

	type statusRow struct {
		StatusCode int   `json:"statusCode"`
		Calls      int64 `json:"calls"`
	}

	var rows []statusRow
	if err := db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]*open_platform.StatusDistItem, 0, len(rows))
	for _, row := range rows {
		percent := 0.0
		if total > 0 {
			percent = float64(row.Calls) / float64(total) * 100
		}
		items = append(items, &open_platform.StatusDistItem{
			StatusCode: row.StatusCode,
			Calls:      row.Calls,
			Percent:    percent,
		})
	}
	return items, nil
}

func (r *openLogRepository) GetLatencyStats(ctx context.Context, query *StatisticsRepoQuery) (*open_platform.LatencyStats, error) {
	whereClause, args := r.buildWhereClause(query)

	// All latency fields converted to milliseconds (ns / 1_000_000).
	sql := `
		SELECT
			COALESCE(AVG(latency) / 1000000.0, 0) AS avg_latency,
			COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY latency) / 1000000.0, 0) AS p50,
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency) / 1000000.0, 0) AS p95,
			COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency) / 1000000.0, 0) AS p99,
			COALESCE(MAX(latency) / 1000000.0, 0) AS max_latency
		FROM sys_open_platform_logs
		` + whereClause

	var stats open_platform.LatencyStats
	if err := r.getDB(ctx).Raw(sql, args...).Scan(&stats).Error; err != nil {
		return nil, err
	}
	return &stats, nil
}

func (r *openLogRepository) GetOverviewStats(ctx context.Context, query *StatisticsRepoQuery) (*open_platform.OverviewStats, error) {
	whereClause, args := r.buildWhereClause(query)

	sql := `
		SELECT
			COUNT(*) AS total_calls,
			COUNT(*) FILTER (WHERE status_code >= 200 AND status_code < 300) AS success_calls,
			COUNT(*) FILTER (WHERE status_code < 200 OR status_code >= 300) AS fail_calls,
			COALESCE(AVG(latency) / 1000000.0, 0) AS avg_latency,
			COUNT(DISTINCT app_id) AS app_count,
			COUNT(DISTINCT api_path || ':' || api_method) AS api_count
		FROM sys_open_platform_logs
		` + whereClause

	var stats open_platform.OverviewStats
	if err := r.getDB(ctx).Raw(sql, args...).Scan(&stats).Error; err != nil {
		return nil, err
	}
	return &stats, nil
}
