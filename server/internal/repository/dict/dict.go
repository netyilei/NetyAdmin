package dict

import (
	"context"

	"gorm.io/gorm"

	"NetyAdmin/internal/domain/entity"
	dictEntity "NetyAdmin/internal/domain/entity/dict"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/pagination"
)

type DictRepository interface {
	// 类型管理
	CreateType(ctx context.Context, t *dictEntity.DictType) error
	UpdateType(ctx context.Context, t *dictEntity.DictType) error
	DeleteType(ctx context.Context, id uint) error
	GetTypeById(ctx context.Context, id uint) (*dictEntity.DictType, error)
	GetTypeByCode(ctx context.Context, code string) (*dictEntity.DictType, error)
	ListType(ctx context.Context, name, code, status string, page, pageSize int) ([]dictEntity.DictType, int64, error)

	// 数据管理
	CreateData(ctx context.Context, d *dictEntity.DictData) error
	UpdateData(ctx context.Context, d *dictEntity.DictData) error
	DeleteData(ctx context.Context, id uint) error
	DeleteDataByTypeCode(ctx context.Context, typeCode string) error
	GetDataById(ctx context.Context, id uint) (*dictEntity.DictData, error)
	ListData(ctx context.Context, dictCode string) ([]dictEntity.DictData, error)
	ListDataFull(ctx context.Context, dictCode, label, status string, page, pageSize int) ([]dictEntity.DictData, int64, error)
}

type dictRepository struct {
	db *gorm.DB
}

func NewDictRepository(db *gorm.DB) DictRepository {
	return &dictRepository{db: db}
}

// getDB 从 context 中获取事务 DB，若不存在则回退到默认 db。
func (r *dictRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDB(ctx, r.db)
}

func (r *dictRepository) CreateType(ctx context.Context, t *dictEntity.DictType) error {
	return r.getDB(ctx).Create(t).Error
}

func (r *dictRepository) UpdateType(ctx context.Context, t *dictEntity.DictType) error {
	return r.getDB(ctx).Save(t).Error
}

func (r *dictRepository) DeleteType(ctx context.Context, id uint) error {
	return r.getDB(ctx).Delete(&dictEntity.DictType{}, id).Error
}

func (r *dictRepository) GetTypeById(ctx context.Context, id uint) (*dictEntity.DictType, error) {
	var t dictEntity.DictType
	if err := r.getDB(ctx).First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *dictRepository) GetTypeByCode(ctx context.Context, code string) (*dictEntity.DictType, error) {
	var t dictEntity.DictType
	if err := r.getDB(ctx).Where("code = ?", code).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *dictRepository) ListType(ctx context.Context, name, code, status string, page, pageSize int) ([]dictEntity.DictType, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = entity.DefaultPageSize
	}

	var list []dictEntity.DictType
	var total int64
	query := r.getDB(ctx).Model(&dictEntity.DictType{})
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if code != "" {
		query = query.Where("code LIKE ?", "%"+code+"%")
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = query.Scopes(pagination.Paginate(page, pageSize)).Find(&list).Error
	return list, total, err
}

func (r *dictRepository) CreateData(ctx context.Context, d *dictEntity.DictData) error {
	return r.getDB(ctx).Create(d).Error
}

func (r *dictRepository) UpdateData(ctx context.Context, d *dictEntity.DictData) error {
	return r.getDB(ctx).Save(d).Error
}

func (r *dictRepository) DeleteData(ctx context.Context, id uint) error {
	return r.getDB(ctx).Delete(&dictEntity.DictData{}, id).Error
}

func (r *dictRepository) DeleteDataByTypeCode(ctx context.Context, typeCode string) error {
	return r.getDB(ctx).Where("dict_code = ?", typeCode).Delete(&dictEntity.DictData{}).Error
}

func (r *dictRepository) GetDataById(ctx context.Context, id uint) (*dictEntity.DictData, error) {
	var d dictEntity.DictData
	if err := r.getDB(ctx).First(&d, id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *dictRepository) ListData(ctx context.Context, dictCode string) ([]dictEntity.DictData, error) {
	var list []dictEntity.DictData
	err := r.getDB(ctx).Where("dict_code = ? AND status = ?", dictCode, entity.StatusEnabled).Order("order_by ASC").Find(&list).Error
	return list, err
}

func (r *dictRepository) ListDataFull(ctx context.Context, dictCode, label, status string, page, pageSize int) ([]dictEntity.DictData, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = entity.DefaultPageSize
	}

	var list []dictEntity.DictData
	var total int64
	query := r.getDB(ctx).Model(&dictEntity.DictData{})
	if dictCode != "" {
		query = query.Where("dict_code = ?", dictCode)
	}
	if label != "" {
		query = query.Where("label LIKE ?", "%"+label+"%")
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = query.Order("order_by ASC").Scopes(pagination.Paginate(page, pageSize)).Find(&list).Error
	return list, total, err
}
