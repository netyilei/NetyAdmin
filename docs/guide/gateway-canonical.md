# 网关 HMAC 签名契约（canonical spec）

> 面向第三方 SDK 实现：任何语言按本契约实现签名即可调用 `/gateway/v1` 业务 API，并获取 WS ticket。
> 与实现逐字段对照：`internal/kernel/security/authn/hmac.go`（签名）+ `internal/modules/gw/adapters/edge.go`（八步管线）。

## 签名要素

| 要素 | 值 |
|------|-----|
| 算法 | HMAC-SHA256，hex 编码（小写） |
| 密钥 | App Secret（管理端创建应用时下发；env `NETYADMIN_GW_SECRET_KEY` 为平台加密钥，非签名密钥） |
| 时间窗 | ±60 秒（`timestamp` 为 Unix 秒） |
| nonce | 随机串，TTL 2 分钟防重放（Redis 记账） |

## canonical 签名串

```
stringToSign = method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + payloadHash
```

- `method`：大写 HTTP 方法。
- `path`：请求路径（含 `/gateway/v1` 前缀，不含 query）。
- `payloadHash`：
  - **GET**：query 按 key 排序后 `k1=v1&k2=v2` 全值拼接串**原直参与签名**（同名参数全值逐个拼入防参数走私；无 query=空串）；
  - **非 GET**：请求 body 原文的 **SHA256 hex**。

## 请求头

| 头 | 值 |
|----|-----|
| `X-NA-App-Id` | App Key |
| `X-NA-Timestamp` | Unix 秒 |
| `X-NA-Nonce` | 随机串 |
| `X-NA-Signature` | HMAC-SHA256 hex |

## 失败形态（统一信封——18xxx 细分码族，契约=contracts/status-codes.yaml）

| 场景 | 码（HTTP） |
|------|-----|
| App 不存在 | 18101（401） |
| App 禁用 | 18102（403） |
| 时间窗外 | 18104（401） |
| 签名不符/缺凭证 | 18103（401） |
| nonce 重放 | 18105（401；2 分钟窗口） |
| quota 超限 | 18106（429） |
| scope 不符/授权组失效 | 18201（403） |
| API 未登记 | 18401（404） |
| quota Rate=0 | **不限流**（显式无限——管理面 RateLimitEnabled=true 且 Rate=0） |
| IPAC 拒绝 | fail-closed deny |
| WS ticket 失效/已用/跨面 | 18501（401 信封——握手失败响应体对 WS 协议不可见） |
| WS ticket 逻辑过期 | 18502（401 信封——Redis 键保留期 2×TTL 内按 payload 过期时间可辨；超桶统一 18501） |

## 实现参考

Go 参考实现：`internal/modules/gw/adapters/edge.go`（八步管线）；可运行调用示例与 WS 客户端示例见[下游开发指南 · 四加入网关](downstream-development.md#四-加入网关把你的-api-开放给第三方)。

## 接入步骤概览

1. 管理端「开放平台」创建应用，获得 AppKey 与 AppSecret（AppSecret 仅创建/重置时明文返回一次）。
2. 按本契约对每个请求计算签名，调用 `POST|GET|... /gateway/v1/<已登记的内部 path>`。
3. 失败按统一信封 `code` 排障（上表）；WS 先用 `POST /gateway/v1/ws/tickets` 换 ticket 再 upgrade。

适用版本 v2.0.0

