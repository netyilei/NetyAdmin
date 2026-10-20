package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newTestContext(method, target string, body io.Reader) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(method, target, body)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	return c, rec
}

// TestBodyLimit_EnforcesMax 验证超过上限的读取返回 *http.MaxBytesError。
func TestBodyLimit_EnforcesMax(t *testing.T) {
	c, _ := newTestContext(http.MethodPost, "/", strings.NewReader("123456789"))

	BodyLimit(8)(c)

	_, err := io.ReadAll(c.Request.Body)
	var maxBytesErr *http.MaxBytesError
	assert.True(t, errors.As(err, &maxBytesErr), "expected *http.MaxBytesError, got %v", err)
}

// TestBodyLimit_AllowsWithinLimit 验证上限内的请求不受影响。
func TestBodyLimit_AllowsWithinLimit(t *testing.T) {
	c, _ := newTestContext(http.MethodPost, "/", strings.NewReader("12345678"))

	BodyLimit(8)(c)

	data, err := io.ReadAll(c.Request.Body)
	assert.NoError(t, err)
	assert.Equal(t, "12345678", string(data))
}

// TestBodyLimit_Disabled 验证 maxBytes <= 0 时不启用限制（零值约定）。
func TestBodyLimit_Disabled(t *testing.T) {
	c, _ := newTestContext(http.MethodPost, "/", strings.NewReader("123456789"))

	BodyLimit(0)(c)

	data, err := io.ReadAll(c.Request.Body)
	assert.NoError(t, err)
	assert.Equal(t, "123456789", string(data))
}
