package storage

import (
	"testing"
)

// TestBuildPublicURL 表驱动测试 BuildPublicURL 的各种 domain/endpoint 写法（重构清单 B-OTHER-1 回归测试）。
//
// 回退分支按寻址风格与 endpoint 实际协议拼接：
//   - StyleVirtualHost: {scheme}://{bucket}.{host}/{key}
//   - StylePath:        {scheme}://{host}/{bucket}/{key}
func TestBuildPublicURL(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		endpoint string
		bucket   string
		key      string
		style    AddressingStyle
		want     string
	}{
		{
			name:     "domain with https",
			domain:   "https://cdn.example.com",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "a/b.jpg",
			style:    StyleVirtualHost,
			want:     "https://cdn.example.com/a/b.jpg",
		},
		{
			name:     "domain with sub-path stripped",
			domain:   "https://cdn.example.com/sub/path",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "a/b.jpg",
			style:    StyleVirtualHost,
			want:     "https://cdn.example.com/a/b.jpg",
		},
		{
			name:     "domain without protocol defaults to https",
			domain:   "cdn.example.com",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "a/b.jpg",
			style:    StyleVirtualHost,
			want:     "https://cdn.example.com/a/b.jpg",
		},
		{
			name:     "empty domain falls back to virtual-host style",
			domain:   "",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "a/b.jpg",
			style:    StyleVirtualHost,
			want:     "https://bucket.s3.amazonaws.com/a/b.jpg",
		},
		{
			name:     "virtual-host style with http endpoint keeps http scheme",
			domain:   "",
			endpoint: "http://cos.example.com",
			bucket:   "bucket",
			key:      "file.txt",
			style:    StyleVirtualHost,
			want:     "http://bucket.cos.example.com/file.txt",
		},
		{
			name:     "path style with https endpoint",
			domain:   "",
			endpoint: "https://minio.example.com",
			bucket:   "bucket",
			key:      "file.txt",
			style:    StylePath,
			want:     "https://minio.example.com/bucket/file.txt",
		},
		{
			name:     "path style with local http minio endpoint",
			domain:   "",
			endpoint: "http://minio.local:9000",
			bucket:   "mybucket",
			key:      "file.txt",
			style:    StylePath,
			want:     "http://minio.local:9000/mybucket/file.txt",
		},
		{
			name:     "path style endpoint with trailing slash normalized",
			domain:   "",
			endpoint: "http://minio.local:9000/",
			bucket:   "mybucket",
			key:      "file.txt",
			style:    StylePath,
			want:     "http://minio.local:9000/mybucket/file.txt",
		},
		{
			name:     "bare host endpoint (legacy form) falls back to http",
			domain:   "",
			endpoint: "localhost:9000",
			bucket:   "mybucket",
			key:      "file.txt",
			style:    StylePath,
			want:     "http://localhost:9000/mybucket/file.txt",
		},
		{
			name:     "domain with port preserved",
			domain:   "http://cdn.example.com:8080",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "a/b.jpg",
			style:    StyleVirtualHost,
			want:     "http://cdn.example.com:8080/a/b.jpg",
		},
		{
			name:     "key with leading slash trimmed",
			domain:   "https://cdn.example.com",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "/a/b.jpg",
			style:    StyleVirtualHost,
			want:     "https://cdn.example.com/a/b.jpg",
		},
		{
			name:     "domain trailing slash trimmed",
			domain:   "https://cdn.example.com/",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "a/b.jpg",
			style:    StyleVirtualHost,
			want:     "https://cdn.example.com/a/b.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPublicURL(tt.domain, tt.endpoint, tt.bucket, tt.key, tt.style)
			if got != tt.want {
				t.Errorf("BuildPublicURL(%q, %q, %q, %q, %d) = %q, want %q",
					tt.domain, tt.endpoint, tt.bucket, tt.key, tt.style, got, tt.want)
			}
		})
	}
}
