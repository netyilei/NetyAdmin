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
	"log/slog"
	"net/http"
	"regexp"
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
				RequestHeader: sanitizeHeaderValue(string(headerBytes)),
				RequestBody:   sanitizeBody(string(requestBody)),
				ResponseBody:  sanitizeBody(writer.body.String()),
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
			response.FailWithCode(c, errorx.CodeAppDisabled, "应用已被禁用")
			c.Abort()
			return
		}

		// 4. IP 访问控制 (IPAC) - fail-closed：校验异常时拒绝请求，避免安全策略被绕过
		clientIP := c.ClientIP()
		allowed, err := ipacSvc.CheckIP(c.Request.Context(), clientIP, &app.ID)
		if err != nil {
			slog.Error("[OpenPlatformAuth] IPAC check error, deny request", "ip", clientIP, "appID", app.ID, "err", err)
			response.FailWithCode(c, errorx.CodeIPBlocked, "访问校验服务异常，请稍后再试")
			c.Abort()
			return
		} else if !allowed {
			response.FailWithCode(c, errorx.CodeIPBlocked, "您的 IP 访问受限")
			c.Abort()
			return
		}

		// 5. Nonce 防重放 (使用缓存模块)
		// nonce TTL 调整为 2 分钟，覆盖整个时间戳容差窗口（±60s 共 120s），防止窗口尾端的 nonce 过期后被重放
		nonceKey := cache.KeyAppNonce(appKey, nonce)
		set, err := appSvc.GetCacheMgr().SetNX(c.Request.Context(), nonceKey, "1", 2*time.Minute)
		if err != nil || !set {
			response.FailWithCode(c, errorx.CodeSignatureFailed, "重复的请求 (Nonce)")
			c.Abort()
			return
		}

		// 6. 解密 AppSecret
		appSecret, err := appSvc.GetAppSecret(c.Request.Context(), app)
		if err != nil {
			response.FailWithCode(c, errorx.CodeInternalError, "系统错误")
			c.Abort()
			return
		}

		// 7. 构造待签名字符串 (StringToSign)
		stringToSign := constructStringToSign(c, timestampStr, nonce, requestBody)

		// 8. 计算 HMAC-SHA256 签名
		expectedSignature := computeHmacSha256(appSecret, stringToSign)

		// 使用 hmac.Equal 进行恒定时间比较，防止时序攻击推导签名
		// 注意：直接比较字符串（!=）会因为短路比较泄露字节差异信息
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			response.FailWithCode(c, errorx.CodeSignatureFailed, "签名验证失败")
			c.Abort()
			return
		}

		// 9. 流量限制 (Rate Limiting)
		allowed, err = appSvc.AllowRequest(c.Request.Context(), app)
		if err != nil || !allowed {
			response.FailWithCode(c, errorx.CodeRateLimited, "已触发流量限制")
			c.Abort()
			return
		}

		// 10. 验证 API 权限 (Scope Check)
		allowedApis, err := apiSvc.GetAppAllowedApis(c.Request.Context(), app.ID)
		if err != nil {
			response.FailWithCode(c, errorx.CodeInternalError, "鉴权服务异常")
			c.Abort()
			return
		}

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
			response.FailWithCode(c, errorx.CodeScopeMismatch, "权限不足 (Scope Mismatch)")
			c.Abort()
			return
		}

		// 将 appID 存入上下文供后续使用
		c.Set("appID", app.ID)
		c.Set("currentOpenApp", app)
		c.Next()
	}
}

func constructStringToSign(c *gin.Context, timestamp, nonce string, requestBody []byte) string {
	method := strings.ToUpper(c.Request.Method)
	path := c.Request.URL.Path

	var payload string
	if method == http.MethodGet {
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
		if len(requestBody) > 0 {
			h := sha256.New()
			h.Write(requestBody)
			payload = fmt.Sprintf("%x", h.Sum(nil))
		}
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", method, path, timestamp, nonce, payload)
}

func computeHmacSha256(secret, data string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// sensitiveKeyPattern 匹配需要脱敏的敏感字段名（不区分大小写）
var sensitiveKeyPattern = regexp.MustCompile(`(?i)(password|passwd|pwd|app_?secret|secret|signature|token|access_?token|refresh_?token|api_?key|private_?key|credit_?card|cvv|ssn)`)

// sanitizeHeaderValue 对 HTTP 头部 JSON 字符串中的敏感字段值进行脱敏
func sanitizeHeaderValue(headerJSON string) string {
	if headerJSON == "" {
		return ""
	}
	var headers map[string][]string
	if err := json.Unmarshal([]byte(headerJSON), &headers); err != nil {
		return "[unparseable header]"
	}
	for k := range headers {
		if sensitiveKeyPattern.MatchString(k) {
			headers[k] = []string{"***REDACTED***"}
		}
	}
	out, err := json.Marshal(headers)
	if err != nil {
		return "[unparseable header]"
	}
	return string(out)
}

// sanitizeBody 对请求/响应体中的敏感字段值进行脱敏
// 支持 JSON 对象和嵌套对象，对匹配敏感字段名的 value 替换为 ***REDACTED***
func sanitizeBody(body string) string {
	if body == "" {
		return ""
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		// 非 JSON 体，直接返回（避免明文密码等被记录，但无法结构化脱敏）
		return body
	}
	sanitized := sanitizeValue(parsed)
	out, err := json.Marshal(sanitized)
	if err != nil {
		return "[unparseable body]"
	}
	return string(out)
}

func sanitizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(val))
		for k, vv := range val {
			if sensitiveKeyPattern.MatchString(k) {
				result[k] = "***REDACTED***"
			} else {
				result[k] = sanitizeValue(vv)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, vv := range val {
			result[i] = sanitizeValue(vv)
		}
		return result
	default:
		return v
	}
}
