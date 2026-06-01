# AI Gateway SDK

## Python

```bash
cd sdk/python
pip install -e .
```

```python
from ai_gateway import GatewayClient
client = GatewayClient("http://localhost:8080", api_key="sk-your-key")
print(client.health())
```

## Node.js

```javascript
const { GatewayClient } = require("./sdk/node");
const client = new GatewayClient("http://localhost:8080", "sk-your-key");
console.log(await client.health());
```
