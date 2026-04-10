package content

import (
	"NetyAdmin/internal/domain/entity"
	"time"
)

type ContentBannerItem struct {
	entity.Model
	entity.Operator
	GroupID       uint       `gorm:"column:group_id;not null;comment:Banner组ID" json:"groupId"`
	Title         string     `gorm:"column:title;type:varchar(200);not null;comment:Banner标题" json:"title"`
	Subtitle      string     `gorm:"column:subtitle;type:varchar(200);comment:副标题" json:"subtitle"`
	ImageURL      string     `gorm:"column:image_url;type:varchar(500);not null;comment:图片URL" json:"imageUrl"`
	ImageAlt      string     `gorm:"column:image_alt;type:varchar(200);comment:图片描述" json:"imageAlt"`
	LinkType      LinkType   `gorm:"column:link_type;type:varchar(20);default:'none';comment:链接类型" json:"linkType"`
	LinkURL       string     `gorm:"column:link_url;type:varchar(500);comment:外部链接地址" json:"linkUrl"`
	LinkArticleID *uint      `gorm:"column:link_article_id;comment:关联文章ID" json:"linkArticleId"`
	Content       string     `gorm:"column:content;type:text;comment:纯文本内容" json:"content"`
	CustomParams  string     `gorm:"column:custom_params;type:text;comment:自定义参数JSON" json:"customParams"`
	Sort          int        `gorm:"column:sort;default:0;comment:排序" json:"sort"`
	StartTime     *time.Time `gorm:"column:start_time;comment:开始显示时间" json:"startTime"`
	EndTime       *time.Time `gorm:"column:end_time;comment:结束显示时间" json:"endTime"`
	ViewCount     int        `gorm:"column:view_count;default:0;comment:浏览次数" json:"viewCount"`
	ClickCount    int        `gorm:"column:click_count;default:0;comment:点击次数" json:"clickCount"`
	Status        string     `gorm:"column:status;type:char(1);default:'1';comment:状态 1启用 0禁用" json:"status"`
	Remark        string     `gorm:"column:remark;type:text;comment:备注" json:"remark"`

	Group   *ContentBannerGroup `gorm:"foreignKey:GroupID;references:ID" json:"group,omitempty"`
	Article *ContentArticle     `gorm:"foreignKey:LinkArticleID;references:ID" json:"article,omitempty"`
}

func (ContentBannerItem) TableName() string {
	return "content_banner_item"
}

func (b *ContentBannerItem) IsEnabled() bool {
	return b.Status == "1"
}

func (b *ContentBannerItem) IsInTimeRange() bool {
	now := time.Now()
	if b.StartTime != nil && now.Before(*b.StartTime) {
		return false
	}
	if b.EndTime != nil && now.After(*b.EndTime) {
		return false
	}
	return true
}
