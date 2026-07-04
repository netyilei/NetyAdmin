# Fitness 项目规范

> ⚠️ AI 助手注意：本文件由项目维护，**不会被运行环境自动注入上下文**。
> 请你在**接到任何任务后、动手之前**，先主动用工具读取以下文件并严格遵守：
>
> 1. 本文件 `AGENTS.md`（项目总规范）
> 2. `RULES.md`（红线规则，违反即为错误）
> 3. `SHARED.md`（共享知识库）
> 4. 若涉及实现，再读 `DESIGN.md`
> 文件不存在则跳过。忽略此读取步骤属于执行缺陷。
>
> ℹ️ 项目根 `README.md` 顶部设有「AI 必读」提醒块，会随运行环境被动注入上下文并指向本文件；
> 即便因片段截断未看到该提醒，**本文件仍须由你主动读取**——这是 AI 的固定动作，不依赖任何自动注入。

---

# 项目概述

- **项目名称**: Fitness
- **项目介绍**: 让健身者用手机生成自己的 3D 形象，直观看到「真实身材 → 目标身材」的演变，用可视化的进步激励坚持。

# 技术栈

- App：Flutter + flutter_scene
- 后端：Go（NetyAdmin 基座）+ Python（AI/3D 微服务）
- GM 后台：Vue3 + soybean-admin（NetyAdmin 自带）
- 存储：PostgreSQL + Redis + 腾讯云 COS（含图片处理/CDN）

# 目录结构

```
Fitness/
├─ docs/                        项目文档（按类型分类，索引见 docs/README.md）
│  ├─ shared/                   共用文档（PRD/架构/API规范/合规等）
│  ├─ frontend/                 前端文档（Flutter App + GM 前台 UI）
│  ├─ backend/                  后端文档（Go 服务端 + Python 微服务 + 部署）
│  │   └─ api-ws/               ⭐ 服务端↔GM管理后台 API/WS 权威协议
│  └─ api-ws/                   ⭐ App↔服务端通信唯一凭证（API + WebSocket，后端主导）
│  └─ README.md                 文档索引
├─ server/                      Go 后端（NetyAdmin 基座，cmd/internal/migrations/...）
├─ admin-web/                   GM 后台前端（Vue3 + soybean-admin，pnpm）
├─ services/                    后端 Python 微服务（AI/3D：measure/body/generate/video/motion/morphtool + common）
├─ app/                         客户端 App（Flutter + flutter_scene，含 H5）
├─ deploy/                      部署配置（docker/、k8s/）
│  ├─ data/                     Docker 持久化数据（不入库）
│  └─ logs/                     日志（不入库）
├─ scripts/                     开发脚本（Windows .bat / Linux-Mac .sh，一键启停）
├─ docker-compose.dev.yml       本地开发中间件（PostgreSQL + Redis）
├─ Makefile                     命令速查
├─ .env.example                 环境配置模板
├─ .gitignore
└─ README.md                    本文件
```

---

# 开发约定

1. **红线规则**: 开发前必须阅读 `RULES.md`，逐条检查不违反。
2. **共享知识**: 开发前阅读 `SHARED.md`，完成后如果有新的知识沉淀，更新 `SHARED.md`。
3. **设计方案**: 如果项目根目录下有 `DESIGN.md`，实现必须严格遵循。
4. **提交规范**: 使用 `<type>(<scope>): <subject>` 格式。例如：
   - `feat(api): add user registration endpoint`
   - `fix(ui): resolve table overflow on mobile`
   - `refactor(rust): extract shared ipc module`

---

# 文件说明

| 文件 | 用途 | 何时读 |
|:-----|:------|:-------|
| `AGENTS.md` | 本文件 — 项目规范 | 每次任务开始时 |
| `RULES.md` | **红线规则** | **开发前必须读** |
| `SHARED.md` | 共享知识库 | **开发前必须读**，完成后按需更新 |
| `DESIGN.md` | 架构设计方案 | 如果存在，实现前必须读 |
| `AUDIT_REPORT.md` | 审计报告 | code-auditor 输出，开发者参考 |

---

# 开发流程

```
设计阶段 ─→ 将方案编写到 DESIGN.md
    ↓
实现阶段 ─→ 读取 DESIGN.md → 按方案编码
    ↓
审计阶段 ─→ AI 审计代码后 输出 AUDIT_REPORT.md
    ↓
知识沉淀 ─→ 完成后更新 SHARED.md
```

> 涉及用户资产/资金的代码，必须经过经过严格审查才能合并。
> 各目录详细文档索引见 [`docs/README.md`](docs/README.md)。前后端接口以 `docs/api-ws/` 为唯一凭证。
