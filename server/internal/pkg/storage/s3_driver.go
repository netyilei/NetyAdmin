package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Driver struct {
	client     *s3.Client
	uploader   *manager.Uploader
	downloader *manager.Downloader
	bucket     string
	domain     string
	pathPrefix string
}

func NewS3Driver(cfg *Config) (*S3Driver, error) {
	region := GetProviderRegion(cfg.Provider, cfg.Region)

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: cfg.Endpoint}, nil
			},
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.Provider == ProviderMinio ||
			cfg.Provider == ProviderCustom
	})

	return &S3Driver{
		client:     client,
		uploader:   manager.NewUploader(client),
		downloader: manager.NewDownloader(client),
		bucket:     cfg.Bucket,
		domain:     cfg.Domain,
		pathPrefix: cfg.PathPrefix,
	}, nil
}

func (d *S3Driver) buildKey(key string) string {
	if d.pathPrefix != "" && !strings.HasPrefix(key, d.pathPrefix+"/") {
		return d.pathPrefix + "/" + key
	}
	return key
}

func (d *S3Driver) buildURL(key string) string {
	if d.domain != "" {
		return d.domain + "/" + key
	}
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", d.bucket, key)
}

func (d *S3Driver) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (*UploadResult, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	fullKey := d.buildKey(key)

	var body io.Reader = reader
	if size > 0 {
		body = io.LimitReader(reader, size)
	}

	result, err := d.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(d.bucket),
		Key:         aws.String(fullKey),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload object: %w", err)
	}

	return &UploadResult{
		URL:      d.buildURL(fullKey),
		Key:      fullKey,
		ETag:     aws.ToString(result.ETag),
		Size:     size,
		MimeType: contentType,
	}, nil
}

func (d *S3Driver) UploadFile(ctx context.Context, key string, filePath string, contentType string) (*UploadResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	if contentType == "" {
		contentType = detectContentType(filePath)
	}

	return d.Upload(ctx, key, file, stat.Size(), contentType)
}

func (d *S3Driver) Download(ctx context.Context, key string) (io.ReadCloser, *ObjectInfo, error) {
	fullKey := d.buildKey(key)

	buffer := manager.NewWriteAtBuffer([]byte{})

	_, err := d.downloader.Download(ctx, buffer, &s3.GetObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download object: %w", err)
	}

	info, err := d.GetObjectInfo(ctx, key)
	if err != nil {
		info = &ObjectInfo{Key: fullKey}
	}

	return io.NopCloser(bytes.NewReader(buffer.Bytes())), info, nil
}

func (d *S3Driver) Delete(ctx context.Context, key string) error {
	fullKey := d.buildKey(key)

	_, err := d.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

func (d *S3Driver) DeleteMultiple(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	objects := make([]types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objects[i] = types.ObjectIdentifier{
			Key: aws.String(d.buildKey(key)),
		}
	}

	_, err := d.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(d.bucket),
		Delete: &types.Delete{
			Objects: objects,
			Quiet:   aws.Bool(true),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete objects: %w", err)
	}
	return nil
}

func (d *S3Driver) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := d.buildKey(key)

	_, err := d.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		var notFound *types.NotFound
		if isNotFoundError(err, notFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}
	return true, nil
}

func (d *S3Driver) GetObjectInfo(ctx context.Context, key string) (*ObjectInfo, error) {
	fullKey := d.buildKey(key)

	result, err := d.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}

	return &ObjectInfo{
		Key:          fullKey,
		Size:         aws.ToInt64(result.ContentLength),
		LastModified: aws.ToTime(result.LastModified),
		ETag:         aws.ToString(result.ETag),
		MimeType:     aws.ToString(result.ContentType),
	}, nil
}

func (d *S3Driver) GetPresignedUploadURL(ctx context.Context, key string, contentType string, expires time.Duration) (string, error) {
	fullKey := d.buildKey(key)

	if expires == 0 {
		expires = 15 * time.Minute
	}

	presignClient := s3.NewPresignClient(d.client)

	req, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(d.bucket),
		Key:         aws.String(fullKey),
		ContentType: aws.String(contentType),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expires
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return req.URL, nil
}

func (d *S3Driver) GetPresignedDownloadURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	fullKey := d.buildKey(key)

	if expires == 0 {
		expires = 15 * time.Minute
	}

	presignClient := s3.NewPresignClient(d.client)

	req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(fullKey),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expires
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return req.URL, nil
}

func (d *S3Driver) ListObjects(ctx context.Context, prefix string, maxKeys int) ([]*ObjectInfo, error) {
	if maxKeys <= 0 {
		maxKeys = 1000
	}

	fullPrefix := d.buildKey(prefix)

	result, err := d.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(d.bucket),
		Prefix:  aws.String(fullPrefix),
		MaxKeys: aws.Int32(int32(maxKeys)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	objects := make([]*ObjectInfo, 0, len(result.Contents))
	for _, obj := range result.Contents {
		objects = append(objects, &ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
			ETag:         aws.ToString(obj.ETag),
		})
	}

	return objects, nil
}

func (d *S3Driver) Copy(ctx context.Context, srcKey, destKey string) error {
	fullSrcKey := d.buildKey(srcKey)
	fullDestKey := d.buildKey(destKey)

	copySource := fmt.Sprintf("%s/%s", d.bucket, fullSrcKey)

	_, err := d.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(d.bucket),
		Key:        aws.String(fullDestKey),
		CopySource: aws.String(copySource),
	})
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}
	return nil
}

func detectContentType(filePath string) string {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".zip":
		return "application/zip"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".txt":
		return "text/plain"
	case ".html":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	default:
		return "application/octet-stream"
	}
}

func isNotFoundError(err error, notFound *types.NotFound) bool {
	_, ok := err.(*types.NotFound)
	return ok || err.Error() == "NotFound" || err.Error() == "404"
}

type S3DriverFactory struct{}

func (f *S3DriverFactory) Create(config *Config) (Driver, error) {
	return NewS3Driver(config)
}

func NewS3DriverFactory() *S3DriverFactory {
	return &S3DriverFactory{}
}

func DetectMimeType(data []byte) string {
	return http.DetectContentType(data)
}
