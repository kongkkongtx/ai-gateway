# AI Gateway

一个轻量、高性能的 **企业级 AI 网关**，位于你的应用与 AI 提供商（OpenAI、Anthropic、DeepSeek、Google 等）之间，提供统一的路由、负载均衡、安全治理、成本控制和可观测性。

[English](README.md) | 简体中文

---

## 为什么需要 AI Gateway？

### 企业 AI 落地的六大痛点

| 痛点 | 问题 | AI Gateway 方案 |
|------|------|----------------|
| 🔒 **安全合规** | Prompt 注入攻击、PII 泄露到 AI 厂商、缺乏审计 | Prompt 检测 + PII 脱敏 + 全链路审计 |
| 💰 **成本失控** | 各部门各自接入 AI，用量不透明，月底账单不可控 | Token 配额管理 + 预算告警 + 自动降级 |
| 🔄 **多云锁定** | 单一 AI 提供商面临价格波动、服务不可用、政策风险 | 统一接口 + 多 Provider 适配 + 自动 Fallback |
| 🎯 **模型选型乱** | 不同场景需要不同模型，手动管理繁琐 | Glob 路由 + 语义路由 (embedding 匹配) |
| 🐌 **重复查询** | 相似问题反复请求，浪费 API 调用 | 语义缓存 (基于相似度匹配) |
| 🔧 **运维复杂** | 缺少统一的监控、告警、配置管理 | Prometheus 指标 + Web 管理控制台 + K8s 原生 |

---

## 功能特性

### 核心能力
- **OpenAI 兼容接口** — 无需修改代码，只需将 SDK 的 base URL 指向网关
- **智能路由** — 按模型名称 glob 匹配 + 按语义内容 embedding 匹配
- **负载均衡** — 加权轮询、最少连接、自动健康检查与故障摘除
- **Provider Fallback** — 上游超时时自动切换到备用提供商

### 安全防护
- **API Key 管理** — 多密钥认证，支持角色权限
- **Prompt 注入检测** — 内置规则引擎 (角色劫持/越狱/数据窃取等 80+ 规则)
- **PII 脱敏** — 邮箱/电话/IP/API Key/SSN/信用卡自动识别与替换
- **速率限制** — Token Bucket 算法，支持内存和 Redis 分布式

### 成本控制
- **Token 配额** — 按 API Key 设置输入/输出 Token 的周期性上限
- **预算告警** — 用量达到阈值时自动通知
- **自动降级** — 超限后自动切换到更便宜的模型

### 可观测性
- **Prometheus 指标** — 请求量、延迟、Token 用量、上游健康状态
- **OpenTelemetry 追踪** — 全链路请求追踪
- **结构化审计日志** — 每次请求的完整记录
- **实时管理面板** — Dashboard / Upstreams / Routes / Keys / Settings

### 部署
- **Kubernetes 原生** — 支持 ConfigMap 配置挂载，一键部署到 K8s
- **单二进制** — 编译为独立可执行文件，无外部依赖
- **配置热加载** — 修改配置文件无需重启

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

### 验证服务

```bash
curl http://localhost:8080/admin/health
# {"status":"ok"}

curl http://localhost:8080/admin/status
# 查看上游和路由的实时状态
```

---

## 配置说明

完整的配置示例见 [configs/gateway.yaml](configs/gateway.yaml)。支持语义路由、语义缓存、成本控制、安全检测等高级配置，默认关闭。

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| GATEWAY_SERVER_HOST | 监听地址 | 0.0.0.0 |
| GATEWAY_SERVER_PORT | 监听端口 | 8080 |
| GATEWAY_LOG_LEVEL | 日志级别 (debug, info, warn, error) | info |
| GATEWAY_LOG_FORMAT | 日志格式 (text, json) | text |
| GATEWAY_AUTH_ENABLED | 启用 API Key 认证 | true |

---

## 使用示例

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

### cURL

```bash
# 非流式请求
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-gateway-demo-key" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "你好"}]}'
```

---

## 产品路线图

| 版本 | 阶段 | 状态 | 关键交付 |
|------|------|------|---------|
| v1.0 | Phase 1 — MVP 核心骨架 | ✅ 已完成 | OpenAI 代理、路由、负载均衡、认证限流 |
| v1.5 | Phase 2 — 多提供商 | ✅ 已完成 | Anthropic/Google/Azure、Fallback、Redis、K8s |
| v2.0 | Phase 3 — 差异化竞争力 | ✅ 已完成 | 语义路由、安全检测、PII 脱敏、语义缓存、成本控制、Web UI |
| v2.1 | Phase 4 — 生产加固 | 🎯 当前开发 | 管线集成、配置编辑、审计面板、权限管控、Docker Compose |
| v2.2 | Phase 5 — 生态集成 | 📋 规划中 | Prompt 管理、Webhook、插件系统、SDK |
| v3.0 | Phase 6 — AI 中台 | 📋 规划中 | A/B 测试、模型评估、多模态、MCP 集成 |

查看完整路线图：[ROADMAP.md](ROADMAP.md)

---

## API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /v1/chat/completions | 聊天补全（支持流式和非流式） |
| POST | /v1/embeddings | 文本嵌入 |
| GET | /admin/health | 健康检查 |
| GET | /admin/status | 网关状态（上游、路由列表） |
| GET | /admin/keys | API Key 列表 |
| POST | /admin/keys | 添加 API Key |
| DELETE | /admin/keys/{key} | 删除 API Key |
| GET | /admin/upstreams | 上游列表 |
| POST | /admin/upstreams | 批量更新上游 |
| DELETE | /admin/upstreams/{name} | 删除上游 |
| GET | /admin/routes | 路由列表 |
| POST | /admin/routes | 批量更新路由 |
| DELETE | /admin/routes/{id} | 删除路由 |
| GET | /admin/config | 网关配置 |
| PUT | /admin/config | 更新网关配置 |
| GET | /metrics | Prometheus 指标 |

---

## 架构

```
                    ┌──────────────────────────────────────┐
                    │            AI Gateway                  │
                    │                                        │
  应用 ──OpenAI──►  │  Auth → RateLimit → Security → Router  │
  兼容              │           → Balancer → Provider        │
                    │           → Cost → Audit → Metrics     │
                    │                                        │
                    │  Web UI ←→ Admin API ←→ Config Watch   │
                    └──────────────┬─────────────────────────┘
                                   │
                        ┌──────────▼──────────┐
                        │  OpenAI / Anthropic  │
                        │  DeepSeek / Google   │
                        │  Azure / 更多        │
                        └─────────────────────┘
```

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
