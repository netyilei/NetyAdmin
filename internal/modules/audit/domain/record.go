// record.go：审计统一记录模型（——V1 operation/access/security/
// system 四类合并）。字段契约：request_id/trace_id/biz_code 全量落库（observability.md
// §1 记录边界）；Detail 只放结构化上下文，敏感字段由产生方脱敏后传入（security.md §3
// 禁明文 PII——审计侧不二次脱敏，入参即契约）。
package domain

import "time"

// RecordType 是审计记录类别（operation_logs.type smallint 镜像）。
type RecordType int8

const (
	TypeOperation RecordType = 1 // 管理端变更（application 写操作）
	TypeAccess    RecordType = 2 // API/WS 访问（transport 产生）
	TypeSecurity  RecordType = 3 // 登录失败/越权/限流（security 触发）
	TypeSystem    RecordType = 4 // 配置/模块变更（system 级）
)

// Record 结果取值（Result 字段；审计只记事实不做枚举校验——产生方契约）。
const (
	ResultSuccess = "success"
	ResultDeny    = "deny"
	ResultError   = "error"
)

// Record ActorType 取值（产生方契约——使用方 import 本包，禁止字面量）。
const (
	ActorAdmin     = "admin"
	ActorUser      = "user"
	ActorService   = "service"
	ActorApp       = "app"
	ActorSystem    = "system"
	ActorAnonymous = "anonymous" // 未认证请求（access 面匿名来源——暴破可见性）
)

// Record 是一条审计记录（Sink 写入的最小单元；json 面向查询响应——
// api-style §2 camelCase；写路径走 CopyFrom 列序不受 tag 影响）。
type Record struct {
	ID        string         `json:"id"`        // ULID（产生方生成；幂等/去重依据）
	Type      RecordType     `json:"type"`      // 类别
	ActorType string         `json:"actorType"` // admin | user | service | app | system | anonymous
	ActorID   string         `json:"actorId"`   // Principal ID（system 记录为空）
	Action    string         `json:"action"`    // 动作（如 admin.users.update / login.failed）
	Resource  string         `json:"resource"`  // 路由/资源（如 /admin/v1/users/:id）
	Result    string         `json:"result"`    // success | deny | error
	BizCode   string         `json:"bizCode"`   // 5 位业务码（error/deny 时必带）
	IP        string         `json:"ip"`
	UserAgent string         `json:"userAgent"`
	RequestID string         `json:"requestId"` // 必带（observability.md §1）
	TraceID   string         `json:"traceId"`
	NodeID    string         `json:"nodeId"`
	Detail    map[string]any `json:"detail"`    // 结构化上下文（产生方已脱敏）
	CreatedAt time.Time      `json:"createdAt"` // 零值由 Sink 落库时补 now（UTC）
}
