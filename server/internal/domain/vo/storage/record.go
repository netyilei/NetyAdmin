package storage

import "time"

type RecordVO struct {
	ID              uint      `json:"id"`
	StorageConfigID uint      `json:"storageConfigId"`
	StorageName     string    `json:"storageName"`
	FileName        string    `json:"fileName"`
	StoredName      string    `json:"storedName"`
	FilePath        string    `json:"filePath"`
	FileURL         string    `json:"fileUrl"`
	FileSize        int64     `json:"fileSize"`
	MimeType        string    `json:"mimeType"`
	FileExt         string    `json:"fileExt"`
	MD5             string    `json:"md5"`
	Source          string    `json:"source"`
	SourceID        uint      `json:"sourceId"`
	SourceInfo      string    `json:"sourceInfo"`
	UploaderIP      string    `json:"uploaderIp"`
	BusinessType    string    `json:"businessType"`
	BusinessID      uint      `json:"businessId"`
	UploadedAt      time.Time `json:"uploadedAt"`
	CreatedAt       time.Time `json:"createdAt"`
}
