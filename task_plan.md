# Documentation Update Plan

## Status: ALL COMPLETED

## Completed Deliverables

### 1. 消息模块文档更新 (docs/server-module-message.md) ✅

**更新内容：**

- 核心特性：新增多认证方式、STARTTLS 支持、灵活加密说明
- 目录结构：标注 email_driver.go 基于 go-simple-mail
- 数据模型：补全 `MsgRecord` 的 `UserID`/`NodeID`/`ErrorMsg`/`CreatedAt`/`UpdatedAt` 字段，补充字段说明注释
- 发送流程：细化步骤，加入状态更新说明
- **新增 4.3 Email 配置读取策略**：说明 email_config 走 DB 直查不做缓存的原因（有意设计：低频操作+实时生效需求）
- **新增第五章 Email 驱动详解**：
  - 5.1 依赖 go-simple-mail
  - 5.2 配置项表格（10 项：enabled/host/port/user/password/from/ssl_enabled/starttls_enabled/auth_type/connect_timeout/send_timeout）
  - 5.3 加密与认证矩阵（端口 465/587/25 × 认证方式）
  - 5.4 常见 SMTP 服务配置示例（QQ/163、Gmail/Outlook、企业邮箱）
- API 接口：更新路径 `POST /admin/v1/message/send`（原文档写的是 `/send/direct`），补充 `POST /admin/v1/message/records/:id/retry`
- 最佳实践：新增 3 条（端口选择、认证方式、超时配置）
- 相关文档：新增状态码规范链接

---

### 2. README.md (中文) ✅

**更新内容：**

- 高性能与高可用：新增 `- **Email 驱动**: 基于 go-simple-mail，支持 SSL/TLS、STARTTLS、多种 SMTP 认证方式`
- 消息中心特性：`STARTTLS 支持` 加入描述

---

### 3. README.en-US.md (英文) ✅

**更新内容：**

- 与 README.md 同步的英文更新

---

### 4. 消息发送日志列表前端问题修复 ✅

**问题根因：**
`message-log/index.vue` 缺少 `loadDicts()` 调用，导致字典数据从未被加载。字典数据用于表格列的 `renderDictTag` 渲染通道、状态、优先级的彩色标签。虽然 renderDictTag 在字典为空时回退到原始值（不崩溃），但这意味着表格列只能显示原始值而没有彩色标签。

**修复方案：**

**文件 1：** `admin-web/src/views/ops/message-log/index.vue`

```typescript
const { loadDicts, renderDictTag } = useDict();
loadDicts(['sys_msg_channel', 'sys_msg_status', 'sys_msg_priority']);
```

**文件 2：** `admin-web/src/views/ops/message-log/components/msg-record-detail-modal.vue`

- 新增 `import { watch } from 'vue'`
- 解构 `loadDicts`
- 在 setup 顶层调用 `loadDicts(['sys_msg_channel', 'sys_msg_status', 'sys_msg_priority'])`
- 新增 `watch` 在弹窗打开时主动加载字典数据

**对比 operation-log/index.vue：** 该文件正确调用了 `loadDicts(['sys_operation_action'])`，因此能正常显示操作类型的彩色标签。message-log 是唯一遗漏。

**TypeScript 注意事项：** 该项目 tsconfig.json 的 `noImplicitAny: false` + `strict: false`，但 `vue-tsc --noEmit` 仍要求显式导入 Vue 组合式 API（与纯 Vue SFC 单文件编译不同）。所以 watch 需要显式 import。

---

## Lint & Typecheck

- `pnpm lint`: ✅ 通过
- `pnpm run typecheck`: ✅ 通过

---

## 文件变更摘要

| 文件 | 变更类型 |
|------|----------|
| docs/server-module-message.md | 更新 |
| README.md | 更新 |
| README.en-US.md | 更新 |
| admin-web/src/views/ops/message-log/index.vue | 修复 |
| admin-web/src/views/ops/message-log/components/msg-record-detail-modal.vue | 修复 |
