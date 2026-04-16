# IP 访问控制模块详解

本文档详细介绍 NetyAdmin IP 访问控制模块（IPAC）的架构设计、匹配算法及二次开发指南。

---

## 一、模块概述

IPAC 模块提供全局及应用级的 IP 流量治理能力，支持精确 IP 匹配及 CIDR 网段匹配。通过内存树（Memory Tree）架构实现高性能过滤。

### 1.1 核心特性

- **分级过滤**：支持全局规则（最高优先级）与应用级独立规则。
- **CIDR 支持**：支持 `192.168.1.1` 单 IP 及 `10.0.0.0/8` 网段匹配。
- **高性能匹配**：规则在启动或变更时全量预加载至内存，匹配耗时 < 0.1ms。
- **分布式同步**：利用 Redis Pub/Sub 机制，实现规则变更全网节点毫秒级热重载。
- **白名单/黑名单**：支持 Allow（放行）与 Deny（封禁）两种动作。

---

## 二、目录结构

```
server/internal/domain/entity/ipac/
├── ipac.go             # IPAC 规则实体定义

server/internal/repository/ipac/
├── ipac.go             # IPAC 仓储实现

server/internal/service/ipac/
├── ipac.go             # 【核心】匹配逻辑与内存缓存管理

server/internal/middleware/
└── ipac_auth.go        # 全局 IP 拦截中间件 (可选)
```

---

## 三、匹配优先级

系统遵循 **“封禁优先，全局优先”** 原则：
1. **全局封禁 (Global Deny)**：一旦命中，立即拦截。
2. **全局放行 (Global Allow)**：一旦命中，立即放行。
3. **应用封禁 (App Deny)**：若请求携带 AppKey，则检查该应用的黑名单。
4. **应用放行 (App Allow)**：检查该应用的白名单。
5. **默认行为**：若均未命中，执行默认放行。

---

## 四、数据模型

### 4.1 IP 规则表 (`sys_ip_access_control`)

```go
type IPAccessControl struct {
    ID        uint       `gorm:"primaryKey"`
    AppID     *string    `gorm:"size:26;index"` // NULL 为全局规则
    IPAddr    string     `gorm:"size:50"`       // IP 或 CIDR (1.1.1.1/24)
    Type      int        `gorm:"default:2"`     // 1: Allow, 2: Deny
    Reason    string     `gorm:"size:255"`      // 原因
    ExpiredAt *time.Time `json:"expiredAt"`     // 过期时间 (可选)
    Status    int        `gorm:"default:1"`     // 1: 启用, 0: 禁用
}
```

---

## 五、API 接口 (Admin)

| Method | Path | 说明 |
|--------|------|------|
| GET | /admin/v1/ops/ip-access | 获取规则列表 (支持按 IP/类型筛选) |
| POST | /admin/v1/ops/ip-access | 新增 IP 规则 |
| PUT | /admin/v1/ops/ip-access | 修改规则 (状态/过期时间) |
| DELETE | /admin/v1/ops/ip-access/:id | 删除规则 |
| DELETE | /admin/v1/ops/ip-access/batch | 批量删除规则 |

---

## 六、二次开发示例

### 6.1 在业务 Service 中手动检查 IP

虽然开放平台中间件已集成 IPAC，但在某些特定业务中（如：高风险红包领取），可能需要手动检查：

```go
func (s *bonusService) ClaimBonus(ctx context.Context, userID string, clientIP string) error {
    // 检查该 IP 是否被封禁
    allowed, err := s.ipacSvc.CheckIP(ctx, clientIP, nil)
    if err != nil || !allowed {
        return errorx.CodeIPBlocked
    }
    // 继续业务...
}
```

### 6.2 扩展 IP 地理位置自动封禁

**1. 引入地理位置库**

```go
func (s *ipacService) AutoBlockByRegion(ctx context.Context, ip string, region string) {
    if region == "SuspiciousArea" {
        s.Create(ctx, &ipac.IPAccessControl{
            IPAddr: ip,
            Type:   ipac.IPACTypeDeny,
            Reason: "Auto blocked by region security policy",
        })
    }
}
```

---

## 七、最佳实践

1. **CIDR 慎用**：在配置全局 Allow 时，避免使用过大的网段（如 `/8`），防止权限泄露。
2. **过期设置**：对于临时封禁，务必设置 `ExpiredAt`，避免数据库记录无限增长。
3. **缓存重载**：在手动修改数据库（非 API 方式）后，需发送 Redis 消息 `netyadmin:ipac:reload` 触发全网重载。
4. **性能监控**：若规则数量超过 1 万条，建议将 `net.IPNet` 的切片结构重构为 **Radix Tree** 或 **Trie Tree** 以优化匹配速度。

---

## 八、相关文档

- [Server架构设计](./server-architecture.md)
- [开放平台模块](./server-module-open-platform.md)
- [缓存模块详解](./server-module-cache.md)
