# 安全基线（security）

> 全局红线的项目化落地。认证/授权/限流/IPAC 的机制设计见；本文件是"必须做到"的清单。

## 1. 密钥与凭证

- 密钥来源：env / file / vault 三种 Provider（kernel/config）；**生产禁占位符，启动 fail-closed**（CodeSecretMissing）。
- 密钥不入仓库、不入日志、不入导出；`.env` 类文件已 gitignore，示例文件只放占位。
- JWT RS256 私钥轮换：新旧公钥重叠窗口验证（hot-update.md §密钥重叠）；上传 HMAC 密钥同理。
- 默认账号 admin + **首启随机密码**（AGENTS.md 参数表），首登强制改密。

## 2. 认证与授权

- 密码：bcrypt/argon2，成本参数进配置；禁止可逆加密存密码。
- Token：RS256 + TokenVersion 吊销；吊销事件 `identity.token.revoked` 广播。
- 授权：Casbin 判定 + 路由元数据声明所需权限；**接收方端口必须二次校验**（服务间调用不信任内网）。
- 审计必记事件：登录成功/失败、越权、限流触发、密钥/配置变更、模块启停（audit 类型 security/operation）。

## 3. PII 与数据分类

- Manifest `data_classification` 标注（public/internal/confidential/pii）；**`pii` 级字段不得明文落库**——加密与盲索引机制由承载该字段的模块实现（基座不预建：G1-15，当前无具名消费者；盲索引可用 HMAC 模式，参考 primitives 的 HMAC 原语）。
- 日志/审计/导出**禁明文 PII**（密码/Token/手机号/证件号/银行卡等，按项目实际字段清单）；脱敏在 log.Manager 出口统一做（logging.md），按 kind 分发到掩码注册表（`primitives.RegisterMasker/Mask`）——**机制零预设**：使用方对自有 PII 字段（手机号/邮箱/证件号/银行卡等）自行注册，不需要 PII 处理可不注册（机制零介入）；未注册 kind 被调用时全文遮蔽（fail-closed）。
- 删除与导出要求见 data.md §5。

## 4. 输入与上传

- 一切输入服务端校验前置（validator DTO）；校验失败 `10001002`。
- 上传：MIME + 文件头**双校验**、大小上限、类型白名单（storage 模块）；禁止用户可控路径拼接。
- 禁 `eval` 类动态执行处理用户输入；SQL 一律参数化（全局红线）。

## 5. 传输与 Web 基线

- CORS 白名单，**禁通配符** + 凭证组合；安全头（CSP/HSTS/X-Frame-Options 等）由 transport 统一注入。
- Body limit / request 超时 / recovery 全接入面默认开启；公开 API 必须限流（维度/算法/失败模式见策略引擎）。
- 生产必须 HTTPS（外部边缘 TLS）；内部 RPC 生产建议叠加 mTLS。
- **反代部署必须登记 `server.trusted_proxies`**（代理 CIDR 白名单）：不登记=不解析 `X-Forwarded-For`，`ClientIP` 退化为直连地址——审计 IP、登录限流、网关 IPAC/配额会全部按代理 IP 判定（多客户端共用代理 IP=限流误伤 / IP 白名单失效）。

## 6. 反模式（评审红线）

- 硬编码凭证/密钥/Token；`admin123` 类弱默认；debug 开关生产开启；错误消息回显堆栈/SQL；日志记录明文密钥。
