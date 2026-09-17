package ipac

type IPACQuery struct {
	AppID   *string `form:"appId"`
	IPAddr  string  `form:"ipAddr"`
	Type    int     `form:"type"`
	Status  *int    `form:"status"`
	Current int     `form:"current"`
	Size    int     `form:"size"`
}

type CreateIPACReq struct {
	AppID  *string `json:"appId"`
	IPAddr string  `json:"ipAddr" binding:"required"` // 单 IP（192.168.1.1）或 CIDR（10.0.0.0/8）
	Type   int     `json:"type" binding:"required,oneof=1 2"`
	Reason string  `json:"reason"`
	// 过期时间：推荐 RFC3339 带时区（如 2026-09-18T00:00:00+08:00，前端提交 ISO 字符串）；
	// 兼容 "2006-01-02 15:04:05" 裸格式，按服务器本地时区解释
	ExpiredAt *string `json:"expiredAt"`
	Status    int     `json:"status" binding:"oneof=0 1"`
}

type UpdateIPACReq struct {
	ID     uint   `json:"id" binding:"required"`
	Type   int    `json:"type" binding:"required,oneof=1 2"`
	Reason string `json:"reason"`
	// 过期时间：推荐 RFC3339 带时区；兼容 "2006-01-02 15:04:05"（按服务器本地时区解释）
	ExpiredAt *string `json:"expiredAt"`
	Status    int     `json:"status" binding:"oneof=0 1"`
}

type BatchDeleteIPACReq struct {
	IDs []uint `json:"ids" binding:"required,min=1"`
}
