// clientapi.go：storage client 端点（——credentials/complete；authn=
// users JWT 中间件承载，principal 即 uploader——cmd 接线）。
package adapters

import (
	"github.com/gin-gonic/gin"

	// blank import 供 swag 解析 @Failure 注解引用（errorx.Envelope）
	_ "github.com/netyilei/netyadmin/internal/kernel/errorx"

	"github.com/netyilei/netyadmin/internal/kernel/transport"
	"github.com/netyilei/netyadmin/internal/modules/storage/application"
)

// RegisterClientRoutes 注册 client 上传端点（module.go 调用——authed 组挂载）。
func RegisterClientRoutes(authed gin.IRouter, svc *application.Service) {
	st := authed.Group("/storage")
	st.POST("/credentials", credentialsHandler(svc))
	st.POST("/complete", completeHandler(svc))
}

// credentialsRequest 凭证请求（config 空=默认配置）。
type credentialsRequest struct {
	ConfigID     string `json:"config_id"`
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	BusinessType string `json:"business_type"`
	BusinessID   string `json:"business_id"`
}

// @Summary 签发直传凭证（presigned PUT + HMAC secret）
// @Tags storage-client
// @ID storage.client.credentials
// @Accept json
// @Param body body credentialsRequest true "上传请求"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 503 {object} errorx.Envelope
// @Router /client/v1/storage/credentials [post]
func credentialsHandler(svc *application.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req credentialsRequest
		if !transport.Bind(c, &req) {
			return
		}
		p := transport.PrincipalFrom(c)
		creds, _, err := svc.CredentialsFor(c.Request.Context(), req.ConfigID,
			req.FileName, req.MimeType, req.BusinessType, req.BusinessID, p.ID, c.ClientIP())
		if err != nil {
			transport.FailOf(c, mapStorageError(err)) // 裸哨兵禁直传 FailOf（10009 归一掩盖语义）
			return
		}
		transport.OK(c, creds)
	}
}

// completeRequest 完成回调（客户端直传后携带凭证 secret）。
type completeRequest struct {
	RecordID string `json:"record_id" validate:"required"`
	Secret   string `json:"secret" validate:"required"`
}

// @Summary 完成上传（HMAC 校验 + CAS 登记）
// @Tags storage-client
// @ID storage.client.complete
// @Accept json
// @Param body body completeRequest true "完成回调"
// @Success 200 {object} errorx.Envelope
// @Failure 400 {object} errorx.Envelope
// @Failure 401 {object} errorx.Envelope
// @Failure 403 {object} errorx.Envelope
// @Failure 409 {object} errorx.Envelope
// @Router /client/v1/storage/complete [post]
func completeHandler(svc *application.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req completeRequest
		if !transport.Bind(c, &req) {
			return
		}
		rec, err := svc.Complete(c.Request.Context(), req.RecordID, req.Secret)
		if err != nil {
			transport.FailOf(c, mapStorageError(err)) // 篡改=403/重放=409/不存在=404——语义码
			return
		}
		transport.OK(c, gin.H{"id": rec.ID, "file_url": rec.FileURL, "status": rec.Status})
	}
}
