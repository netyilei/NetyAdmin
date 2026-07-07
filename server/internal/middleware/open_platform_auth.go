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

	openDto "NetyAdmin/internal/interface/admin/dto/open_platform"
	"NetyAdmin/internal/pkg/auth"
	"NetyAdmin/internal/pkg/errorx"
	"NetyAdmin/internal/pkg/recovery"
	"NetyAdmin/internal/pkg/requestid"
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

		// appIDForLog 供 defer 闭包捕获：app 解析成功后赋值，鉴权失败时保持空串。
		// Round 7：原 c.Set("appID", ...) + c.Get("appID") 自写自读模式已删除，
		// 改用闭包捕获局部变量，避免通过 gin.Context 中转（中间件自己写自己读是反模式）。
		appIDForLog := ""

		defer func() {
			// 记录日志
			latency := time.Since(startTime).Nanoseconds()
			statusCode := c.Writer.Status()

			headerBytes, _ := json.Marshal(c.Request.Header)

			logReq := &openDto.RecordOpenLogReq{
				AppID:         appIDForLog,
				AppKey:        appKey,
				ApiPath:       c.Request.URL.Path,
				ApiMethod:     c.Request.Method,
				ClientIP:      c.ClientIP(),
				StatusCode:    statusCode,
				Latency:       latency,
				RequestHeader: sanitizeHeaderValue(string(headerBytes)),
				RequestBody:   sanitizeBody(string(requestBody)),
				ResponseBody:  sanitizeBody(writer.body.String()),
				// Task 8.5: 从 c.Request.Context() 提取 request_id，
				// 由 middleware.RequestID 中间件注入；service 层透传到 entity 后写入 DB。
				// 注意：GoSafe 闭包内执行 logSvc.Record 时已脱离原请求 ctx 生命周期，
				// 因此必须在 defer 之前取出 requestID，不能依赖 c.Request.Context() 仍然存活。
				RequestID: requestid.FromContext(c.Request.Context()),
			}

			// 异步记录（GoSafe 包裹 recover + Sentry 上报，防止 panic 影响节点）
			recovery.GoSafe("open_platform:record", func() {
				logSvc.Record(context.Background(), logReq)
			})
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
		// app 解析成功，记录 ID 供 defer 日志闭包使用
		appIDForLog = app.ID

		// 3. 检查应用状态（auth.AppStatusDisabled 镜像 entity 常量，避免 middleware 直接 import entity）
		if app.Status == auth.AppStatusDisabled {
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

		// 5. 解密 AppSecret
		appSecret, err := appSvc.GetAppSecret(c.Request.Context(), app)
		if err != nil {
			response.FailWithCode(c, errorx.CodeInternalError, "系统错误")
			c.Abort()
			return
		}

		// 6. 构造待签名字符串 (StringToSign)
		stringToSign := constructStringToSign(c, timestampStr, nonce, requestBody)

		// 7. 计算 HMAC-SHA256 签名
		expectedSignature := computeHmacSha256(appSecret, stringToSign)

		// 使用 hmac.Equal 进行恒定时间比较，防止时序攻击推导签名
		// 注意：直接比较字符串（!=）会因为短路比较泄露字节差异信息
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			response.FailWithCode(c, errorx.CodeSignatureFailed, "签名验证失败")
			c.Abort()
			return
		}

		// 8. Nonce 防重放 (使用缓存模块) - 移到签名验证之后
		// 顺序变更原因：原顺序在签名验证前 SetNX，攻击者可用任意 nonce 占用缓存槽位 2 分钟，
		// 造成 DoS 向量。现仅在签名验证通过后才 SetNX，消除 DoS 风险。
		// nonce TTL 调整为 2 分钟，覆盖整个时间戳容差窗口（±60s 共 120s）。
		set, err := appSvc.TryConsumeNonce(c.Request.Context(), appKey, nonce, 2*time.Minute)
		if err != nil || !set {
			response.FailWithCode(c, errorx.CodeSignatureFailed, "重复的请求 (Nonce)")
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

		// 路由未匹配时 Gin 会直接返回 404，根本不会进入本中间件，因此
		// c.FullPath() 在此处必有值，无需 fallback 到 c.Request.URL.Path。
		matchedPath := c.FullPath()
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

		// 构造 AppContext（仅 handler 实际需要的 3 个字段）注入 gin.Context。
		// Round 7：完成第二阶段迁移，删除 c.Set("appID", ...) 遗留 key，
		// 下游统一通过 c.Get("currentAppContext") 读取应用信息。
		appCtx := &auth.AppContext{
			ID:        app.ID,
			AppKey:    app.AppKey,
			StorageID: app.StorageID,
		}
		c.Set("currentAppContext", appCtx)
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
