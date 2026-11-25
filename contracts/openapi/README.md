# HTTP API 契约（OpenAPI 3.1）

对外 HTTP 接口的单一事实源。三份 spec 覆盖全部接入面，**由代码 swag 注解生成，禁止手改生成物**。

## 三面 spec

| 文件 | 接入面 | 路径前缀 | 规模（paths / operations） | 认证 |
|------|--------|----------|---------------------------|------|
| [admin.v1.yaml](admin.v1.yaml) | 管理端 | `/admin/v1/*` | 77 / 103 | `BearerAuth`（`Authorization: Bearer <access_token>`）；权限码由 RBAC 校验 |
| [client.v1.yaml](client.v1.yaml) | C 端示例面 | `/client/v1/*` | 13 / 13 | `BearerAuth`（User JWT；WS upgrade 用 ticket query） |
| [gateway.v1.yaml](gateway.v1.yaml) | 开放平台网关 | `/gateway/v1/*` | 3 / 7 | `GatewayHMAC` 四头（`X-NA-*`），见 [签名契约](../../docs/guide/gateway-canonical.md) |

每份 spec 的 `servers[0].url` 是**路径前缀**（如 `/admin/v1`），完整地址 = 你的部署域名 + 该前缀；`info.title/version` 标识面与契约版本（当前 1.0.0）。

## 下游怎么用

- **浏览/调试**：把 yaml 导入 Swagger UI、Postman 或 Apifox 即可看到全部端点、参数与响应 schema；连本机 `http:/7.0.0.1:8080` 直接调。
- **生成客户端**：`openapi-generator-cli generate -i contracts/openapi/client.v1.yaml -g typescript-fetch -o sdk/client`（任意语言/生成器同理）；`operationId` 全局唯一且稳定（`{surface}.{module}.{resource}.{action}`），适合作为方法名。
- **响应结构**：所有端点统一信封 `Envelope`：成功 `{code:"10000", message_key, data}`；失败 `{code, message_key, details, request_id, trace_id}`。分页 `data:{list,total,page,page_size}`。
- **错误码**：`code` 为 5 位字符串，含义与 HTTP 映射见 [状态码规范](../../docs/rules/status-codes.md) / [status-codes.yaml](../status-codes.yaml)。
- **认证（spec 内已声明）**：三份 spec 的 `components.securitySchemes` 与根级 `security` 均填写，可直接用 Swagger UI「Authorize」注入、或用生成器自动加头。
  - `BearerAuth`（admin/client）：`Authorization: Bearer <access_token>`（登录端点换发；refresh 用 refresh_token）——定义来自 swag `@securityDefinitions.apikey`。
  - `GatewayHMAC`（gateway）：`X-NA-App-Id / X-NA-Timestamp / X-NA-Nonce / X-NA-Signature` 四头，canonical 见 [签名契约](../../docs/guide/gateway-canonical.md)；scheme 仅锚定 `X-NA-Signature`，其余三头为 HMAC 计算输入。
  - 免鉴权 operation 显式 `security: []`（admin: login/captcha/refresh-token/configs-public；client: register/login/ws；gateway: ws）——由 `split-openapi.mjs` 的 `publicPaths` 维护，新增公开端点须同步该清单。
- **WS 端点**：`/ws` upgrade 与 `/ws/tickets` 在 spec 中可见，帧协议见 [WS 协议规范](../../docs/rules/ws-protocol.md)。

## 生成与门禁（维护者）

```bash
# ① swag v2（v2.0.0-rc6+；v1.x 无 --v3.1）收齐三面注解（会混在同一份临时 spec）
swag init --v3.1 --parseDependency --parseInternal -g cmd/netyadmin/main.go -o <tmp> --ot yaml
# ② 按 path 前缀切分 + $ref 闭包 + securitySchemes/root security 注入，产出三份 yaml
node scripts/gen/split-openapi.mjs <tmp>/swagger.yaml
# ③ 校验
node scripts/ci/run.mjs        # check-openapi；CI 对基线做 oasdiff 破坏性 diff
```

- 变更流程：改 handler 上的 swag 注解 → 重新生成 → 入库三份 yaml；注解落在已知前缀之外会切分失败。
- 安全面：`BearerAuth` 由 main.go `@securityDefinitions.apikey` 生成；`GatewayHMAC` 与根级 `security`/免鉴权 `security: []` 由切分脚本注入（swag v2 生成物只保留单一 securityDefinitions 块）。新增免鉴权端点必须同步 `SURFACES[].publicPaths`。
- 兼容承诺：破坏式变更升路径版本（`/admin/v1` → `/admin/v2`）；发布 tag 即该版本的契约快照。
- 新增接入面：先补 `scripts/gen/split-openapi.mjs` 的 `SURFACES` 前缀规则，再写注解。

适用版本 v2.0.0
