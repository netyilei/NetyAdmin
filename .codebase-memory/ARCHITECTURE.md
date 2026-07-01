# NetyAdmin 架构记录（跨会话记忆）

> 最后更新：2026-07-01
> 下次继续时说「继续迁移」

## 项目概述
Go 基座程序，Gin + GORM + PostgreSQL，Admin 管理后台 + Client API

## 当前迁移进度

### ✅ 已完成
- **Phase 1.1**: gin-contrib/cors v1.7.7 替换自研 CORS
- **Phase 1.2**: 合并 TraceID/RequestID，删除 trace.go
- **Phase 1.3**: health-go v5.5.5 替换自研健康检查器，新增 /health 端点
- **Phase 2.1**: golang-migrate v4.18.2 替换自研迁移器（57 up + 39 down，二进制嵌入）
- **Phase 2.2**: minio-go v7.2.1 替换 AWS SDK v2 存储层
- **死代码清理**: 27+ 个未使用符号已删除
- **限流抽离**: ulule/limiter v3.11.2 替换 cache.RateLimit（含 Lua 脚本），cache/manager.go 640→554 行
- **Phase 3.1 validator**: 9 个 Email 字段补 binding:"omitempty,email" tag
- **Phase 3.3b slog 统一**: 47 处 log.Printf → slog 结构化日志（10 个文件）
- **HTTP 超时统一**: app.go/wire.go 硬编码 → config.ServerConfig 读取
- **文件拆分**: user.go 701→208+198+330，admin.go 623→197+170+291，**全项目零超 600 行文件**
- **文档更新**: server-module-migration.md + server-module-storage.md

### ⏭️ 评估后跳过（ROI 低）
- **Phase 2.3 asynq**: 会丢失 Admin 运行时任务管理能力（启停/热更新/状态查询）
- **Phase 3.2 watermill**: 自研 Pub/Sub 总线 287 行质量好，替换需改 10 个调用点
- **Phase 3.3 LogBus**: LogBus 是批量 DB 写入器（不是日志框架），不能被 zerolog/slog 替换

### ✅ 红线规则全部达标
- 单文件 ≤ 600 行（零超标）
- 无魔法数字（超时/限流/健康检查等全部抽常量或读 config）
- 无死代码（staticcheck U1000 零告警）
- 无补丁代码（所有 IF 场景直接重构）

### 引入的依赖
- gin-contrib/cors v1.7.7, health-go v5.5.5, golang-migrate v4.18.2, minio-go v7.2.1, ulule/limiter v3.11.2

### 移除的依赖
- AWS SDK v2 全系列

### 图谱数据
- 5,982 节点，16,111 边（含 server Go + admin-web 前端）
- 入口: server/cmd/server/main.go
