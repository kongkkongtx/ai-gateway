# AI Gateway Quick Start

5 分钟快速上手指南。

## 1. 启动网关

### Docker Compose（推荐）

```bash
docker-compose up -d
```

### 直接运行

```bash
make build
./bin/ai-gateway -config configs/gateway.yaml
```

## 2. 验证服务

```bash
curl http://localhost:8080/admin/health
# {"status":"ok"}
```

## 3. 发送第一个请求

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-gateway-demo-key" \
  -d '{"model":"deepseek-chat","messages":[{"role":"user","content":"你好"}]}'
```

## 4. 使用 OpenAI SDK

```python
import openai

openai.base_url = "http://localhost:8080/v1/"
openai.api_key = "sk-gateway-demo-key"

response = openai.chat.completions.create(
    model="deepseek-chat",
    messages=[{"role":"user","content":"你好"}]
)
print(response.choices[0].message.content)
```

## 5. 查看管理后台

浏览器打开 http://localhost:8080

## 下一步

- 查看完整配置：[configs/gateway.yaml](configs/gateway.yaml)
- 查看 API 文档：[docs/openapi.yaml](docs/openapi.yaml)
- 查看路线图：[ROADMAP.md](ROADMAP.md)
