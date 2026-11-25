# admin — NetyAdmin 管理端

Vue 3 + Vite + TypeScript + Pinia + Element Plus 的管理端，基于 Geeker-Admin（MIT）定制为 NetyAdmin 基座前端。

## 开发

```bash
pnpm install
pnpm dev          # http://localhost:5173，/admin 与 /client 代理到本机 8080（含 WS）
pnpm type:check   # TS 类型检查
pnpm build:pro    # 生产构建 → admin/dist（后端启动时同源挂载）
```

- 开发期接口走 `.env.development` 的 `VITE_PROXY`（不剥前缀，后端路由自带 `/admin/v1`）；生产 `VITE_API_URL=""` 表示同源。
- 后端需先启动（默认 8080）并完成迁移；账号 `admin`，首启密码由迁移命令输出一次（首次登录需改密，或 `-mode=reset-password` 重置）。

## 功能页

登录、首页、系统管理（管理员/角色/菜单/API/按钮/配置/日志/存储/消息/用户）、开放平台（应用/API/scope/IP 组）、任务中心。

## 约定

- `src/api/modules/*`：与后端三面契约对应；`src/constants/status-codes.ts` 由 `scripts/gen/status-codes.mjs` 生成（**禁手改**，改 `contracts/status-codes.yaml` 后重新生成）。
- WS：`src/core/ws/client.ts` 为 admin surface 参考实现（单例连接、ticket 握手、心跳/重连/订阅恢复），页面经 `src/hooks/useWebsocket.ts` 消费；协议常量与 `internal/modules/realtime/api/protocol` 对齐。
- 接口契约：`contracts/openapi/admin.v1.yaml`（不要再手写接口文档）。

适用版本 v2.0.0
