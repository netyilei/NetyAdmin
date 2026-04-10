package content

import "silentorder/internal/domain/entity"

type ContentBannerGroup struct {
	entity.Model
	entity.Operator
	Name        string `gorm:"column:name;type:varchar(100);not null;comment:Banner组名称" json:"name"`
	Code        string `gorm:"column:code;type:varchar(50);not null;comment:Banner组编码" json:"code"`
	Description string `gorm:"column:description;type:varchar(255);comment:描述" json:"description"`
	Position    string `gorm:"column:position;type:varchar(50);comment:位置标识" json:"position"`
	Width       int    `gorm:"column:width;comment:建议宽度" json:"width"`
	Height      int    `gorm:"column:height;comment:建议高度" json:"height"`
	MaxItems    int    `gorm:"column:max_items;default:10;comment:最大Banner数量" json:"maxItems"`
	AutoPlay    bool   `gorm:"column:auto_play;default:true;comment:是否自动播放" json:"autoPlay"`
	Interval        int    `gorm:"column:interval;default:5000;comment:轮播间隔(毫秒)" json:"interval"`
	Sort            int    `gorm:"column:sort;default:0;comment:排序" json:"sort"`
	StorageConfigID *uint  `gorm:"column:storage_config_id;comment:绑定的对象存储ID" json:"storageConfigId"`
	Status          string `gorm:"column:status;type:char(1);default:'1';comment:状态 1启用 0禁用" json:"status"`
	Remark      string `gorm:"column:remark;type:text;comment:备注" json:"remark"`

	Banners []ContentBannerItem `gorm:"foreignKey:GroupID;references:ID" json:"banners,omitempty"`
}

func (ContentBannerGroup) TableName() string {
	return "content_banner_group"
}

func (g *ContentBannerGroup) IsEnabled() bool {
	return g.Status == "1"
}
