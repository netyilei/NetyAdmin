package message

type MsgTemplateQuery struct {
	Current int    `form:"current"`
	Size    int    `form:"size"`
	Channel string `form:"channel"`
	Code    string `form:"code"`
	Name    string `form:"name"`
	Status  *int   `form:"status"`
}

type MsgRecordQuery struct {
	Current  int    `form:"current"`
	Size     int    `form:"size"`
	Channel  string `form:"channel"`
	Receiver string `form:"receiver"`
	Status   *int   `form:"status"`
}

type SendDirectReq struct {
	Channel  string `json:"channel" binding:"required"`
	Receiver string `json:"receiver" binding:"required"`
	Title    string `json:"title"`
	Content  string `json:"content" binding:"required"`
}
