package storage

import (
	"testing"
)

// TestBuildPublicURL 表驱动测试 BuildPublicURL 的各种 domain/endpoint 写法（重构清单 B-OTHER-1 回归测试）。
func TestBuildPublicURL(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		endpoint string
		bucket   string
		key      string
		want     string
	}{
		{
			name:     "domain with https",
			domain:   "https://cdn.example.com",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "a/b.jpg",
			want:     "https://cdn.example.com/a/b.jpg",
		},
		{
			name:     "domain with sub-path stripped",
			domain:   "https://cdn.example.com/sub/path",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "a/b.jpg",
			want:     "https://cdn.example.com/a/b.jpg",
		},
		{
			name:     "domain without protocol defaults to https",
			domain:   "cdn.example.com",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "a/b.jpg",
			want:     "https://cdn.example.com/a/b.jpg",
		},
		{
			name:     "empty domain falls back to virtual-host style",
			domain:   "",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "a/b.jpg",
			want:     "https://bucket.s3.amazonaws.com/a/b.jpg",
		},
		{
			name:     "domain with port preserved",
			domain:   "http://cdn.example.com:8080",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "a/b.jpg",
			want:     "http://cdn.example.com:8080/a/b.jpg",
		},
		{
			name:     "key with leading slash trimmed",
			domain:   "https://cdn.example.com",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "/a/b.jpg",
			want:     "https://cdn.example.com/a/b.jpg",
		},
		{
			name:     "domain trailing slash trimmed",
			domain:   "https://cdn.example.com/",
			endpoint: "https://s3.amazonaws.com",
			bucket:   "bucket",
			key:      "a/b.jpg",
			want:     "https://cdn.example.com/a/b.jpg",
		},
		{
			name:     "endpoint with http protocol stripped",
			domain:   "",
			endpoint: "http://minio.local:9000",
			bucket:   "mybucket",
			key:      "file.txt",
			want:     "https://mybucket.minio.local:9000/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPublicURL(tt.domain, tt.endpoint, tt.bucket, tt.key)
			if got != tt.want {
				t.Errorf("BuildPublicURL(%q, %q, %q, %q) = %q, want %q",
					tt.domain, tt.endpoint, tt.bucket, tt.key, got, tt.want)
			}
		})
	}
}
