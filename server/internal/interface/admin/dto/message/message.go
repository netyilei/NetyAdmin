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

// CreateTemplateReq 创建消息模板请求
type CreateTemplateReq struct {
	Code          string `json:"code" binding:"required,max=50"`
	Name          string `json:"name" binding:"required,max=100"`
	Channel       string `json:"channel" binding:"required,oneof=sms email internal push"`
	Title         string `json:"title" binding:"max=200"`
	Content       string `json:"content" binding:"required"`
	ProviderTplID string `json:"providerTplId" binding:"max=100"`
	Status        int    `json:"status" binding:"oneof=0 1"`
}

// UpdateTemplateReq 更新消息模板请求
// 注意：ID 不在 DTO 中，由 URL :id 传入
// 注意：Code 不可变，创建后不允许修改
type UpdateTemplateReq struct {
	Name          string `json:"name" binding:"required,max=100"`
	Channel       string `json:"channel" binding:"required,oneof=sms email internal push"`
	Title         string `json:"title" binding:"max=200"`
	Content       string `json:"content" binding:"required"`
	ProviderTplID string `json:"providerTplId" binding:"max=100"`
	Status        int    `json:"status" binding:"oneof=0 1"`
}
