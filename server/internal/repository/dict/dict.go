package dict

import (
	"context"
	"gorm.io/gorm"
	dictEntity "NetyAdmin/internal/domain/entity/dict"
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

func (r *dictRepository) CreateType(ctx context.Context, t *dictEntity.DictType) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *dictRepository) UpdateType(ctx context.Context, t *dictEntity.DictType) error {
	return r.db.WithContext(ctx).Save(t).Error
}

func (r *dictRepository) DeleteType(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&dictEntity.DictType{}, id).Error
}

func (r *dictRepository) GetTypeById(ctx context.Context, id uint) (*dictEntity.DictType, error) {
	var t dictEntity.DictType
	err := r.db.WithContext(ctx).First(&t, id).Error
	return &t, err
}

func (r *dictRepository) GetTypeByCode(ctx context.Context, code string) (*dictEntity.DictType, error) {
	var t dictEntity.DictType
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&t).Error
	return &t, err
}

func (r *dictRepository) ListType(ctx context.Context, name, code, status string, page, pageSize int) ([]dictEntity.DictType, int64, error) {
	var list []dictEntity.DictType
	var total int64
	query := r.db.WithContext(ctx).Model(&dictEntity.DictType{})
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
	err = query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *dictRepository) CreateData(ctx context.Context, d *dictEntity.DictData) error {
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *dictRepository) UpdateData(ctx context.Context, d *dictEntity.DictData) error {
	return r.db.WithContext(ctx).Save(d).Error
}

func (r *dictRepository) DeleteData(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&dictEntity.DictData{}, id).Error
}

func (r *dictRepository) GetDataById(ctx context.Context, id uint) (*dictEntity.DictData, error) {
	var d dictEntity.DictData
	err := r.db.WithContext(ctx).First(&d, id).Error
	return &d, err
}

func (r *dictRepository) ListData(ctx context.Context, dictCode string) ([]dictEntity.DictData, error) {
	var list []dictEntity.DictData
	err := r.db.WithContext(ctx).Where("dict_code = ? AND status = '1'", dictCode).Order("order_by ASC").Find(&list).Error
	return list, err
}

func (r *dictRepository) ListDataFull(ctx context.Context, dictCode, label, status string, page, pageSize int) ([]dictEntity.DictData, int64, error) {
	var list []dictEntity.DictData
	var total int64
	query := r.db.WithContext(ctx).Model(&dictEntity.DictData{})
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
	err = query.Order("order_by ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}
