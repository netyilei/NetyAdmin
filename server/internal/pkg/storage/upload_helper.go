package storage

import (
	"context"

	storageVO "NetyAdmin/internal/domain/vo/storage"
)

// CompleteUploadParams 是 CompleteUpload 的统一参数载体。
//
// 设计原则（重构清单 B-OTHER-2）：
// 三端（admin/client/user）的 CompleteUpload handler 逻辑 100% 同构
// （bind → 调 recordService.CompleteUpload → 透传），仅 DTO 类型名不同。
// 由于 RULES.md §9 禁止 DTO 跨端共享，三端各自 bind 自己的 DTO 后转换为本结构，
// 再调用 CompleteUploadFromParams，消除 handler 中的样板复制。
type CompleteUploadParams struct {
	RecordID  uint
	Secret    string
	ObjectKey string
	FileURL   string
	FileSize  int64
	MimeType  string
	MD5       string
}

// RecordCompleter 是 RecordService.CompleteUpload 的镜像接口，
// 供 upload helper 调用而不反向依赖 service 层。
type RecordCompleter interface {
	CompleteUpload(
		ctx context.Context,
		recordID uint,
		secret string,
		objectKey string,
		fileURL string,
		fileSize int64,
		mimeType string,
		md5 string,
	) (*storageVO.RecordVO, error)
}

// CompleteUploadFromParams 执行 CompleteUpload 的核心逻辑。
//
// 三端 handler 各自 bind DTO 后转换为 CompleteUploadParams，调用本函数，
// 避免在三个 handler 中复制粘贴同一份 7 参 service 调用样板。
func CompleteUploadFromParams(
	ctx context.Context,
	svc RecordCompleter,
	p CompleteUploadParams,
) (*storageVO.RecordVO, error) {
	return svc.CompleteUpload(
		ctx,
		p.RecordID,
		p.Secret,
		p.ObjectKey,
		p.FileURL,
		p.FileSize,
		p.MimeType,
		p.MD5,
	)
}
