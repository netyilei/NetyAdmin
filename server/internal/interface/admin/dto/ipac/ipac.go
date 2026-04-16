package ipac

type IPACQuery struct {
	AppID    *string `form:"appId"`
	IPAddr   string  `form:"ipAddr"`
	Type     int     `form:"type"`
	Status   *int    `form:"status"`
	Current  int     `form:"current"`
	Size     int     `form:"size"`
}

type CreateIPACReq struct {
	AppID     *string `json:"appId"`
	IPAddr    string  `json:"ipAddr" binding:"required"`
	Type      int     `json:"type" binding:"required,oneof=1 2"`
	Reason    string  `json:"reason"`
	ExpiredAt *string `json:"expiredAt"` // 格式: "2006-01-02 15:04:05"
	Status    int     `json:"status" binding:"oneof=0 1"`
}

type UpdateIPACReq struct {
	ID        uint    `json:"id" binding:"required"`
	Type      int     `json:"type" binding:"required,oneof=1 2"`
	Reason    string  `json:"reason"`
	ExpiredAt *string `json:"expiredAt"`
	Status    int     `json:"status" binding:"oneof=0 1"`
}

type BatchDeleteIPACReq struct {
	IDs []uint `json:"ids" binding:"required,min=1"`
}
