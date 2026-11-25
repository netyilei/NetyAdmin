// port.go：api.UploadService 的模块侧实现（domain → api DTO 转换）。
package storage

import (
	"context"

	"github.com/netyilei/netyadmin/internal/modules/storage/api"
	"github.com/netyilei/netyadmin/internal/modules/storage/application"
)

// uploadPort 实现 api.UploadService（注册名 storage.UploadService）。
type uploadPort struct{ svc *application.Service }

var _ api.UploadService = uploadPort{}

// Credentials 适配 Service.CredentialsFor：只回凭证 DTO（记录/配置实体不出域）。
func (p uploadPort) Credentials(ctx context.Context, req api.CredentialsRequest) (*api.Credentials, error) {
	cred, _, err := p.svc.CredentialsFor(ctx, req.ConfigID, req.FileName, req.MimeType,
		req.BusinessType, req.BusinessID, req.UploaderID, req.UploaderIP)
	if err != nil {
		return nil, err
	}
	return &api.Credentials{
		RecordID:  cred.RecordID,
		UploadURL: cred.UploadURL,
		ObjectKey: cred.ObjectKey,
		Secret:    cred.Secret,
		Headers:   cred.Headers,
		ExpiresAt: cred.ExpiresAt,
	}, nil
}

// Complete 适配 Service.Complete：返回对象 key。
func (p uploadPort) Complete(ctx context.Context, recordID, secret string) (string, error) {
	rec, err := p.svc.Complete(ctx, recordID, secret)
	if err != nil {
		return "", err
	}
	return rec.ObjectKey, nil
}
