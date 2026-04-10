package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"netyadmin/internal/pkg/errorx"
	"netyadmin/internal/pkg/response"
)

func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		done := make(chan struct{})
		go func() {
			defer close(done)
			c.Next()
		}()

		select {
		case <-done:
			return
		case <-ctx.Done():
			response.FailWithStatus(c, http.StatusRequestTimeout, errorx.CodeTooManyRequest, "请求超时")
			c.Abort()
		}
	}
}

func TimeoutWithHandler(timeout time.Duration, timeoutHandler func(*gin.Context)) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		done := make(chan struct{})
		go func() {
			defer close(done)
			c.Next()
		}()

		select {
		case <-done:
			return
		case <-ctx.Done():
			if timeoutHandler != nil {
				timeoutHandler(c)
			} else {
				response.FailWithStatus(c, http.StatusRequestTimeout, errorx.CodeTooManyRequest, "请求超时")
				c.Abort()
			}
		}
	}
}
