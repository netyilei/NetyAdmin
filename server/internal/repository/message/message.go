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

	// Internal
	CreateInternal(ctx context.Context, msg *msgEntity.MsgInternal) error

	// Client Internal Message
	ListUserInternalMsgs(ctx context.Context, userID string, page, pageSize int, readFilter *int) ([]*UserInternalMsg, int64, error)
	GetInternalMsgDetail(ctx context.Context, msgInternalID uint64, userID string) (*UserInternalMsg, error)
	MarkInternalMsgRead(ctx context.Context, msgInternalID uint64, userID string) error
	MarkAllInternalMsgRead(ctx context.Context, userID string) error
	CountUnreadInternalMsgs(ctx context.Context, userID string) (int64, error)
}

type MsgRepoQuery struct {
	Page     int
	PageSize int
	Channel  string
	Code     string
	Name     string
	Status   *int
	Receiver string
	UserID   string
}

type UserInternalMsg struct {
	MsgInternalID uint64     `json:"msgInternalId"`
	MsgRecordID   uint64     `json:"msgRecordId"`
	Type          int        `json:"type"`
	Title         string     `json:"title"`
	Content       string     `json:"content"`
	IsRead        bool       `json:"isRead"`
	ReadAt        *time.Time `json:"readAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
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

func (r *msgRepository) ListUserInternalMsgs(ctx context.Context, userID string, page, pageSize int, readFilter *int) ([]*UserInternalMsg, int64, error) {
	var total int64

	countDB := r.db.WithContext(ctx).
		Table("msg_internal mi").
		Joins("JOIN msg_records mr ON mi.msg_record_id = mr.id").
		Joins("LEFT JOIN msg_internal_reads mir ON mir.msg_internal_id = mi.id AND mir.user_id = ?", userID).
		Where("mr.channel = ? AND mr.status = ? AND (mr.user_id = ? OR mi.type = 1)", "internal", msgEntity.MsgStatusSuccess, userID)

	if readFilter != nil {
		if *readFilter == 1 {
			countDB = countDB.Where("mir.id IS NOT NULL")
		} else if *readFilter == 0 {
			countDB = countDB.Where("mir.id IS NULL")
		}
	}

	if err := countDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var results []*UserInternalMsg
	queryDB := r.db.WithContext(ctx).
		Table("msg_internal mi").
		Select("mi.id as msg_internal_id, mi.msg_record_id, mi.type, mr.title, mr.content, CASE WHEN mir.id IS NOT NULL THEN true ELSE false END as is_read, mir.read_at, mr.created_at").
		Joins("JOIN msg_records mr ON mi.msg_record_id = mr.id").
		Joins("LEFT JOIN msg_internal_reads mir ON mir.msg_internal_id = mi.id AND mir.user_id = ?", userID).
		Where("mr.channel = ? AND mr.status = ? AND (mr.user_id = ? OR mi.type = 1)", "internal", msgEntity.MsgStatusSuccess, userID)

	if readFilter != nil {
		if *readFilter == 1 {
			queryDB = queryDB.Where("mir.id IS NOT NULL")
		} else if *readFilter == 0 {
			queryDB = queryDB.Where("mir.id IS NULL")
		}
	}

	if page > 0 && pageSize > 0 {
		queryDB = queryDB.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	if err := queryDB.Order("mr.created_at DESC").Scan(&results).Error; err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

func (r *msgRepository) GetInternalMsgDetail(ctx context.Context, msgInternalID uint64, userID string) (*UserInternalMsg, error) {
	var result UserInternalMsg
	err := r.db.WithContext(ctx).
		Table("msg_internal mi").
		Select("mi.id as msg_internal_id, mi.msg_record_id, mi.type, mr.title, mr.content, CASE WHEN mir.id IS NOT NULL THEN true ELSE false END as is_read, mir.read_at, mr.created_at").
		Joins("JOIN msg_records mr ON mi.msg_record_id = mr.id").
		Joins("LEFT JOIN msg_internal_reads mir ON mir.msg_internal_id = mi.id AND mir.user_id = ?", userID).
		Where("mi.id = ? AND mr.channel = ? AND mr.status = ? AND (mr.user_id = ? OR mi.type = 1)", msgInternalID, "internal", msgEntity.MsgStatusSuccess, userID).
		Take(&result).Error

	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *msgRepository) MarkInternalMsgRead(ctx context.Context, msgInternalID uint64, userID string) error {
	var existing msgEntity.MsgInternalRead
	err := r.db.WithContext(ctx).
		Where("msg_internal_id = ? AND user_id = ?", msgInternalID, userID).
		First(&existing).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	read := &msgEntity.MsgInternalRead{
		MsgInternalID: msgInternalID,
		UserID:        userID,
		ReadAt:        time.Now(),
	}
	return r.db.WithContext(ctx).Create(read).Error
}

func (r *msgRepository) MarkAllInternalMsgRead(ctx context.Context, userID string) error {
	var unreadIDs []uint64
	err := r.db.WithContext(ctx).
		Table("msg_internal mi").
		Select("mi.id").
		Joins("JOIN msg_records mr ON mi.msg_record_id = mr.id").
		Joins("LEFT JOIN msg_internal_reads mir ON mir.msg_internal_id = mi.id AND mir.user_id = ?", userID).
		Where("mr.channel = ? AND mr.status = ? AND (mr.user_id = ? OR mi.type = 1) AND mir.id IS NULL", "internal", msgEntity.MsgStatusSuccess, userID).
		Scan(&unreadIDs).Error

	if err != nil {
		return err
	}

	if len(unreadIDs) == 0 {
		return nil
	}

	reads := make([]*msgEntity.MsgInternalRead, 0, len(unreadIDs))
	now := time.Now()
	for _, id := range unreadIDs {
		reads = append(reads, &msgEntity.MsgInternalRead{
			MsgInternalID: id,
			UserID:        userID,
			ReadAt:        now,
		})
	}

	return r.db.WithContext(ctx).CreateInBatches(reads, 100).Error
}

func (r *msgRepository) CountUnreadInternalMsgs(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("msg_internal mi").
		Joins("JOIN msg_records mr ON mi.msg_record_id = mr.id").
		Joins("LEFT JOIN msg_internal_reads mir ON mir.msg_internal_id = mi.id AND mir.user_id = ?", userID).
		Where("mr.channel = ? AND mr.status = ? AND (mr.user_id = ? OR mi.type = 1) AND mir.id IS NULL", "internal", msgEntity.MsgStatusSuccess, userID).
		Count(&count).Error

	return count, err
}

func (r *msgRepository) CreateInternal(ctx context.Context, msg *msgEntity.MsgInternal) error {
	return r.db.WithContext(ctx).Create(msg).Error
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
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if query.UserID != "" {
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
