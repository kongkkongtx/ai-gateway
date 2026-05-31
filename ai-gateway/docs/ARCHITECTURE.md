# AI Gateway 架构设计文档

> **版本**: 2.0 | **最后更新**: 2026-05-31
> **定位**: 应用层 AI 治理中间件 — 企业级多云 AI 网关

---

## 项目定位

一个独立部署的 **Go 网关服务**，提供 OpenAI 兼容接口，在应用和 AI 提供商之间做路由、负载均衡、安全治理、成本控制、语义理解。面向企业的 **AI 基础设施层**，解决多云管理、成本失控、安全合规三大核心痛点。

---

## 已完成三阶段总览

### Phase 1: 核心骨架（MVP）
一个能跑起来、能代理 OpenAI 请求的最小闭环。

| 模块 | 实现内容 | 生产就绪 |
|------|---------|---------|
| 项目初始化 | Go 模块脚手架，目录结构，Makefile，Dockerfile | ✅ |
| 配置加载 | YAML 文件驱动，`GATEWAY_` 环境变量覆盖 | ✅ |
| HTTP Server | OpenAI 兼容接口 (`POST /v1/chat/completions`, `/v1/embeddings`) | ✅ |
| Provider 适配器 | OpenAI 适配器，流式/非流式透传 | ✅ |
| 中间件管线 | Auth → RateLimit → Audit → Recovery → Tracing | ✅ |
| Router | 模型名 glob 匹配，优先级排序 | ✅ |
| Load Balancer | 加权轮询，TCP 健康检查 | ✅ |
| Metrics | Prometheus (`/metrics`) + OpenTelemetry | ✅ |
| Admin API | `GET /admin/health`, `/admin/status` | ✅ |

### Phase 2: 多提供商与服务化（可投产）

| 模块 | 实现内容 | 生产就绪 |
|------|---------|---------|
| Anthropic 适配器 | Claude 协议转换 | ✅ |
| Google/Gemini 适配器 | Gemini 协议转换 | ✅ |
| Azure OpenAI 适配器 | 不同 auth/endpoint 格式 | ✅ |
| Provider Fallback | 超时/报错时自动切到备用 | ✅ |
| Redis 集成 | 分布式限流、集群状态共享 | ✅ |
| Config Watch | 文件变更热加载，零重启 | ✅ |
| K8s 部署 | Deployment + Service + ConfigMap + HPA | ✅ |

### Phase 3: 进阶能力（差异化竞争力）

| 模块 | 实现内容 | 生产就绪 |
|------|---------|---------|
| 语义路由 | 基于 embedding 相似度的内容路由 | ✅ 核心逻辑，待集成到 middleware |
| Prompt 安全检测 | 注入检测插件接口 + 内置规则引擎 | ✅ 核心逻辑，待集成到 middleware |
| PII 脱敏 | 邮箱/电话/IP/API Key/信用卡/SSN 正则替换 | ✅ 完整实现 |
| 语义缓存 | 基于 embedding 的响应缓存 | ✅ 核心逻辑，待集成到 middleware |
| 成本控制 | 配额管理、预算告警、自动降级到更便宜模型 | ✅ 核心逻辑，待集成到 middleware |
| Web UI | 管理控制台（Dashboard/Upstreams/Routes/Keys/Settings/Playground） | ✅ |

---

## 企业 AI 痛点与产品能力映射

通过调研当前企业（特别是中国内地企业）使用 AI 的核心痛点，以下是产品能力映射：

### 痛点 1: 多云锁定与切换成本
- **问题**: 企业深度绑定单一 AI 提供商（如 OpenAI），面临价格波动、服务不可用、政策风险
- **方案**: 统一的 OpenAI 兼容接口，多 provider 适配器 + 自动 fallback + 加权轮询
- **覆盖**: Phase 1 (LB) + Phase 2 (多提供商 + Fallback)

### 痛点 2: 成本失控与透明度不足
- **问题**: 各部门各自接入 AI，缺少统一的用量追踪和预算控制，月底账单不可控
- **方案**: Token 级别的配额管理、按 API Key 的预算限制、自动降级到更便宜的模型
- **覆盖**: Phase 3 (Cost Tracker)

### 痛点 3: 安全合规与数据泄露
- **问题**: Prompt 注入攻击、敏感数据（PII）泄露到 AI 提供商、审计缺失
- **方案**: Prompt 注入检测引擎 + PII 自动脱敏 + 全链路审计日志 + API Key 管理
- **覆盖**: Phase 1 (Auth/Audit) + Phase 3 (Security)

### 痛点 4: 大模型选型与路由混乱
- **问题**: 不同业务场景需要不同模型（代码用 Claude、对话用 GPT、推理用 DeepSeek），手动管理繁琐
- **方案**: 基于模型名称的 glob 路由 + 基于语义内容的 embedding 路由
- **覆盖**: Phase 1 (Router) + Phase 3 (Semantic Router)

### 痛点 5: 重复查询浪费资源
- **问题**: 相似问题反复请求，造成不必要的 API 调用成本
- **方案**: 基于语义相似度的缓存，相同意图的查询直接返回缓存结果
- **覆盖**: Phase 3 (Semantic Cache)

### 痛点 6: 运维复杂度过高
- **问题**: 需要独立搭建监控、告警、配置管理系统
- **方案**: 内置 Prometheus 指标 + Admin API + Web 管理控制台 + K8s 原生部署
- **覆盖**: Phase 1 (Observability) + Phase 2 (K8s) + Phase 3 (Web UI)

---

## 当前架构概览

```
                    ┌──────────────────────────────────────────────────────┐
                    │                    AI Gateway                         │
                    │                                                       │
                    │         ┌─────────┐  ┌──────────┐  ┌─────────┐      │
   App ──OpenAI───►  │ ──►    │  Auth   │─►│ RateLimit│─►│ Security│─►    │
   SDK  compat      │         └─────────┘  └──────────┘  └─────────┘      │
                    │              │              │              │         │
                    │         ┌────▼──────────────▼──────────────▼────┐    │
                    │         │         Request Pipeline              │    │
                    │         │  Semantic Router → Cost Check → Audit │    │
                    │         └────────────────┬──────────────────────┘    │
                    │                          │                          │
                    │         ┌────────────────▼──────────────────────┐    │
                    │         │        Router (Model Match)           │    │
                    │         │   glob: "gpt-4*" / semantic match     │    │
                    │         └────────────────┬──────────────────────┘    │
                    │                          │                          │
                    │         ┌────────────────▼──────────────────────┐    │
                    │         │      Load Balancer + Fallback         │    │
                    │         └──┬───────────┬───────────┬────────────┘    │
                    │            │           │           │                 │
                    │         ┌──▼──┐    ┌──▼──┐    ┌──▼──┐              │
                    │         │OpenAI│    │Claude│    │Gemini│ ← Provider  │
                    │         │Adapter│   │Adapter│  │Adapter│   Adapters  │
                    │         └─────┘    └─────┘    └─────┘              │
                    │                                                       │
                    │         ┌──────────────────────────────────────┐    │
                    │         │       Response Pipeline              │    │
                    │         │  Semantic Cache → Cost Record → Log  │    │
                    │         └──────────────────────────────────────┘    │
                    │                                                       │
                    │    ┌──────────────────────────────────────────┐     │
                    │    │          Management Layer                 │     │
                    │    │  Web UI ←→ Admin API ←→ Config Watch     │     │
                    │    │  Prometheus Metrics ←→ Redis Cluster      │     │
                    │    └──────────────────────────────────────────┘     │
                    └──────────────────────────────────────────────────────┘
```

---

## 关键设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| HTTP 框架 | **chi** | 轻量、stdlib 兼容、中间件链原生支持 |
| 配置格式 | **YAML** | 人工可读写，K8s ConfigMap 原生支持 |
| 限流算法 | **Token Bucket** | 简单成熟，支持突发 |
| 健康检查 | **TCP Dial** + 可选 HTTP probe | 轻量无侵入 |
| 日志 | **slog** (Go 1.21+ stdlib) | 零依赖，结构化，和 OTel 生态兼容 |
| 指标 | **Prometheus Go client** | 行业标准 |
| API 兼容 | **OpenAI 格式**作为内部统一格式 | 生态最广，迁移成本最低 |
| 语义路由 | **Embedding 余弦相似度** | 无需训练，启动即用 |
| PII 脱敏 | **正则 + Hash** | 零外部依赖，可自托管 |
| 语义缓存 | **内存 + Redis 双存储** | 单实例低延迟，集群共享 |

---

## 项目目录结构

```
ai-gateway/
├── cmd/gateway/main.go              # 入口
├── internal/
│   ├── config/                      # 配置加载/热加载/持久化
│   │   ├── config.go                #   Config 结构定义
│   │   ├── watcher.go               #   文件监听热加载
│   │   └── persist.go               #   YAML 写入持久化 (Phase 4)
│   ├── server/                      # HTTP server + 路由注册
│   │   ├── server.go                #   主服务初始化 + 路由注册
│   │   ├── management.go            #   Admin API 处理 (Phase 4 扩展)
│   │   └── middleware/
│   │       ├── auth.go              #   API Key 验证
│   │       ├── ratelimit.go         #   速率限制 (内存/Redis)
│   │       ├── security.go          #   Prompt 注入 + PII (Phase 3)
│   │       ├── cost.go              #   成本控制中间件 (Phase 3)
│   │       ├── audit.go             #   请求/响应审计日志
│   │       ├── cors.go              #   CORS 跨域 (Phase 4)
│   │       ├── recovery.go          #   Panic 恢复
│   │       └── tracing.go           #   OpenTelemetry
│   ├── router/                      # 路由匹配引擎
│   │   └── matcher.go               #   glob 匹配 + 优先级排序
│   ├── balancer/                    # 负载均衡策略
│   │   └── balancer.go              #   加权轮询 + 健康检查
│   ├── provider/                    # 提供商适配器
│   │   ├── provider.go              #   统一接口定义
│   │   ├── openai/adapter.go
│   │   ├── anthropic/adapter.go
│   │   ├── google/adapter.go
│   │   └── azure/adapter.go
│   ├── cache/                       # 缓存接口 (Redis + 内存)
│   │   └── redis.go
│   ├── semantic/                    # 语义路由/缓存 (Phase 3)
│   │   ├── semantic.go              #   配置 + 余弦相似度
│   │   ├── router.go                #   语义路由匹配
│   │   ├── cache.go                 #   语义缓存存储
│   │   └── embedding.go             #   Embedding 客户端
│   ├── security/                    # 安全模块 (Phase 3)
│   │   ├── config.go                #   配置 + 接口定义
│   │   ├── prompt.go                #   注入检测规则引擎
│   │   └── pii.go                   #   PII 脱敏实现
│   ├── cost/                        # 成本控制 (Phase 3)
│   │   ├── config.go                #   配额配置
│   │   └── tracker.go               #   用量追踪
│   └── metrics/                     # Prometheus + OTel
├── ui/src/                          # React Web 管理界面
│   ├── pages/                       #   页面组件
│   │   ├── Dashboard.tsx            #   仪表盘总览
│   │   ├── Upstreams.tsx            #   上游管理 (CRUD)
│   │   ├── Routes.tsx               #   路由管理 (含 Fallback)
│   │   ├── Keys.tsx                 #   API Key 管理 (Phase 4)
│   │   ├── Settings.tsx             #   网关设置 (Phase 4)
│   │   └── Playground.tsx           #   Chat 测试
│   └── components/                  #   通用组件
├── configs/                         # 示例配置文件
├── deploy/                          # K8s 部署清单
├── docs/
│   └── ARCHITECTURE.md              # 本文档
├── Makefile
├── Dockerfile
└── README.md
```

---

## 测试策略

- **单元测试** — 每个内部包隔离测试（router 匹配逻辑、balancer 策略、rate limiter 计算）
- **集成测试** — 单进程拉起测试 server，用真实 HTTP 客户端发送请求到 mock 上游
- **端到端测试** — Docker Compose 模式：gateway + redis + mock provider
- **契约测试** — 验证 OpenAI 兼容接口的请求/响应格式正确性

### 测试用例范围

1. 路由匹配：精确匹配、glob 通配、优先级排序、无匹配时的默认行为
2. 负载均衡：加权分布比例、删除上游后的重新分布、健康检查摘除
3. 安全层：无效 API Key 拒绝、超限返回 429、配额耗尽返回 403
4. 提供商适配：流式/非流式响应正确透传、错误码映射
5. 并发安全：高并发下限流计数器正确、balancer 无竞态

---

## 假设与默认选择

- K8s 部署作为一等公民，但也提供 `docker run` 一键启动方式
- 限流基于内存（单实例可用），并通过 Redis 实现分布式（Phase 2）
- 统一内部格式采用 OpenAI 的 chat completion 结构，其他提供商做格式转换
- 配置变更通过文件 watch 热加载，不依赖 K8s sidecar
- 安全层提供插件接口，外部检测服务可选接入（Phase 3）
- Phase 3 模块（语义路由/缓存）默认关闭，需显示配置启用
