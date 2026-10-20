package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestOpenPlatformAuth_RejectsOversizedBody 验证验签前的请求体上限：
// 超过 maxSignedRequestBodyBytes 的请求在读取阶段即被拒绝（413），
// 不会进入签名校验或占用无界内存。
func TestOpenPlatformAuth_RejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := strings.NewReader(strings.Repeat("a", maxSignedRequestBodyBytes+1))
	req := httptest.NewRequest(http.MethodPost, "/client/v1/echo", body)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	// 超限在读取阶段返回，早于任何 service 调用，故依赖可传 nil。
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
