package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestOpenPlatformAuth_RejectsOversizedBody 验证全局 BodyLimit + 验签中间件的组合：
// 超过 DefaultMaxRequestBodyBytes 的请求在读取阶段即被拒绝（413），
// 不会进入签名校验或占用无界内存。
func TestOpenPlatformAuth_RejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := strings.NewReader(strings.Repeat("a", int(DefaultMaxRequestBodyBytes)+1))
	req := httptest.NewRequest(http.MethodPost, "/client/v1/echo", body)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	// 与生产装配一致：全局 BodyLimit 先于 OpenPlatformAuth 执行。
	// 超限在读取阶段返回，早于任何 service 调用，故依赖可传 nil。
	BodyLimit(DefaultMaxRequestBodyBytes)(c)
	OpenPlatformAuth(nil, nil, nil, nil)(c)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Contains(t, rec.Body.String(), "请求体过大")
}

// TestConstructStringToSign_QueryParams 验证 GET 待签名串：
// key 按字典序；同名参数的每个值按出现顺序全部参与（防止参数走私）。
func TestConstructStringToSign_QueryParams(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		payload string
	}{
		{
			name:    "single values sorted by key",
			query:   "pageSize=10&categoryId=1&page=1",
			payload: "categoryId=1&page=1&pageSize=10",
		},
		{
			name:    "duplicate keys include all values in order",
			query:   "a=1&a=3&b=2",
			payload: "a=1&a=3&b=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/client/v1/content/articles?"+tt.query, nil)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = req

			want := "GET\n/client/v1/content/articles\n1713888000\nnonce\n" + tt.payload
			assert.Equal(t, want, constructStringToSign(c, "1713888000", "nonce", nil))
		})
	}
}

// TestSanitizeBody_TruncatesOversizedBodies 验证超过日志上限的请求体只保留大小摘要，
// 不进入 JSON 解析/递归脱敏（大 Body 性能保护）。
func TestSanitizeBody_TruncatesOversizedBodies(t *testing.T) {
	body := strings.Repeat("a", maxLoggedBodyBytes+1)

	got := sanitizeBody(body)

	assert.Contains(t, got, `"_truncated":true`)
	assert.Contains(t, got, fmt.Sprintf(`"size":%d`, len(body)))
}

// TestSanitizeBody_SanitizesSmallJSON 验证上限内的 JSON 仍按敏感字段脱敏。
func TestSanitizeBody_SanitizesSmallJSON(t *testing.T) {
	got := sanitizeBody(`{"password":"secret","name":"ok"}`)

	assert.Contains(t, got, "***REDACTED***")
	assert.NotContains(t, got, "secret")
	assert.Contains(t, got, `"name":"ok"`)
}

// TestResponseWriter_TruncatesBufferedBody 验证响应体只保留前 maxLoggedBodyBytes 字节，
// total 仍记录完整长度，超限时日志使用大小摘要。
func TestResponseWriter_TruncatesBufferedBody(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	w := &openPlatformResponseWriter{ResponseWriter: c.Writer}

	payload := []byte(strings.Repeat("x", maxLoggedBodyBytes+100))
	n, err := w.Write(payload)
	assert.NoError(t, err)
	assert.Equal(t, len(payload), n)

	assert.Equal(t, maxLoggedBodyBytes, w.body.Len(), "缓冲只保留上限内的字节")
	assert.Equal(t, len(payload), w.total, "total 记录完整响应长度")
	assert.Contains(t, sanitizeResponseBody(w), `"_truncated":true`)
}
