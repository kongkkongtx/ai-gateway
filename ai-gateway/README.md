# AI Gateway

A lightweight, high-performance AI gateway that sits between your applications and AI providers (OpenAI, Anthropic, Google, etc.), providing unified routing, load balancing, security, and observability.

## Features

- **OpenAI-compatible API** — Drop-in replacement for existing OpenAI SDKs, just change the base URL
- **Smart Routing** — Route requests by model name with glob patterns and priority matching
- **Load Balancing** — Weighted round-robin, least-connections, health checks
- **Security** — API key management, rate limiting, audit logging
- **Observability** — Prometheus metrics, structured logging, OpenTelemetry tracing
- **Kubernetes-native** — Ready for K8s deployment with ConfigMap-driven configuration

## Quick Start

```bash
# Build
make build

# Run with demo config
export OPENAI_API_KEY="sk-your-key"
./bin/ai-gateway -config configs/gateway.yaml

# Or use make
make run
```

## Configuration

See [configs/gateway.yaml](configs/gateway.yaml) for a complete example.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GATEWAY_SERVER_HOST` | Server host | `0.0.0.0` |
| `GATEWAY_SERVER_PORT` | Server port | `8080` |
| `GATEWAY_LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `GATEWAY_LOG_FORMAT` | Log format (text, json) | `text` |
| `GATEWAY_AUTH_ENABLED` | Enable API key authentication | `true` |

### Example: Using with OpenAI SDK

```python
import openai

openai.base_url = "http://localhost:8080/v1/"
openai.api_key = "sk-gateway-demo-key"

response = openai.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/chat/completions` | Chat completions (streaming + non-streaming) |
| POST | `/v1/embeddings` | Text embeddings |
| GET | `/admin/health` | Health check |
| GET | `/admin/status` | Gateway status (upstreams, routes) |
| GET | `/metrics` | Prometheus metrics |

## Development

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Format code
make fmt
```

## License

Apache 2.0