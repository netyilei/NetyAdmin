package content

import "time"

type ContentArticleDTO struct {
	ID            uint       `json:"id"`
	CategoryID    uint       `json:"categoryId"`
	CategoryName  string     `json:"categoryName"`
	Title         string     `json:"title"`
	TitleColor    string     `json:"titleColor"`
	CoverImage    string     `json:"coverImage"`
	Summary       string     `json:"summary"`
	Content       string     `json:"content"`
	ContentType   string     `json:"contentType"`
	Author        string     `json:"author"`
	Source        string     `json:"source"`
	Keywords      string     `json:"keywords"`
	Tags          string     `json:"tags"`
	IsTop         bool       `json:"isTop"`
	TopSort       int        `json:"topSort"`
	IsHot         bool       `json:"isHot"`
	IsRecommend   bool       `json:"isRecommend"`
	AllowComment  bool       `json:"allowComment"`
	ViewCount     int        `json:"viewCount"`
	LikeCount     int        `json:"likeCount"`
	CommentCount  int        `json:"commentCount"`
	PublishStatus string     `json:"publishStatus"`
	PublishedAt   *time.Time `json:"publishedAt"`
	ScheduledAt   *time.Time `json:"scheduledAt"`
	CreatedBy     uint       `json:"createdBy"`
	UpdatedBy     uint       `json:"updatedBy"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type CreateContentArticleDTO struct {
	CategoryID    uint    `json:"categoryId" form:"categoryId" binding:"required"`
	Title         string  `json:"title" form:"title" binding:"required,max=200"`
	TitleColor    string  `json:"titleColor" form:"titleColor" binding:"omitempty,max=20"`
	CoverImage    string  `json:"coverImage" form:"coverImage" binding:"omitempty"`
	Summary       string  `json:"summary" form:"summary" binding:"omitempty,max=500"`
	Content       string  `json:"content" form:"content" binding:"omitempty"`
	ContentType   string  `json:"contentType" form:"contentType" binding:"omitempty,oneof=plaintext richtext"`
	Author        string  `json:"author" form:"author" binding:"omitempty,max=50"`
	Source        string  `json:"source" form:"source" binding:"omitempty,max=100"`
	Keywords      string  `json:"keywords" form:"keywords" binding:"omitempty,max=200"`
	Tags          string  `json:"tags" form:"tags" binding:"omitempty,max=200"`
	IsTop         bool    `json:"isTop" form:"isTop"`
	TopSort       int     `json:"topSort" form:"topSort"`
	IsHot         bool    `json:"isHot" form:"isHot"`
	IsRecommend   bool    `json:"isRecommend" form:"isRecommend"`
	AllowComment  bool    `json:"allowComment" form:"allowComment"`
	PublishStatus string  `json:"publishStatus" form:"publishStatus" binding:"omitempty,oneof=draft published scheduled"`
	ScheduledAt   *string `json:"scheduledAt" form:"scheduledAt"`
}

type UpdateContentArticleDTO struct {
	CategoryID    *uint   `json:"categoryId" form:"categoryId"`
	Title         string  `json:"title" form:"title" binding:"omitempty,max=200"`
	TitleColor    string  `json:"titleColor" form:"titleColor" binding:"omitempty,max=20"`
	CoverImage    string  `json:"coverImage" form:"coverImage"`
	Summary       string  `json:"summary" form:"summary" binding:"omitempty,max=500"`
	Content       string  `json:"content" form:"content"`
	ContentType   string  `json:"contentType" form:"contentType" binding:"omitempty,oneof=plaintext richtext"`
	Author        string  `json:"author" form:"author" binding:"omitempty,max=50"`
	Source        string  `json:"source" form:"source" binding:"omitempty,max=100"`
	Keywords      string  `json:"keywords" form:"keywords" binding:"omitempty,max=200"`
	Tags          string  `json:"tags" form:"tags" binding:"omitempty,max=200"`
	IsTop         *bool   `json:"isTop" form:"isTop"`
	TopSort       *int    `json:"topSort" form:"topSort"`
	IsHot         *bool   `json:"isHot" form:"isHot"`
	IsRecommend   *bool   `json:"isRecommend" form:"isRecommend"`
	AllowComment  *bool   `json:"allowComment" form:"allowComment"`
	PublishStatus string  `json:"publishStatus" form:"publishStatus" binding:"omitempty,oneof=draft published scheduled"`
	ScheduledAt   *string `json:"scheduledAt" form:"scheduledAt"`
}

type SetArticleTopDTO struct {
	IsTop   bool `json:"isTop" form:"isTop" binding:"required"`
	TopSort int  `json:"topSort" form:"topSort"`
}

type ContentArticleListQueryDTO struct {
	Current       int    `form:"current"`
	Size          int    `form:"size"`
	CategoryID    uint   `form:"categoryId"`
	Title         string `form:"title"`
	PublishStatus string `form:"publishStatus"`
	IsTop         *bool  `form:"isTop"`
	IsHot         *bool  `form:"isHot"`
	IsRecommend   *bool  `form:"isRecommend"`
	Author        string `form:"author"`
	StartTime     string `form:"startTime"`
	EndTime       string `form:"endTime"`
}
