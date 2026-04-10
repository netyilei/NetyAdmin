package content

import (
	"NetyAdmin/internal/domain/entity"
	"time"
)

type PublishStatus string

const (
	PublishStatusDraft     PublishStatus = "draft"
	PublishStatusPublished PublishStatus = "published"
	PublishStatusScheduled PublishStatus = "scheduled"
)

type LinkType string

const (
	LinkTypeNone     LinkType = "none"
	LinkTypeInternal LinkType = "internal"
	LinkTypeExternal LinkType = "external"
	LinkTypeArticle  LinkType = "article"
)

type ContentArticle struct {
	entity.Model
	entity.Operator
	CategoryID    uint          `gorm:"column:category_id;not null;comment:分类ID" json:"categoryId"`
	Title         string        `gorm:"column:title;type:varchar(200);not null;comment:文章标题" json:"title"`
	TitleColor    string        `gorm:"column:title_color;type:varchar(20);default:'#333333';comment:标题颜色" json:"titleColor"`
	CoverImage    string        `gorm:"column:cover_image;type:varchar(500);comment:封面图片URL" json:"coverImage"`
	Summary       string        `gorm:"column:summary;type:varchar(500);comment:文章摘要" json:"summary"`
	Content       string        `gorm:"column:content;type:text;comment:文章内容" json:"content"`
	ContentType   ContentType   `gorm:"column:content_type;type:varchar(20);default:'richtext';comment:内容类型" json:"contentType"`
	Author        string        `gorm:"column:author;type:varchar(50);comment:作者" json:"author"`
	Source        string        `gorm:"column:source;type:varchar(100);comment:来源" json:"source"`
	Keywords      string        `gorm:"column:keywords;type:varchar(200);comment:关键词" json:"keywords"`
	Tags          string        `gorm:"column:tags;type:varchar(200);comment:标签" json:"tags"`
	IsTop         bool          `gorm:"column:is_top;default:false;comment:是否置顶" json:"isTop"`
	TopSort       int           `gorm:"column:top_sort;default:0;comment:置顶排序" json:"topSort"`
	IsHot         bool          `gorm:"column:is_hot;default:false;comment:是否热门" json:"isHot"`
	IsRecommend   bool          `gorm:"column:is_recommend;default:false;comment:是否推荐" json:"isRecommend"`
	AllowComment  bool          `gorm:"column:allow_comment;default:true;comment:是否允许评论" json:"allowComment"`
	ViewCount     int           `gorm:"column:view_count;default:0;comment:浏览次数" json:"viewCount"`
	LikeCount     int           `gorm:"column:like_count;default:0;comment:点赞次数" json:"likeCount"`
	CommentCount  int           `gorm:"column:comment_count;default:0;comment:评论次数" json:"commentCount"`
	PublishStatus PublishStatus `gorm:"column:publish_status;type:varchar(20);default:'draft';comment:发布状态" json:"publishStatus"`
	PublishedAt   *time.Time    `gorm:"column:published_at;comment:发布时间" json:"publishedAt"`
	ScheduledAt   *time.Time    `gorm:"column:scheduled_at;comment:定时发布时间" json:"scheduledAt"`
	Sort          int           `gorm:"column:sort;default:0;comment:排序" json:"sort"`
	Status        string        `gorm:"column:status;type:char(1);default:'1';comment:状态 1启用 0禁用" json:"status"`
	Remark        string        `gorm:"column:remark;type:text;comment:备注" json:"remark"`

	Category *ContentCategory `gorm:"foreignKey:CategoryID;references:ID" json:"category,omitempty"`
}

func (ContentArticle) TableName() string {
	return "content_article"
}

func (a *ContentArticle) IsEnabled() bool {
	return a.Status == "1"
}

func (a *ContentArticle) IsPublished() bool {
	return a.PublishStatus == PublishStatusPublished
}

func (a *ContentArticle) IsScheduled() bool {
	return a.PublishStatus == PublishStatusScheduled && a.ScheduledAt != nil
}

func (a *ContentArticle) ShouldPublishNow() bool {
	if a.PublishStatus != PublishStatusScheduled || a.ScheduledAt == nil {
		return false
	}
	return a.ScheduledAt.Before(time.Now()) || a.ScheduledAt.Equal(time.Now())
}
