# AI Gateway

**Enterprise AI Governance Gateway for Private Deployment** — Unified access to multiple LLM providers with built-in security, cost governance, audit trails, and observability at the gateway layer.

[English](README.md) | [简体中文](README.zh.md)

---

## Why AI Gateway?

AI Gateway sits between your application and AI providers (OpenAI, Anthropic, DeepSeek, Google, etc.). It goes beyond simple proxying and load balancing to provide the security, cost control, and operational governance that enterprises need.

### Four Core Capabilities

| Capability | Description |
|------|------|
| 🔗 **Unified Access** | OpenAI-compatible API — switch models with one line of code. Supports OpenAI / Claude / Gemini / Azure / self-hosted models |
| 🔒 **Security & Compliance** | Prompt injection detection, PII sanitization, security policy enforcement at the gateway layer |
| 💰 **Cost Governance** | Token tracking, budget limits, cost alerts, model fallback, semantic caching — make AI spending transparent and controllable |
| 🏠 **Private Deployment** | Docker / Kubernetes one-click deployment. Logs, configs, keys, and business data stay under your control |

---

## Quick Start

```bash
# Build
make build

# Run
./bin/ai-gateway -config configs/gateway.yaml

# Or use make
make run
```

### 5-Minute Integration

```bash
# Health check
curl http://localhost:8080/admin/health
# {"status":"ok"}

# Chat completion via OpenAI SDK
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-gateway-demo-key" \
  -d '''{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}'''
```

### Python (OpenAI SDK)

```python
import openai

openai.base_url = "http://localhost:8080/v1/"
openai.api_key = "sk-gateway-demo-key"

response = openai.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello"}]
)
print(response.choices[0].message.content)
```

---

## Features

### Core
- **OpenAI-Compatible API** — Point your SDK base URL to the gateway, no code changes needed
- **Smart Routing** — Model name glob matching + semantic embedding-based routing
- **Load Balancing** — Weighted round-robin, least connections, automatic health checks and failover
- **Provider Fallback** — Automatic switch to backup providers on upstream timeout
- **Plugin System** — Custom Provider / Security / Router plugins via HTTP interfaces

### Security
- **API Key Management** — Multi-key authentication with role-based permissions
- **Prompt Injection Detection** — Built-in rule engine (80+ rules: role hijacking, jailbreaking, data exfiltration)
- **PII Sanitization** — Auto-detect and mask email/phone/IP/API Key/SSN/credit card
- **Rate Limiting** — Token Bucket algorithm, in-memory and Redis distributed modes

### Cost Control
- **Token Quotas** — Per-key periodic limits on input/output tokens
- **Budget Alerts** — Automatic notifications when usage hits thresholds
- **Auto Degradation** — Switch to cheaper models when limits are exceeded
- **Semantic Cache** — Cache similar queries to reduce duplicate API calls

### Observability
- **Prometheus Metrics** — Request volume, latency, token usage, upstream health
- **OpenTelemetry Tracing** — End-to-end request tracing
- **Structured Audit Logs** — Complete records for every request
- **Webhook Notifications** — Real-time push for cost alerts, security events, upstream failures
- **Admin Dashboard** — Dashboard / Upstreams / Routes / Keys / Settings / Prompts / Audit Logs

### Deployment
- **Kubernetes Native** — ConfigMap-based configuration, one-click K8s deployment
- **Single Binary** — Compiles to a standalone executable, zero external dependencies
- **Hot Reload** — Update configuration without restarting

---

## Module Maturity

| Module | Implemented | Integrated | Production Verified |
|---|---:|---:|---:|
| OpenAI-compatible Proxy | ✅ | ✅ | ✅ |
| Multi-provider Routing | ✅ | ✅ | Needs load testing |
| Load Balancer + Health Check | ✅ | ✅ | Needs load testing |
| API Key Auth + Rate Limit | ✅ | ✅ | ✅ |
| Prompt Injection Detection | ✅ | ✅ | Needs verification |
| PII Sanitization | ✅ | ✅ | Needs verification |
| Semantic Router | ✅ | ✅ | Needs verification |
| Semantic Cache | ✅ | ✅ | Needs verification |
| Cost Control | ✅ | ✅ | Needs verification |
| Audit Logs | ✅ | ✅ | Needs persistence |
| Prompt Templates | ✅ | ✅ | Needs verification |
| Webhook Notifications | ✅ | ✅ | Needs verification |
| Plugin System | ✅ | ✅ | Needs verification |
| Admin UI | ✅ | ✅ | Needs refinement |
| Python / Node SDK | ✅ | ✅ | Needs refinement |

---

## Architecture

```
                    ┌──────────────────────────────────────────┐
                    │              AI Gateway                    │
                    │                                            │
  App ──OpenAI──►   │  Auth → RateLimit → Security → Router     │
  compatible        │    → Balancer → Provider                   │
                    │    → Cost → Audit → Metrics                │
                    │                                            │
                    │  Web UI ←→ Admin API ←→ Config Watch      │
                    │  Plugin System (Provider/Security/Router)  │
                    └──────────────┬─────────────────────────────┘
                                   │
                        ┌──────────▼──────────┐
                        │  OpenAI / Anthropic  │
                        │  DeepSeek / Google   │
                        │  Azure / Self-Hosted │
                        └─────────────────────┘
```

---

## Configuration

See [configs/gateway.yaml](configs/gateway.yaml) for a complete example. Supports semantic routing, semantic cache, cost control, security detection, prompt templates, webhooks, and plugin system (disabled by default).

### Environment Variables

| Variable | Description | Default |
|------|------|--------|
| GATEWAY_SERVER_HOST | Listen address | 0.0.0.0 |
| GATEWAY_SERVER_PORT | Listen port | 8080 |
| GATEWAY_LOG_LEVEL | Log level (debug, info, warn, error) | info |
| GATEWAY_LOG_FORMAT | Log format (text, json) | text |
| GATEWAY_AUTH_ENABLED | Enable API key auth | true |
| GATEWAY_REDIS_ADDR | Redis address | — |
| GATEWAY_REDIS_PASSWORD | Redis password | — |

---

## API Endpoints

| Method | Path | Description |
|------|------|------|
| POST | /v1/chat/completions | Chat completion (streaming and non-streaming) |
| POST | /v1/embeddings | Text embeddings |
| GET | /admin/health | Health check |
| GET | /admin/status | Gateway status (upstreams, routes) |
| GET | /admin/upstreams | List upstreams |
| POST | /admin/upstreams | Batch update upstreams |
| DELETE | /admin/upstreams/{name} | Delete upstream |
| GET | /admin/routes | List routes |
| POST | /admin/routes | Batch update routes |
| DELETE | /admin/routes/{id} | Delete route |
| GET | /admin/keys | List API keys |
| POST | /admin/keys | Add API key |
| DELETE | /admin/keys/{key} | Delete API key |
| GET | /admin/config | Gateway config |
| PUT | /admin/config | Update gateway config |
| GET | /admin/audit-logs | Query audit logs |
| GET | /admin/cost-stats | Cost statistics |
| GET | /admin/prompts | List prompt templates |
| POST | /admin/prompts | Create/update prompt template |
| DELETE | /admin/prompts/{id} | Delete prompt template |
| GET | /admin/webhook | Webhook config |
| PUT | /admin/webhook | Update webhook config |
| GET | /admin/plugins | List plugins |
| POST | /admin/plugins | Reload plugin config |
| GET | /metrics | Prometheus metrics |
| GET | /openapi.yaml | OpenAPI specification |

---

## Roadmap

| Version | Phase | Status | Key Deliverables |
|------|------|------|---------|
| v1.0 | Phase 1 — MVP Skeleton | ✅ Complete | OpenAI proxy, routing, load balancing, auth/rate limiting |
| v1.5 | Phase 2 — Multi-Provider | ✅ Complete | Anthropic/Google/Azure, Fallback, Redis, K8s |
| v2.0 | Phase 3 — Differentiation | ✅ Complete | Semantic routing, security, PII, semantic cache, cost control, Web UI, prompts, webhooks, plugins, SDK |
| v2.1 | Production Hardening | 🎯 In Development | Pipeline integration, streaming compat, cost completion, audit persistence, config rollback, Docker Compose, tests |
| v2.2 | Enterprise Governance | 📋 Planned | Multi-user/RBAC, team cost, API key lifecycle, security hub, audit export, SSO |
| v3.0 | AI Optimization | 📋 Future | A/B testing, model evaluation, multimodal, RAG, MCP |

Full roadmap: [ROADMAP.md](ROADMAP.md) | Midterm review: [MIDTERM_REVIEW_RECOMMENDATIONS.md](docs/MIDTERM_REVIEW_RECOMMENDATIONS.md)

---

## Development

```bash
# Run tests
make test

# Generate coverage report
make test-coverage

# Format code
make fmt

# Lint
make lint

# Docker build
make docker-build
```

---

## License

Apache 2.0
