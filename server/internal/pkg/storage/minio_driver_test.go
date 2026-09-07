package storage

import (
	"testing"

	"github.com/minio/minio-go/v7"
)

// TestParseEndpoint 表驱动测试 endpoint 统一解析。
//
// 关键契约：含协议前缀必须被剥离（minio-go 只接受裸 host）、
// 裸 host 保持历史 http 语义、尾斜杠与空白被容忍、内嵌路径被拒绝。
func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHost string
		wantTLS  bool
		wantErr  bool
	}{
		{name: "https endpoint", input: "https://cos.ap-shanghai.myqcloud.com", wantHost: "cos.ap-shanghai.myqcloud.com", wantTLS: true},
		{name: "http endpoint with port", input: "http://localhost:9000", wantHost: "localhost:9000"},
		{name: "bare host legacy form defaults to http", input: "localhost:9000", wantHost: "localhost:9000"},
		{name: "trailing slash trimmed", input: "https://s3.amazonaws.com/", wantHost: "s3.amazonaws.com", wantTLS: true},
		{name: "surrounding whitespace trimmed", input: "  https://obs.example.com  ", wantHost: "obs.example.com", wantTLS: true},
		{name: "empty rejected", input: "", wantErr: true},
		{name: "protocol only rejected", input: "https://", wantErr: true},
		{name: "embedded path rejected", input: "https://host.example.com/sub", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, secure, err := ParseEndpoint(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseEndpoint(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if host != tt.wantHost {
				t.Errorf("host = %q, want %q", host, tt.wantHost)
			}
			if secure != tt.wantTLS {
				t.Errorf("secure = %v, want %v", secure, tt.wantTLS)
			}
		})
	}
}

// TestAddressingStyleFor 验证供应商到寻址风格的映射。
//
// 关键回归：腾讯 COS 等非 minio-go 白名单云厂商必须拿到虚拟主机风格，
// 否则请求会走 path-style 而 COS 只支持 virtual-host（404）。
func TestAddressingStyleFor(t *testing.T) {
	tests := []struct {
		provider Provider
		want     AddressingStyle
	}{
		{ProviderMinio, StylePath},
		{ProviderCustom, StylePath},
		{ProviderCloudflare, StylePath},
		{Provider("tencent"), StyleVirtualHost},
		{Provider("aliyun"), StyleVirtualHost},
		{Provider("huawei"), StyleVirtualHost},
		{Provider("qiniu"), StyleVirtualHost},
		{Provider("aws"), StyleVirtualHost},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			if got := AddressingStyleFor(tt.provider); got != tt.want {
				t.Errorf("AddressingStyleFor(%q) = %v, want %v", tt.provider, got, tt.want)
			}
		})
	}
}

// TestAddressingStyleBucketLookup 验证寻址风格到 minio-go BucketLookup 选项的映射，
// 确保不回退到 BucketLookupAuto（Auto 对腾讯 COS 等厂商会错误选择 path-style）。
func TestAddressingStyleBucketLookup(t *testing.T) {
	if got := StylePath.bucketLookup(); got != minio.BucketLookupPath {
		t.Errorf("StylePath.bucketLookup() = %v, want BucketLookupPath", got)
	}
	if got := StyleVirtualHost.bucketLookup(); got != minio.BucketLookupDNS {
		t.Errorf("StyleVirtualHost.bucketLookup() = %v, want BucketLookupDNS", got)
	}
}

// TestNewMinioDriverEndpointRegression 驱动构造回归测试（minio.New 不发起网络请求）。
//
// 核心回归：含协议前缀的 endpoint 此前会触发 minio-go 的
// "Endpoint url cannot have fully qualified paths" 错误导致驱动注册失败，
// 现在必须构造成功且 EndpointURL 正确。
func TestNewMinioDriverEndpointRegression(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		provider Provider
		wantURL  string
		wantErr  bool
	}{
		{
			name:     "https cloud endpoint (COS)",
			endpoint: "https://cos.ap-shanghai.myqcloud.com",
			provider: Provider("tencent"),
			wantURL:  "https://cos.ap-shanghai.myqcloud.com",
		},
		{
			name:     "local minio with protocol",
			endpoint: "http://localhost:9000",
			provider: ProviderMinio,
			wantURL:  "http://localhost:9000",
		},
		{
			name:     "local minio bare host keeps http (legacy)",
			endpoint: "localhost:9000",
			provider: ProviderMinio,
			wantURL:  "http://localhost:9000",
		},
		{
			name:     "endpoint with trailing slash",
			endpoint: "https://s3.amazonaws.com/",
			provider: Provider("aws"),
			wantURL:  "https://s3.amazonaws.com",
		},
		{
			name:     "embedded path rejected",
			endpoint: "https://host.example.com/sub",
			provider: Provider("tencent"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, err := NewMinioDriver(&Config{
				Provider:  tt.provider,
				Endpoint:  tt.endpoint,
				Bucket:    "test-bucket",
				AccessKey: "ak",
				SecretKey: "sk",
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewMinioDriver(%q) error = %v, wantErr %v", tt.endpoint, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			md, ok := driver.(*minioDriver)
			if !ok {
				t.Fatalf("NewMinioDriver 返回类型 %T，want *minioDriver", driver)
			}
			if got := md.client.EndpointURL().String(); got != tt.wantURL {
				t.Errorf("EndpointURL = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

// TestMinioDriverBuildURL 验证驱动 URL 构造与请求寻址风格同源：
// path-style 供应商回退 URL 为 {scheme}://{host}/{bucket}/{key}，
// virtual-host 供应商为 {scheme}://{bucket}.{host}/{key}。
func TestMinioDriverBuildURL(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		provider Provider
		domain   string
		want     string
	}{
		{
			name:     "cos virtual-host fallback",
			endpoint: "https://cos.ap-shanghai.myqcloud.com",
			provider: Provider("tencent"),
			want:     "https://test-bucket.cos.ap-shanghai.myqcloud.com/2026/a.jpg",
		},
		{
			name:     "minio path-style fallback",
			endpoint: "http://localhost:9000",
			provider: ProviderMinio,
			want:     "http://localhost:9000/test-bucket/2026/a.jpg",
		},
		{
			name:     "domain takes precedence over endpoint fallback",
			endpoint: "https://cos.ap-shanghai.myqcloud.com",
			provider: Provider("tencent"),
			domain:   "https://cdn.example.com",
			want:     "https://cdn.example.com/2026/a.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Provider:  tt.provider,
				Endpoint:  tt.endpoint,
				Bucket:    "test-bucket",
				AccessKey: "ak",
				SecretKey: "sk",
				Domain:    tt.domain,
			}
			driver, err := NewMinioDriver(cfg)
			if err != nil {
				t.Fatalf("NewMinioDriver(%q) error = %v", tt.endpoint, err)
			}
			md := driver.(*minioDriver)
			if got := md.buildURL("2026/a.jpg"); got != tt.want {
				t.Errorf("buildURL = %q, want %q", got, tt.want)
			}
		})
	}
}
