// Package keys 是 configcenter 的 key 唯一封装。
//
// 铁律（docs/rules/namespaces.md）：业务代码禁止裸 key 字符串，一律 import 本包。
// configcenter 的首个 key 需求=热更键契约（动态配置键语义——非 Redis key，
// 是 sys_configs 的 group/key 命名契约）：GroupLog 组 + 模块/类型级别键。
package keys

// GroupLog 是日志级别动态配置组（热更消费者=log.Manager——cmd wireConfigHotReload）。
const GroupLog = "log"

// GroupTaskcenter 是任务中心暂停态组（F2：键=job 名，值=true/false，TypeBool；
// taskcenter pause/resume 写入→变更广播→各节点 scheduler.SetPaused 集群一致）。
const GroupTaskcenter = "taskcenter"

// GroupMessage 是消息模块运行期配置组（SMTP 发送参数；admin「邮件设置」写入
// →变更广播→各节点重建 SMTP 驱动，集群一致）。密码不在此组——env-only。
const GroupMessage = "message"

// SMTP 配置键契约（group=GroupMessage；启动期 config.yaml 为回退默认值）。
const (
	SMTPHostKey     = "smtp.host"
	SMTPPortKey     = "smtp.port"
	SMTPUsernameKey = "smtp.username"
	SMTPFromKey     = "smtp.from"
)

// SMTPKeys 返回 SMTP 键全集（读/写/删除遍历的唯一源，顺序稳定）。
func SMTPKeys() []string {
	return []string{SMTPHostKey, SMTPPortKey, SMTPUsernameKey, SMTPFromKey}
}

// LogModuleKey 构造模块级别键（组内 key 形如 module:<name>）。
func LogModuleKey(module string) string { return "module:" + module }

// LogTypeKey 构造类型级别键（组内 key 形如 type:<typ>）。
func LogTypeKey(typ string) string { return "type:" + typ }

// SnapshotKey 是 Watcher 快照的复合键拼接唯一出口（group+":"+key）。
func SnapshotKey(group, key string) string { return group + ":" + key }
