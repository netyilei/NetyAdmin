package v1

type InternalMsgListReq struct {
	Page       int `form:"page" binding:"omitempty,min=1"`
	PageSize   int `form:"pageSize" binding:"omitempty,min=1,max=100"`
	ReadFilter *int `form:"readFilter" binding:"omitempty,oneof=0 1"`
}

type MarkReadReq struct {
	MsgInternalID uint64 `json:"msgInternalId" binding:"required"`
}

type MarkAllReadReq struct{}
