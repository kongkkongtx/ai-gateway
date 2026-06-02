# AI Gateway

**面向企业私有化部署的 AI 治理网关** — 统一接入多家大模型服务，在网关层实现安全防护、成本治理、审计追踪和可观测性。

[English](README.md) | 简体中文

---

## 为什么选择 AI Gateway？

AI Gateway 位于你的应用与 AI 提供商（OpenAI、Anthropic、DeepSeek、Google 等）之间，不仅提供统一的路由和负载均衡，更在企业关心的安全合规、成本控制和运维治理方面提供完整的解决方案。

### 四大核心能力

| 能力 | 说明 |
|------|------|
| 🔗 **统一接入** | OpenAI-compatible API，一行代码切换模型；支持 OpenAI / Claude / Gemini / Azure / 私有模型 |
| 🔒 **安全可控** | Prompt 注入检测、PII 脱敏、安全策略拦截，在网关层守护数据安全 |
| 💰 **成本治理** | Token 统计、预算限制、成本告警、模型降级、语义缓存，让 AI 支出透明可控 |
| 🏠 **私有部署** | Docker / Kubernetes 一键部署，日志、配置、密钥、业务数据由企业自行掌控 |

---

## 快速开始

```bash
# 编译
make build

# 运行
./bin/ai-gateway -config configs/gateway.yaml

# 或者直接用 make
make run
```

### 5 分钟接入

```bash
# 验证服务
curl http://localhost:8080/admin/health
# {"status":"ok"}

# 使用 OpenAI SDK 发起请求
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-gateway-demo-key" \
  -d '''{"model": "gpt-4", "messages": [{"role": "user", "content": "你好"}]}'''
```

### Python（OpenAI SDK）

```python
import openai

openai.base_url = "http://localhost:8080/v1/"
openai.api_key = "sk-gateway-demo-key"

response = openai.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "你好"}]
)
print(response.choices[0].message.content)
```

---

## 功能特性

### 核心能力
- **OpenAI 兼容接口** — 无需修改代码，只需将 SDK 的 base URL 指向网关
- **智能路由** — 按模型名称 glob 匹配 + 按语义内容 embedding 匹配
- **负载均衡** — 加权轮询、最少连接、自动健康检查与故障摘除
- **Provider Fallback** — 上游超时时自动切换到备用提供商
- **插件系统** — 自定义 Provider / Security / Router 插件，通过 HTTP 接口扩展

### 安全防护
- **API Key 管理** — 多密钥认证，支持角色权限
- **Prompt 注入检测** — 内置规则引擎 (角色劫持/越狱/数据窃取等 80+ 规则)
- **PII 脱敏** — 邮箱/电话/IP/API Key/SSN/信用卡自动识别与替换
- **速率限制** — Token Bucket 算法，支持内存和 Redis 分布式

### 成本控制
- **Token 配额** — 按 API Key 设置输入/输出 Token 的周期性上限
- **预算告警** — 用量达到阈值时自动通知
- **自动降级** — 超限后自动切换到更便宜的模型
- **语义缓存** — 相似请求命中缓存，减少重复 API 调用

### 可观测性
- **Prometheus 指标** — 请求量、延迟、Token 用量、上游健康状态
- **OpenTelemetry 追踪** — 全链路请求追踪
- **结构化审计日志** — 每次请求的完整记录
- **Webhook 通知** — 成本告警、安全事件、上游故障的实时推送
- **实时管理面板** — Dashboard / Upstreams / Routes / Keys / Settings / Prompts / Audit Logs

### 部署
- **Kubernetes 原生** — 支持 ConfigMap 配置挂载，一键部署到 K8s
- **单二进制** — 编译为独立可执行文件，无外部依赖
- **配置热加载** — 修改配置文件无需重启

---

## 模块成熟度

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

---

## 架构

```
                    ┌──────────────────────────────────────────┐
                    │              AI Gateway                    │
                    │                                            │
  应用 ──OpenAI──►  │  Auth → RateLimit → Security → Router     │
  兼容              │    → Balancer → Provider                   │
                    │    → Cost → Audit → Metrics                │
                    │                                            │
                    │  Web UI ←→ Admin API ←→ Config Watch      │
                    │  Plugin System (Provider/Security/Router)  │
                    └──────────────┬─────────────────────────────┘
                                   │
                        ┌──────────▼──────────┐
                        │  OpenAI / Anthropic  │
                        │  DeepSeek / Google   │
                        │  Azure / 私有模型     │
                        └─────────────────────┘
```

---

## 配置说明

完整的配置示例见 [configs/gateway.yaml](configs/gateway.yaml)。支持语义路由、语义缓存、成本控制、安全检测、Prompt 模版、Webhook、插件系统等高级配置，默认关闭。

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| GATEWAY_SERVER_HOST | 监听地址 | 0.0.0.0 |
| GATEWAY_SERVER_PORT | 监听端口 | 8080 |
| GATEWAY_LOG_LEVEL | 日志级别 (debug, info, warn, error) | info |
| GATEWAY_LOG_FORMAT | 日志格式 (text, json) | text |
| GATEWAY_AUTH_ENABLED | 启用 API Key 认证 | true |
| GATEWAY_REDIS_ADDR | Redis 地址 | — |
| GATEWAY_REDIS_PASSWORD | Redis 密码 | — |

---

## API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /v1/chat/completions | 聊天补全（支持流式和非流式） |
| POST | /v1/embeddings | 文本嵌入 |
| GET | /admin/health | 健康检查 |
| GET | /admin/status | 网关状态（上游、路由列表） |
| GET | /admin/upstreams | 上游列表 |
| POST | /admin/upstreams | 批量更新上游 |
| DELETE | /admin/upstreams/{name} | 删除上游 |
| GET | /admin/routes | 路由列表 |
| POST | /admin/routes | 批量更新路由 |
| DELETE | /admin/routes/{id} | 删除路由 |
| GET | /admin/keys | API Key 列表 |
| POST | /admin/keys | 添加 API Key |
| DELETE | /admin/keys/{key} | 删除 API Key |
| GET | /admin/config | 网关配置 |
| PUT | /admin/config | 更新网关配置 |
| GET | /admin/audit-logs | 审计日志查询 |
| GET | /admin/cost-stats | 成本统计 |
| GET | /admin/prompts | Prompt 模版列表 |
| POST | /admin/prompts | 创建/更新 Prompt 模版 |
| DELETE | /admin/prompts/{id} | 删除 Prompt 模版 |
| GET | /admin/webhook | Webhook 配置 |
| PUT | /admin/webhook | 更新 Webhook 配置 |
| GET | /admin/plugins | 插件列表 |
| POST | /admin/plugins | 重载插件配置 |
| GET | /metrics | Prometheus 指标 |
| GET | /openapi.yaml | OpenAPI 规范 |

---

## 产品路线图

| 版本 | 阶段 | 状态 | 关键交付 |
|------|------|------|---------|
| v1.0 | Phase 1 — MVP 核心骨架 | ✅ 已完成 | OpenAI 代理、路由、负载均衡、认证限流 |
| v1.5 | Phase 2 — 多提供商 | ✅ 已完成 | Anthropic/Google/Azure、Fallback、Redis、K8s |
| v2.0 | Phase 3 — 差异化竞争力 | ✅ 已完成 | 语义路由、安全检测、PII 脱敏、语义缓存、成本控制、Web UI、Prompt 模版、Webhook、插件系统、SDK |
| v2.1 | 生产可信版本 | 🎯 当前开发 | 管线集成、流式兼容、成本补全、审计持久化、配置回滚、Docker Compose、集成测试 |
| v2.2 | 企业治理版本 | 📋 规划中 | 多用户/RBAC、团队成本、API Key 生命周期、安全策略中心、审计导出、SSO |
| v3.0 | 智能优化版本 | 📋 远期规划 | A/B 测试、模型评估、多模态、RAG、MCP |

查看完整路线图：[ROADMAP.md](ROADMAP.md) | 中期审查：[MIDTERM_REVIEW_RECOMMENDATIONS.md](docs/MIDTERM_REVIEW_RECOMMENDATIONS.md)

---

## 开发

```bash
# 运行测试
make test

# 生成覆盖率报告
make test-coverage

# 格式化代码
make fmt

# 代码检查
make lint

# Docker 构建
make docker-build
```

---

## 开源协议

Apache 2.0
