# scripts/ 脚本台账（唯一入口：新增/删减/移动/用法变更必须同步本文件）

## tools/
- `test-coverage.sh` — Service 层覆盖率门禁（≥70%），见 docs/server-architecture.md §6.4。
- `upgrade_preflight.sh` — 基座升级预检（检查项自 2.0.0 起未变，验收锚点跟随当前发布）：Redis ≥8.1 / PG 可达与迁移版本 / aes_key 非默认；FAIL 即禁止升级。用法见 docs/upgrade-guide.md。

## data/
- `sequence_sync.sql` — 序列同步一次性数据订正（历史遗留，仅手工执行）。

## debug/
- `e2e_session.sh` — NetyAdmin 本地栈全链路 E2E：开放平台签名 → 登录/刷新/顶号/登出（user_tokens CAS）→ Incr TTL 补挂 → api_ratelimit 规则隔离与非法规则丢弃。依赖本地 PG(root/123456/netyadmin) + Redis + 测试用户 verifyuser01；自动在 server/tmp/e2e/ 派生独立实例配置（端口 9091）。用法：`bash scripts/debug/e2e_session.sh`，输出 PASS/FAIL 与证据行。注意：端级拒绝响应码为 101002（middleware 伪装禁用）。
