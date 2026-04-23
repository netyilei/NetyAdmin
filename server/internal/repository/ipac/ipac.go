package ipac

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity/ipac"
	"NetyAdmin/internal/pkg/pagination"
)

type IPACRepository interface {
	Create(ctx context.Context, item *ipac.IPAccessControl) error
	Update(ctx context.Context, item *ipac.IPAccessControl) error
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*ipac.IPAccessControl, error)
	List(ctx context.Context, query *IPACQuery) ([]*ipac.IPAccessControl, int64, error)
	GetByIP(ctx context.Context, ip string, appID *string) (*ipac.IPAccessControl, error)
	GetAllEffective(ctx context.Context) ([]*ipac.IPAccessControl, error)
	DeleteBatch(ctx context.Context, ids []uint) error
	GetAppIPFilterEnabled(ctx context.Context) (map[string]bool, error)
	LinkRulesToApp(ctx context.Context, appID string, ruleIDs []uint) error
}

type IPACQuery struct {
	AppID    *string
	IPAddr   string
	Type     int
	Status   *int
	Page     int
	PageSize int
}

type ipacRepository struct {
	db *gorm.DB
}

func NewIPACRepository(db *gorm.DB) IPACRepository {
	return &ipacRepository{db: db}
}

func (r *ipacRepository) Create(ctx context.Context, item *ipac.IPAccessControl) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ipacRepository) Update(ctx context.Context, item *ipac.IPAccessControl) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *ipacRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&ipac.IPAccessControl{}, id).Error
}

func (r *ipacRepository) GetByID(ctx context.Context, id uint) (*ipac.IPAccessControl, error) {
	var item ipac.IPAccessControl
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

func (r *ipacRepository) List(ctx context.Context, query *IPACQuery) ([]*ipac.IPAccessControl, int64, error) {
	var list []*ipac.IPAccessControl
	var total int64
	db := r.db.WithContext(ctx).Model(&ipac.IPAccessControl{})

	if query.AppID != nil && *query.AppID != "" {
		db = db.Where("app_id = ?", query.AppID)
	} else {
		db = db.Where("app_id IS NULL OR app_id = ''")
	}

	if query.IPAddr != "" {
		db = db.Where("ip_addr LIKE ?", "%"+query.IPAddr+"%")
	}

	if query.Type > 0 {
		db = db.Where("type = ?", query.Type)
	}

	if query.Status != nil {
		db = db.Where("status = ?", query.Status)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if query.Page > 0 && query.PageSize > 0 {
		db = db.Scopes(pagination.Paginate(query.Page, query.PageSize))
	}

	err := db.Order("created_at DESC").Find(&list).Error
	return list, total, err
}

func (r *ipacRepository) GetByIP(ctx context.Context, ip string, appID *string) (*ipac.IPAccessControl, error) {
	var item ipac.IPAccessControl
	db := r.db.WithContext(ctx).Where("ip_addr = ?", ip)
	if appID != nil && *appID != "" {
		db = db.Where("app_id = ?", appID)
	} else {
		db = db.Where("app_id IS NULL OR app_id = ''")
	}

	err := db.First(&item).Error
	return &item, err
}

func (r *ipacRepository) GetAllEffective(ctx context.Context) ([]*ipac.IPAccessControl, error) {
	var list []*ipac.IPAccessControl
	err := r.db.WithContext(ctx).Where("status = ?", ipac.IPACStatusEnabled).
		Where("expired_at IS NULL OR expired_at > NOW()").
		Find(&list).Error
	return list, err
}

func (r *ipacRepository) DeleteBatch(ctx context.Context, ids []uint) error {
	return r.db.WithContext(ctx).Delete(&ipac.IPAccessControl{}, ids).Error
}

func (r *ipacRepository) GetAppIPFilterEnabled(ctx context.Context) (map[string]bool, error) {
	type appFilter struct {
		ID string `gorm:"primaryKey"`
	}
	var list []appFilter
	err := r.db.WithContext(ctx).Table("sys_apps").
		Select("id").
		Where("ip_filter_enabled = ? AND deleted_at = 0", true).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(list))
	for _, item := range list {
		result[item.ID] = true
	}
	return result, nil
}

func (r *ipacRepository) LinkRulesToApp(ctx context.Context, appID string, ruleIDs []uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ipac.IPAccessControl{}).
			Where("app_id = ?", appID).
			Update("app_id", nil).Error; err != nil {
			return err
		}
		if len(ruleIDs) > 0 {
			if err := tx.Model(&ipac.IPAccessControl{}).
				Where("id IN ?", ruleIDs).
				Update("app_id", appID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
