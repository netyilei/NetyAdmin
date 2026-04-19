package middleware

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	openEntity "NetyAdmin/internal/domain/entity/open_platform"
	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/response"
	ipacSvcPkg "NetyAdmin/internal/service/ipac"
	openSvcPkg "NetyAdmin/internal/service/open_platform"
)

type openPlatformResponseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *openPlatformResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// OpenPlatformAuth 开放平台签名验证中间件
func OpenPlatformAuth(appSvc openSvcPkg.AppService, apiSvc openSvcPkg.OpenApiService, logSvc openSvcPkg.OpenLogService, ipacSvc ipacSvcPkg.IPACService) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		appKey := c.GetHeader("X-App-Key")
		timestampStr := c.GetHeader("X-Timestamp")
		nonce := c.GetHeader("X-Nonce")
		signature := c.GetHeader("X-Signature")

		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		writer := &openPlatformResponseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = writer

		defer func() {
			// 记录日志
			latency := time.Since(startTime).Nanoseconds()
			statusCode := c.Writer.Status()

			appID, _ := c.Get("appID")
			appIDStr := ""
			if appID != nil {
				appIDStr = appID.(string)
			}

			headerBytes, _ := json.Marshal(c.Request.Header)

			log := &openEntity.OpenPlatformLog{
				AppID:         appIDStr,
				AppKey:        appKey,
				ApiPath:       c.Request.URL.Path,
				ApiMethod:     c.Request.Method,
				ClientIP:      c.ClientIP(),
				StatusCode:    statusCode,
				Latency:       latency,
				RequestHeader: string(headerBytes),
				RequestBody:   string(requestBody),
				ResponseBody:  writer.body.String(),
				CreatedAt:     startTime,
			}

			// 异步记录
			go logSvc.Record(context.Background(), log)
		}()

		if appKey == "" || timestampStr == "" || nonce == "" || signature == "" {
			response.FailWithCode(c, errorx.CodeInvalidParams, "缺少签名参数")
			c.Abort()
			return
		}

		// 1. 验证时钟容差 (±60s)
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			response.FailWithCode(c, errorx.CodeInvalidParams, "时间戳格式错误")
			c.Abort()
			return
		}

		now := time.Now().Unix()
		if timestamp < now-60 || timestamp > now+60 {
			response.FailWithCode(c, errorx.CodeRequestExpired, "请求已过期")
			c.Abort()
			return
		}

		// 2. 获取 App 信息
		app, err := appSvc.GetAppByKey(c.Request.Context(), appKey)
		if err != nil {
			response.FailWithCode(c, errorx.CodeAppKeyInvalid, "AppKey 无效")
			c.Abort()
			return
		}

		// 3. 检查应用状态
		if app.Status == openEntity.AppStatusDisabled {
			response.FailWithCode(c, errorx.CodeAppKeyInvalid, "应用已被禁用")
			c.Abort()
			return
		}

		// 3. IP 访问控制 (IPAC)
		clientIP := c.ClientIP()
		allowed, err := ipacSvc.CheckIP(c.Request.Context(), clientIP, &app.ID)
		if err != nil {
			// 如果匹配过程出错，为了安全，记录日志并放行 (或者为了更严谨可以拦截，这里遵循 IPACAuth 逻辑)
		} else if !allowed {
			response.FailWithCode(c, errorx.CodeIPBlocked, "您的 IP 访问受限")
			c.Abort()
			return
		}

		// 4. Nonce 防重放 (使用缓存模块)
		nonceKey := cache.KeyAppNonce(appKey, nonce)
		set, err := appSvc.GetCacheMgr().SetNX(c.Request.Context(), nonceKey, "1", 60*time.Second)
		if err != nil || !set {
			response.FailWithCode(c, errorx.CodeSignatureFailed, "重复的请求 (Nonce)")
			c.Abort()
			return
		}

		// 5. 解密 AppSecret
		appSecret, err := appSvc.GetAppSecret(c.Request.Context(), app)
		if err != nil {
			response.FailWithCode(c, errorx.CodeInternalError, "系统错误")
			c.Abort()
			return
		}

		// 6. 构造待签名字符串 (StringToSign)
		stringToSign := constructStringToSign(c, timestampStr, nonce)

		// 7. 计算 HMAC-SHA256 签名
		expectedSignature := computeHmacSha256(appSecret, stringToSign)

		if signature != expectedSignature {
			response.FailWithCode(c, errorx.CodeSignatureFailed, "签名验证失败")
			c.Abort()
			return
		}

		// 8. 流量限制 (Rate Limiting)
		allowed, err = appSvc.AllowRequest(c.Request.Context(), app)
		if err != nil || !allowed {
			response.FailWithCode(c, errorx.CodeRateLimited, "已触发流量限制")
			c.Abort()
			return
		}

		// 9. 验证 API 权限
		allowedApis, err := apiSvc.GetAppAllowedApis(c.Request.Context(), app.ID)
		if err == nil && len(allowedApis) > 0 {
			matchedPath := c.FullPath()
			if matchedPath == "" {
				matchedPath = c.Request.URL.Path
			}
			currentApi := strings.ToUpper(c.Request.Method) + ":" + matchedPath
			matched := false
			for _, api := range allowedApis {
				if api == currentApi {
					matched = true
					break
				}
			}
			if !matched {
				response.FailWithCode(c, errorx.CodeScopeMismatch, "权限不足")
				c.Abort()
				return
			}
		}

		// 将 appID 存入上下文供后续使用
		c.Set("appID", app.ID)
		c.Next()
	}
}

func constructStringToSign(c *gin.Context, timestamp, nonce string) string {
	method := strings.ToUpper(c.Request.Method)
	path := c.Request.URL.Path

	var payload string
	if method == http.MethodGet {
		// GET 请求：对 Query 参数按 key 排序并拼接
		query := c.Request.URL.Query()
		keys := make([]string, 0, len(query))
		for k := range query {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var sb strings.Builder
		for i, k := range keys {
			if i > 0 {
				sb.WriteString("&")
			}
			sb.WriteString(k)
			sb.WriteString("=")
			sb.WriteString(query.Get(k))
		}
		payload = sb.String()
	} else {
		// POST/PUT/DELETE：计算 Body 的 SHA256 哈希
		var body []byte
		if c.Request.Body != nil {
			body, _ = io.ReadAll(c.Request.Body)
			// 将 Body 重新写回，以便后续 Handler 使用
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		}
		if len(body) > 0 {
			h := sha256.New()
			h.Write(body)
			payload = fmt.Sprintf("%x", h.Sum(nil))
		}
	}

	// 构造规则: Method + Path + Timestamp + Nonce + Payload
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", method, path, timestamp, nonce, payload)
}

func computeHmacSha256(secret, data string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
