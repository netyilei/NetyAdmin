// Package session 实现 Session 平面：token 吊销、登录会话存储、刷新轮换。
// 策略唯一源 contracts/state-policies.yaml session 段（Go 常量镜像）：
//
//	token_verification = local_jwt（本地 RS256 校验不经本包， AuthN 组合原语）；
//	token_revocation / session_store：Redis + DB 读回退（single: db_fallback_or_fail，
//	multi: fail_closed——DB 回调不改变声明）；refresh_rotation：双拓扑 fail_closed。
//
// 行为模式：Redis 操作失败 → 按拓扑+策略分发（fail-closed 路径返回
// ErrUnavailable 哨兵，调用方 errors.Is 可判）；键前缀 sess:* 具名常量。
// 已知权衡：吊销标记丢失（Redis 重启未持久化）窗口见 faces.go IsRevoked 注释。
package session
