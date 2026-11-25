// Package api 是 storage 模块的跨模块端口（下游模块 import 本包即可消费
// 对象存储直传能力；禁止 import application/domain）。
package api

import (
	"context"
	"time"
)

// UploadService 是对象存储直传端口（注册名 `storage.UploadService`）：
//
//	up, err := module.ResolveAs[storageapi.UploadService](registry.Services, "storage.UploadService")
//	cred, err := up.Credentials(ctx, storageapi.CredentialsRequest{FileName: "a.png", MimeType: "image/png"})
//	// 客户端按 cred.UploadURL 直传，完成后：
//	key, err := up.Complete(ctx, cred.RecordID, cred.Secret)
type UploadService interface {
	// Credentials 签发直传凭证（presigned PUT + 完成校验 secret）。
	Credentials(ctx context.Context, req CredentialsRequest) (*Credentials, error)
	// Complete 确认上传完成（HMAC 校验）；返回对象 key（业务侧持久化该 key）。
	Complete(ctx context.Context, recordID, secret string) (objectKey string, err error)
}

// CredentialsRequest 是凭证申请参数（ConfigID 空=使用默认存储配置）。
type CredentialsRequest struct {
	ConfigID     string
	FileName     string
	MimeType     string
	BusinessType string // 业务分类（用于记录归类/清理策略）
	BusinessID   string
	UploaderID   string // 上传者身份（审计面）
	UploaderIP   string
}

// Credentials 是直传凭证（客户端按 UploadURL+Headers 发起 PUT）。
type Credentials struct {
	RecordID  string            // 直传记录 ID（Complete 时回传）
	UploadURL string            // presigned PUT URL
	ObjectKey string            // 对象 key（Complete 返回一致值）
	Secret    string            // 完成校验 secret（HMAC，防伪造 complete）
	Headers   map[string]string // PUT 需要携带的头
	ExpiresAt time.Time         // 凭证过期时间
}
