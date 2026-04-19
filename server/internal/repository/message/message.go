package message

import (
	"context"
	"time"

	"gorm.io/gorm"

	msgEntity "NetyAdmin/internal/domain/entity/message"
)

type MsgRepository interface {
	// Template
	CreateTemplate(ctx context.Context, tpl *msgEntity.MsgTemplate) error
	UpdateTemplate(ctx context.Context, tpl *msgEntity.MsgTemplate) error
	DeleteTemplate(ctx context.Context, id uint64) error
	GetTemplateByCode(ctx context.Context, code string) (*msgEntity.MsgTemplate, error)
	ListTemplates(ctx context.Context, query *MsgRepoQuery) ([]*msgEntity.MsgTemplate, int64, error)

	// Record
	CreateRecord(ctx context.Context, rec *msgEntity.MsgRecord) error
	UpdateRecord(ctx context.Context, rec *msgEntity.MsgRecord) error
	GetRecordByID(ctx context.Context, id uint64) (*msgEntity.MsgRecord, error)
	ListRecords(ctx context.Context, query *MsgRepoQuery) ([]*msgEntity.MsgRecord, int64, error)
	DeleteRecordsBefore(ctx context.Context, before time.Time) error
}

type MsgRepoQuery struct {
	Page     int
	PageSize int
	Channel  string
	Code     string
	Name     string
	Status   *int
	Receiver string
	UserID   uint64
}

type msgRepository struct {
	db *gorm.DB
}

func NewMsgRepository(db *gorm.DB) MsgRepository {
	return &msgRepository{db: db}
}

// Template implementations
func (r *msgRepository) CreateTemplate(ctx context.Context, tpl *msgEntity.MsgTemplate) error {
	return r.db.WithContext(ctx).Create(tpl).Error
}

func (r *msgRepository) UpdateTemplate(ctx context.Context, tpl *msgEntity.MsgTemplate) error {
	return r.db.WithContext(ctx).Save(tpl).Error
}

func (r *msgRepository) DeleteTemplate(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&msgEntity.MsgTemplate{}, id).Error
}

func (r *msgRepository) GetTemplateByCode(ctx context.Context, code string) (*msgEntity.MsgTemplate, error) {
	var tpl msgEntity.MsgTemplate
	err := r.db.WithContext(ctx).Where("code = ? AND status = 1", code).First(&tpl).Error
	return &tpl, err
}

func (r *msgRepository) ListTemplates(ctx context.Context, query *MsgRepoQuery) ([]*msgEntity.MsgTemplate, int64, error) {
	var list []*msgEntity.MsgTemplate
	var total int64
	db := r.db.WithContext(ctx).Model(&msgEntity.MsgTemplate{})

	if query.Channel != "" {
		db = db.Where("channel = ?", query.Channel)
	}
	if query.Code != "" {
		db = db.Where("code LIKE ?", "%"+query.Code+"%")
	}
	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if query.Page > 0 && query.PageSize > 0 {
		db = db.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize)
	}

	err := db.Order("created_at DESC").Find(&list).Error
	return list, total, err
}

func (r *msgRepository) DeleteRecordsBefore(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).Where("created_at < ?", before).Delete(&msgEntity.MsgRecord{}).Error
}

// Record implementations
func (r *msgRepository) CreateRecord(ctx context.Context, rec *msgEntity.MsgRecord) error {
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *msgRepository) UpdateRecord(ctx context.Context, rec *msgEntity.MsgRecord) error {
	return r.db.WithContext(ctx).Save(rec).Error
}

func (r *msgRepository) GetRecordByID(ctx context.Context, id uint64) (*msgEntity.MsgRecord, error) {
	var rec msgEntity.MsgRecord
	err := r.db.WithContext(ctx).First(&rec, id).Error
	return &rec, err
}

func (r *msgRepository) ListRecords(ctx context.Context, query *MsgRepoQuery) ([]*msgEntity.MsgRecord, int64, error) {
	var list []*msgEntity.MsgRecord
	var total int64
	db := r.db.WithContext(ctx).Model(&msgEntity.MsgRecord{})

	if query.Channel != "" {
		db = db.Where("channel = ?", query.Channel)
	}
	if query.Receiver != "" {
		db = db.Where("receiver LIKE ?", "%"+query.Receiver+"%")
	}
	if query.UserID != 0 {
		db = db.Where("user_id = ?", query.UserID)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if query.Page > 0 && query.PageSize > 0 {
		db = db.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize)
	}

	err := db.Order("created_at DESC").Find(&list).Error
	return list, total, err
}
