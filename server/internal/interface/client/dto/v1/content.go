package v1

import "time"

type ClientArticleListReq struct {
	Page       int    `form:"page" json:"page"`
	PageSize   int    `form:"pageSize" json:"pageSize"`
	CategoryID uint   `form:"categoryId" json:"categoryId" binding:"required"`
	Keyword    string `form:"keyword" json:"keyword"`
}

type ClientArticleItemVO struct {
	ID           uint       `json:"id"`
	CategoryID   uint       `json:"categoryId"`
	CategoryName string     `json:"categoryName"`
	Title        string     `json:"title"`
	TitleColor   string     `json:"titleColor"`
	CoverImage   string     `json:"coverImage"`
	Summary      string     `json:"summary"`
	ContentType  string     `json:"contentType"`
	Author       string     `json:"author"`
	Source       string     `json:"source"`
	IsTop        bool       `json:"isTop"`
	IsHot        bool       `json:"isHot"`
	IsRecommend  bool       `json:"isRecommend"`
	ViewCount    int        `json:"viewCount"`
	LikeCount    int        `json:"likeCount"`
	CommentCount int        `json:"commentCount"`
	PublishedAt  *time.Time `json:"publishedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type ClientArticleDetailVO struct {
	ID           uint       `json:"id"`
	CategoryID   uint       `json:"categoryId"`
	CategoryName string     `json:"categoryName"`
	Title        string     `json:"title"`
	TitleColor   string     `json:"titleColor"`
	CoverImage   string     `json:"coverImage"`
	Summary      string     `json:"summary"`
	Content      string     `json:"content"`
	ContentType  string     `json:"contentType"`
	Author       string     `json:"author"`
	Source       string     `json:"source"`
	Keywords     string     `json:"keywords"`
	Tags         string     `json:"tags"`
	IsTop        bool       `json:"isTop"`
	IsHot        bool       `json:"isHot"`
	IsRecommend  bool       `json:"isRecommend"`
	AllowComment bool       `json:"allowComment"`
	ViewCount    int        `json:"viewCount"`
	LikeCount    int        `json:"likeCount"`
	CommentCount int        `json:"commentCount"`
	PublishedAt  *time.Time `json:"publishedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type ClientBannerGroupVO struct {
	ID          uint                 `json:"id"`
	Name        string               `json:"name"`
	Code        string               `json:"code"`
	Description string               `json:"description"`
	Position    string               `json:"position"`
	Width       int                  `json:"width"`
	Height      int                  `json:"height"`
	AutoPlay    bool                 `json:"autoPlay"`
	Interval    int                  `json:"interval"`
	Banners     []ClientBannerItemVO `json:"banners"`
}

type ClientBannerItemVO struct {
	ID           uint   `json:"id"`
	Title        string `json:"title"`
	Subtitle     string `json:"subtitle"`
	ImageURL     string `json:"imageUrl"`
	ImageAlt     string `json:"imageAlt"`
	LinkType     string `json:"linkType"`
	LinkURL      string `json:"linkUrl"`
	Content      string `json:"content"`
	CustomParams string `json:"customParams"`
	Sort         int    `json:"sort"`
}
