# AI Gateway 产品路线图

> **Roadmap v2.0** | **更新日期**: 2026-05-31
> 本文档面向产品与研发团队，规划后续版本的开发优先级和功能范围。

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

---

## 下一阶段规划

### Phase 4: 企业级生产加固 (当前优先级)

> **目标**: 将 Phase 3 模块与中间件管线完全集成，补充企业必备的生产能力

| 优先级 | 功能 | 说明 | 工作量 |
|--------|------|------|--------|
| 🔴 P0 | **Phase 3 模块与管线集成** | 将 Semantic Router / Security / Cost / Semantic Cache 完全接入 server.go 的中间件链 | 3d |
| 🔴 P0 | **配置 Web 编辑器** | 在 UI 中支持编辑 semantic/cost/security 配置并持久化 | 2d |
| 🔴 P0 | **配置校验与错误提示** | 提交配置时的前端校验 + 后端精确错误反馈 | 1d |
| 🟡 P1 | **审计日志查询 UI** | Admin API 新增日志存储 + 前端查询页面 | 3d |
| 🟡 P1 | **用量统计面板** | Dashboard 增加 Token 用量/成本趋势图表 | 2d |
| 🟡 P1 | **API Key 权限管控** | 按角色 (admin/readonly) 限制 API 可访问范围 | 2d |
| 🟢 P2 | **多语言本地化** | 中英文界面切换 | 1d |
| 🟢 P2 | **Docker Compose 一键启动** | gateway + redis + 前端集成部署 | 1d |
| 🟢 P2 | **健康检查面板** | 上游健康状态历史、告警配置 | 1d |

### Phase 5: 生态与集成

> **目标**: 构建插件生态，支持企业定制和第三方集成

| 优先级 | 功能 | 说明 | 工作量 |
|--------|------|------|--------|
| 🔴 P0 | **Prompt 模版管理** | 系统级/system prompt 管理，变量注入，版本控制 | 3d |
| 🔴 P0 | **Webhook 通知** | 成本告警、安全事件、上游故障的 webhook 推送 | 2d |
| 🟡 P1 | **插件系统** | 自定义 provider/security/route 插件的加载框架 | 5d |
| 🟡 P1 | **OpenAPI 规范** | 统一 Admin API 的 OpenAPI/Swagger 文档 | 2d |
| 🟡 P1 | **Python/Node SDK** | 官方 SDK 封装，简化对接 | 3d |
| 🟢 P2 | **Terraform Provider** | 通过 IaC 管理网关配置 | 3d |
| 🟢 P2 | **Grafana Dashboard** | 官方 Grafana 监控面板模板 | 1d |

### Phase 6: AI 原生能力

> **目标**: 从"网关"进化为"AI 中台"

| 优先级 | 功能 | 说明 | 工作量 |
|--------|------|------|--------|
| 🔴 P0 | **A/B 测试引擎** | 对同一模型聚合多个 provider，按比例分配流量做效果对比 | 4d |
| 🔴 P0 | **模型效果评估** | 基于预设测试集的自动评测，辅助模型选型 | 5d |
| 🟡 P1 | **智能缓存预热** | 根据历史访问模式，主动缓存高概率查询 | 3d |
| 🟡 P1 | **多模态支持** | Image/Vision/Audio 接口的代理与适配 | 3d |
| 🟡 P1 | **知识库集成** | RAG 检索插件的标准接口 | 4d |
| 🟢 P2 | **MCP Server 支持** | 作为 MCP (Model Context Protocol) Host，管理工具注册 | 3d |
| 🟢 P2 | **团队协作** | 多用户/多租户、操作审计、审批流程 | 5d |

---

## 架构演进路径

```
Phase 1-3 (已完成)              Phase 4 (当前)                Phase 5-6 (未来)
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   Core Gateway   │ ──► │  Enterprise Hard │ ──► │  AI Middleware   │
│                  │     │                  │     │   Platform       │
│  - Proxy         │     │  - Full Pipeline │     │                  │
│  - Routing       │     │  - Config UI     │     │  - Plugin System │
│  - Auth/Rate     │     │  - Audit/Usage   │     │  - A/B Testing   │
│  - Multi-Provider │    │  - RBAC          │     │  - RAG/MCP       │
│  - Semantic      │     │  - L10N          │     │  - Multi-Tenant  │
│  - Security/Cost │     │  - Docker Compose│     │  - Auto Eval     │
└──────────────────┘     └──────────────────┘     └──────────────────┘
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
- [ ] 缺少 graceful shutdown 时的 Redis 数据持久化
- [ ] 缺少配置变更的版本控制
- [ ] 缺少多环境（dev/staging/prod）的配置管理策略

---

## 开发优先级原则

1. **P0 (优先级最高)**: 影响核心功能可用性或用户数据安全的功能
2. **P1 (高优先级)**: 影响用户体验或运维效率的功能
3. **P2 (中优先级)**: 锦上添花的功能，可在后续迭代中完成

### 版本建议

```
v2.1 — Phase 4 完成 (预计 2-3 周)
  ├── Phase 3 模块与管线完全集成
  ├── 配置 Web 编辑器 (semantic/cost/security)
  ├── 审计日志查询 + 用量统计面板
  └── 权限管控 + Docker Compose

v2.2 — Phase 5 完成 (预计 4-6 周)
  ├── Prompt 模版管理
  ├── Webhook 通知系统
  ├── 插件系统框架
  └── 官方 SDK

v3.0 — Phase 6 完成 (预计 8-12 周)
  ├── A/B 测试引擎
  ├── 模型效果评估
  ├── 多模态支持
  └── MCP Server 集成
```
