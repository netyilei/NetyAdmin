package v1

type EchoRequest struct {
	Message string `json:"message" binding:"required"`
}

type EchoResponse struct {
	Message   string `json:"message"`
	AppID     string `json:"appId"`
	Timestamp int64  `json:"timestamp"`
}
