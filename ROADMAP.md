# AI Gateway 产品路线图

> **Roadmap v3.0** | **更新日期**: 2026-06-02
> 本文档面向产品与研发团队，规划后续版本的开发优先级和功能范围。
> 基于 [中期审查建议书](docs/MIDTERM_REVIEW_RECOMMENDATIONS.md) 调整。

---

## 已完成里程碑

### v1.0 (Phase 1) — MVP 核心骨架 ✅
**目标**: 能代理 OpenAI 请求的最小闭环

- [x] Go 模块框架 + HTTP Server
- [x] OpenAI 兼容接口 (Chat/Embeddings)
- [x] API Key 认证 + 速率限制
- [x] 模型名路由 (glob 匹配 + 优先级)
- [x] 负载均衡 (加权轮询 + 健康检查)
- [x] Prometheus 指标 + OTel 追踪
- [x] Admin API + 基础管理端点

### v1.5 (Phase 2) — 多提供商与服务化 ✅
**目标**: 多 AI 提供商支持，可投入生产

- [x] Anthropic / Google / Azure 适配器
- [x] Provider 自动 fallback
- [x] Redis 分布式限流
- [x] 配置文件热加载
- [x] K8s 部署清单 (Deployment/Service/ConfigMap/HPA)

### v2.0 (Phase 3) — 差异化竞争力 ✅
**目标**: 企业级 AI 治理能力

- [x] 语义路由 (embedding 相似度)
- [x] Prompt 注入检测引擎 + 内置规则
- [x] PII 自动脱敏 (邮箱/电话/IP/Key/SSN/信用卡)
- [x] 语义缓存 (基于语义相似度)
- [x] 成本控制 (Token 配额 + 预算告警 + 自动降级)
- [x] Web 管理控制台
- [x] Prompt 模版管理 + 版本控制
- [x] Webhook 通知 (成本告警 / 安全事件 / 上游故障)
- [x] 插件系统框架 (Provider / Security / Router)
- [x] Python / Node SDK
- [x] OpenAPI 规范文档
- [x] Grafana Dashboard

---

## 下一阶段规划

> **核心策略调整**: 下一阶段不再继续横向堆叠功能，而是聚焦生产可信度和企业治理能力。
> 将产品定位为 **"面向企业私有化部署的 AI 治理网关"**，暂缓大规模 AI 中台能力建设。

### v2.1: 生产可信版本 (当前优先级)

> **目标**: 让产品可部署、可接入、可观测、可审计、可验证。
> 用户能够在 10 分钟内完成部署，在 30 分钟内接入真实模型请求，并在管理后台看到可验证的用量、成本、审计和安全数据。

| 优先级 | 功能 | 说明 | 工作量 |
|--------|------|------|--------|
| 🔴 P0 | **Phase 3 模块完整管线集成** | Security / Cost / Semantic Router / Semantic Cache 完整接入中间件链，避免仅在局部 handler 调用 | 3d |
| 🔴 P0 | **流式响应生产级兼容** | 验证 OpenAI SDK 流式兼容性；覆盖 OpenAI / Anthropic / Gemini / Azure Provider 流式格式转换 | 2d |
| 🔴 P0 | **成本记录逻辑补全** | Token 统计、模型维度 + API Key 维度成本记录，Dashboard 展示成本趋势和 Top N 高消耗 | 2d |
| 🔴 P0 | **审计日志持久化** | 从内存 ring buffer 迁移到持久化后端 (SQLite / PostgreSQL)，支持过滤查询和导出 | 3d |
| 🔴 P0 | **配置校验与回滚** | 配置提交前校验 + 精确错误反馈，配置变更保留版本可回滚 | 2d |
| 🔴 P0 | **Docker Compose 一键体验** | gateway + redis + UI + mock provider，开箱即用的 demo 配置 | 1d |
| 🔴 P0 | **集成测试与契约测试** | 覆盖 /v1/chat/completions、/v1/embeddings、主要 /admin/* 接口的关键路径 | 3d |
| 🟡 P1 | **API Key 权限管控** | 按角色 (admin/readonly) 限制 API 可访问范围 | 2d |
| 🟡 P1 | **安全策略配置 UI** | 在管理后台中可视化编辑安全检测规则和 PII 脱敏策略 | 2d |
| 🟡 P1 | **用量统计面板** | Dashboard 增加 Token 用量 / 成本趋势图表 | 2d |
| 🟡 P1 | **Webhook 告警闭环** | 完整验证从事件触发到 webhook 推送的端到端流程 | 1d |
| 🟡 P1 | **OpenAPI 管理接口补全** | 补充所有 /admin/* 接口的 OpenAPI 文档 | 2d |
| 🟡 P1 | **Helm Chart 初版** | 官方 Helm Chart，支持一键部署到 Kubernetes | 2d |
| 🟢 P2 | **Quick Start 文档** | 面向新用户的 5 分钟快速上手指南 | 1d |
| 🟢 P2 | **多语言本地化** | 中英文界面切换 | 1d |
| 🟢 P2 | **健康检查面板** | 上游健康状态历史、告警配置 | 1d |

### v2.2: 企业治理版本

> **目标**: 形成企业内部团队可使用的治理产品，形成安全合规 + 成本治理 + 私有部署的竞争壁垒。

| 优先级 | 功能 | 说明 | 工作量 |
|--------|------|------|--------|
| 🔴 P0 | **多用户与 RBAC** | 用户登录、角色权限模型 (admin/editor/viewer) | 4d |
| 🔴 P0 | **团队 / 项目成本统计** | 按团队或项目维度聚合 Token 用量和成本，支持排行 | 3d |
| 🔴 P0 | **API Key 生命周期管理** | Key 创建、轮换、过期、吊销，支持绑定用户和权限 | 2d |
| 🔴 P0 | **安全策略中心** | 集中管理 Prompt Injection 规则、PII 脱敏规则、IP 黑白名单 | 3d |
| 🟡 P1 | **审计日志导出与保留** | 支持 CSV/JSON 导出，可配置保留周期和自动清理 | 2d |
| 🟡 P1 | **配置变更审批与回滚** | 关键配置变更需审批，支持一键回滚到历史版本 | 3d |
| 🟡 P1 | **高可用部署文档** | 多副本 + Redis Sentinel + 健康检查的高可用部署方案 | 2d |
| 🟡 P1 | **SSO / OIDC 集成** | 支持企业 SSO (OIDC / LDAP) 统一身份认证 | 3d |
| 🟢 P2 | **企业部署指南** | 面向企业 IT 运维的完整部署和运维手册 | 2d |
| 🟢 P2 | **性能压测报告** | 公开的吞吐量、延迟和并发测试报告 | 2d |
| 🟢 P2 | **多租户隔离** | 租户级别的数据隔离和配额管理 | 4d |

### v3.0: 智能优化版本

> **目标**: 从 AI 治理网关演进为 AI 治理与优化平台。
> ⚠️ 注意：以下能力仅在 v2.1 和 v2.2 稳定后才推进。

| 优先级 | 功能 | 说明 | 工作量 |
|--------|------|------|--------|
| 🔴 P0 | **A/B 测试引擎** | 对同一模型聚合多个 provider，按比例分配流量做效果对比 | 4d |
| 🔴 P0 | **模型效果评估** | 基于预设测试集的自动评测，辅助模型选型 | 5d |
| 🟡 P1 | **智能缓存预热** | 根据历史访问模式，主动缓存高概率查询 | 3d |
| 🟡 P1 | **多模态支持** | Image / Vision / Audio 接口的代理与适配 | 3d |
| 🟡 P1 | **知识库集成 (RAG)** | RAG 检索插件的标准接口 | 4d |
| 🟡 P1 | **MCP Server 支持** | 作为 MCP (Model Context Protocol) Host，管理工具注册 | 3d |
| 🟢 P2 | **智能模型路由** | 基于请求复杂度、成本、延迟自动选择最优模型 | 4d |
| 🟢 P2 | **团队协作** | 多用户协作、操作审批流程 | 3d |
| 🟢 P2 | **Terraform Provider** | 通过 IaC 管理网关配置 | 3d |

---

## 模块成熟度矩阵

为提升文档可信度，避免"功能已完成"和"生产可用"之间的误解。

| 模块 | 已实现 | 已集成 | 生产验证 |
|---|---:|---:|---:|
| OpenAI-compatible Proxy | ✅ | ✅ | ✅ |
| Multi-provider Routing | ✅ | ✅ | 待压测 |
| Load Balancer + Health Check | ✅ | ✅ | 待压测 |
| API Key Auth + Rate Limit | ✅ | ✅ | ✅ |
| Prompt Injection Detection | ✅ | ✅ | 待验证 |
| PII Sanitization | ✅ | ✅ | 待验证 |
| Semantic Router | ✅ | ✅ | 待验证 |
| Semantic Cache | ✅ | ✅ | 待验证 |
| Cost Control | ✅ | ✅ | 待验证 |
| Audit Logs | ✅ | ✅ | 待持久化 |
| Prompt Templates | ✅ | ✅ | 待验证 |
| Webhook Notifications | ✅ | ✅ | 待验证 |
| Plugin System | ✅ | ✅ | 待验证 |
| Admin UI | ✅ | ✅ | 待完善 |
| Python / Node SDK | ✅ | ✅ | 待完善 |
| Grafana Dashboard | ✅ | ✅ | 待验证 |
| Docker Compose | — | — | 待构建 |
| Helm Chart | — | — | 待构建 |
| Integration Tests | 部分 | — | 待补全 |

---

## 架构演进路径

```
Phase 1-3 (已完成)          v2.1 (当前)              v2.2 (下一阶段)          v3.0 (未来)
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│   Core Gateway   │  │  Production Hard │  │  Enterprise Gov  │  │  AI Optimization │
│                  │  │                  │  │                  │  │                  │
│  - Proxy         │  │  - Full Pipeline │  │  - Multi-User    │  │  - A/B Testing   │
│  - Routing       │  │  - Streaming     │  │  - RBAC          │  │  - Model Eval    │
│  - Auth/Rate     │  │  - Audit Persist │  │  - Team Cost     │  │  - RAG/MCP       │
│  - Multi-Provider│→ │  - Config Rollback│→ │  - Key Lifecycle │→ │  - Multimodal    │
│  - Semantic      │  │  - Docker Compose│  │  - Security Hub  │  │  - Smart Routing │
│  - Security/Cost │  │  - Tests         │  │  - Audit Export  │  │  - Cache Preheat │
│  - Prompt/Plugin │  │  - Helm Chart    │  │  - SSO/OIDC      │  │  - Terraform     │
└──────────────────┘  └──────────────────┘  └──────────────────┘  └──────────────────┘
```

---

## 技术债务与待改进项

### 架构层面
- [ ] Phase 3 模块（Security/Cost/Semantic Cache）未完全接入中间件链
- [ ] Cost Middleware 中 Token 用量的实际记录逻辑尚未完成（`costResponseWriter.Write` 目前是占位）
- [ ] Security Middleware 的 `sanitize` action 未实现
- [ ] Semantic Router / Cache 目前只在 `handleChatCompletion` 中调用，应抽取为独立中间件
- [ ] 配置结构体中的 `time.Duration` 字段在 JSON API 中需要自定义序列化/反序列化

### 代码质量
- [ ] 单元测试覆盖率未覆盖 Phase 3 模块（semantic/security/cost 无测试文件）
- [ ] 缺少集成测试（server 级测试）
- [ ] 缺少 API 契约测试

### 运维层面
- [ ] 审计日志仍为内存 ring buffer，缺少持久化
- [ ] 缺少 graceful shutdown 时的 Redis 数据持久化
- [ ] 缺少配置变更的版本控制与回滚
- [ ] 缺少多环境（dev/staging/prod）的配置管理策略

---

## 开发优先级原则

1. **P0 (优先级最高)**: 影响核心功能可用性或用户数据安全的功能
2. **P1 (高优先级)**: 影响用户体验或运维效率的功能
3. **P2 (中优先级)**: 锦上添花的功能，可在后续迭代中完成

### 版本建议

```
v2.1 — 生产可信版本 (预计 3-4 周)
  ├── Phase 3 模块与管线完全集成
  ├── 流式响应生产级兼容
  ├── 成本记录逻辑补全
  ├── 审计日志持久化
  ├── 配置校验与回滚
  ├── Docker Compose 一键体验
  ├── 集成测试与契约测试
  └── P1: 权限管控 + 安全 UI + 用量面板 + Webhook + Helm

v2.2 — 企业治理版本 (预计 4-6 周)
  ├── 多用户与 RBAC
  ├── 团队 / 项目成本统计
  ├── API Key 生命周期管理
  ├── 安全策略中心
  ├── 审计导出与保留
  ├── 配置审批与回滚
  └── SSO / 高可用部署

v3.0 — 智能优化版本 (预计 8-12 周)
  ├── A/B 测试引擎
  ├── 模型效果评估
  ├── 多模态支持
  ├── RAG 插件接口
  └── MCP Server 集成
```

---

## 参考

- [中期审查建议书](docs/MIDTERM_REVIEW_RECOMMENDATIONS.md) — 产品定位、竞争力分析和详细建议
