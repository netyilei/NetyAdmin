package content

import "NetyAdmin/internal/domain/entity"

type ContentType string

const (
	ContentTypePlainText ContentType = "plaintext"
	ContentTypeRichText  ContentType = "richtext"
)

type ContentCategory struct {
	entity.Model
	entity.Operator
	ParentID    uint        `gorm:"column:parent_id;default:0;comment:父级分类ID" json:"parentId"`
	Name        string      `gorm:"column:name;type:varchar(50);not null;comment:分类名称" json:"name"`
	Code        string      `gorm:"column:code;type:varchar(50);comment:分类编码" json:"code"`
	Icon        string      `gorm:"column:icon;type:varchar(100);comment:分类图标" json:"icon"`
	Sort            int         `gorm:"column:sort;default:0;comment:排序" json:"sort"`
	StorageConfigID *uint       `gorm:"column:storage_config_id;comment:绑定的对象存储ID" json:"storageConfigId"`
	ContentType     ContentType `gorm:"column:content_type;type:varchar(20);default:'richtext';comment:内容类型" json:"contentType"`
	Status      string      `gorm:"column:status;type:char(1);default:'1';comment:状态 1启用 0禁用" json:"status"`
	Remark      string      `gorm:"column:remark;type:text;comment:备注" json:"remark"`

	Children []ContentCategory `gorm:"foreignKey:ParentID;references:ID" json:"children,omitempty"`
}

func (ContentCategory) TableName() string {
	return "content_category"
}

func (c *ContentCategory) IsEnabled() bool {
	return c.Status == entity.StatusEnabled
}

func (c *ContentCategory) IsPlainText() bool {
	return c.ContentType == ContentTypePlainText
}
