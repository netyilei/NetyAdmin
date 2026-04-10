package content

import "time"

type ContentBannerItemDTO struct {
	ID               uint       `json:"id"`
	GroupID          uint       `json:"groupId"`
	GroupName        string     `json:"groupName"`
	Title            string     `json:"title"`
	Subtitle         string     `json:"subtitle"`
	ImageURL         string     `json:"imageUrl"`
	ImageAlt         string     `json:"imageAlt"`
	LinkType         string     `json:"linkType"`
	LinkURL          string     `json:"linkUrl"`
	LinkArticleID    *uint      `json:"linkArticleId"`
	LinkArticleTitle string     `json:"linkArticleTitle,omitempty"`
	Content          string     `json:"content"`
	CustomParams     string     `json:"customParams"`
	Sort             int        `json:"sort"`
	StartTime        *time.Time `json:"startTime"`
	EndTime          *time.Time `json:"endTime"`
	ViewCount        int        `json:"viewCount"`
	ClickCount       int        `json:"clickCount"`
	Status           string     `json:"status"`
	CreatedBy        uint       `json:"createdBy"`
	UpdatedBy        uint       `json:"updatedBy"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type CreateContentBannerItemDTO struct {
	GroupID       uint    `json:"groupId" form:"groupId" binding:"required"`
	Title         string  `json:"title" form:"title" binding:"required,max=200"`
	Subtitle      string  `json:"subtitle" form:"subtitle" binding:"omitempty,max=200"`
	ImageURL      string  `json:"imageUrl" form:"imageUrl" binding:"required"`
	ImageAlt      string  `json:"imageAlt" form:"imageAlt" binding:"omitempty,max=200"`
	LinkType      string  `json:"linkType" form:"linkType" binding:"omitempty,oneof=none internal external article"`
	LinkURL       string  `json:"linkUrl" form:"linkUrl" binding:"omitempty"`
	LinkArticleID *uint   `json:"linkArticleId" form:"linkArticleId"`
	Content       string  `json:"content" form:"content"`
	CustomParams  string  `json:"customParams" form:"customParams"`
	Sort          int     `json:"sort" form:"sort"`
	StartTime     *string `json:"startTime" form:"startTime"`
	EndTime       *string `json:"endTime" form:"endTime"`
	Status        string  `json:"status" form:"status" binding:"omitempty,oneof=0 1"`
}

type ContentBannerItemListQueryDTO struct {
	Current   int    `form:"current"`
	Size      int    `form:"size"`
	GroupID   uint   `form:"groupId"`
	Title     string `form:"title"`
	Status    string `form:"status"`
	StartTime string `form:"startTime"`
	EndTime   string `form:"endTime"`
}

type UpdateContentBannerItemDTO struct {
	Title         string  `json:"title" form:"title" binding:"omitempty,max=200"`
	Subtitle      string  `json:"subtitle" form:"subtitle" binding:"omitempty,max=200"`
	ImageURL      string  `json:"imageUrl" form:"imageUrl"`
	ImageAlt      string  `json:"imageAlt" form:"imageAlt" binding:"omitempty,max=200"`
	LinkType      string  `json:"linkType" form:"linkType" binding:"omitempty,oneof=none internal external article"`
	LinkURL       string  `json:"linkUrl" form:"linkUrl"`
	LinkArticleID *uint   `json:"linkArticleId" form:"linkArticleId"`
	Content       string  `json:"content" form:"content"`
	CustomParams  string  `json:"customParams" form:"customParams"`
	Sort          int     `json:"sort" form:"sort"`
	StartTime     *string `json:"startTime" form:"startTime"`
	EndTime       *string `json:"endTime" form:"endTime"`
	Status        string  `json:"status" form:"status" binding:"omitempty,oneof=0 1"`
}
