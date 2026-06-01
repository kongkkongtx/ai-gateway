"""
AI Gateway Python SDK
=====================
Python client for managing AI Gateway.

Usage:
    from ai_gateway import GatewayClient

    client = GatewayClient("http://localhost:8080", api_key="sk-your-key")
    print(client.health())
"""

import json
from typing import Any, Dict, List, Optional
from urllib.request import Request, urlopen
from urllib.error import HTTPError


class GatewayError(Exception):
    """Raised when the Gateway API returns an error."""
    def __init__(self, status: int, body: str):
        self.status = status
        self.body = body
        super().__init__(f"HTTP {status}: {body}")


class GatewayClient:
    """Client for AI Gateway Admin API."""

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        api_key: Optional[str] = None,
        timeout: int = 30,
    ):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout

    def _request(
        self,
        method: str,
        path: str,
        body: Optional[Dict] = None,
        params: Optional[Dict] = None,
    ) -> Any:
        url = f"{self.base_url}{path}"
        if params:
            qs = "&".join(f"{k}={v}" for k, v in params.items() if v is not None)
            url = f"{url}?{qs}"

        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        data = json.dumps(body).encode() if body else None
        req = Request(url, data=data, headers=headers, method=method)

        try:
            with urlopen(req, timeout=self.timeout) as resp:
                raw = resp.read().decode()
                if raw:
                    return json.loads(raw)
                return {}
        except HTTPError as e:
            raise GatewayError(e.code, e.read().decode())

    # === System ===
    def health(self) -> Dict:
        return self._request("GET", "/admin/health")

    def status(self) -> Dict:
        return self._request("GET", "/admin/status")

    # === Upstreams ===
    def list_upstreams(self) -> List[Dict]:
        return self._request("GET", "/admin/upstreams")

    def add_upstream(self, config: Dict) -> Dict:
        return self._request("POST", "/admin/upstreams/single", config)

    def delete_upstream(self, name: str) -> Dict:
        return self._request("DELETE", f"/admin/upstreams/{name}")

    def reload_upstreams(self, upstreams: List[Dict]) -> Dict:
        return self._request("POST", "/admin/upstreams", upstreams)

    # === Routes ===
    def list_routes(self) -> List[Dict]:
        return self._request("GET", "/admin/routes")

    def add_route(self, config: Dict) -> Dict:
        return self._request("POST", "/admin/routes/single", config)

    def delete_route(self, route_id: str) -> Dict:
        return self._request("DELETE", f"/admin/routes/{route_id}")

    def reload_routes(self, routes: List[Dict]) -> Dict:
        return self._request("POST", "/admin/routes", routes)

    # === API Keys ===
    def list_keys(self) -> List[Dict]:
        return self._request("GET", "/admin/keys")

    def add_key(self, key: str, name: str, roles: Optional[List[str]] = None) -> Dict:
        return self._request("POST", "/admin/keys", {"key": key, "name": name, "roles": roles or ["admin"]})

    def delete_key(self, key: str) -> Dict:
        return self._request("DELETE", f"/admin/keys/{key}")

    # === Config ===
    def get_config(self) -> Dict:
        return self._request("GET", "/admin/config")

    def update_config(self, config: Dict) -> Dict:
        return self._request("PUT", "/admin/config", config)

    # === Observability ===
    def query_audit_logs(self, limit: int = 100, level: Optional[str] = None, key_name: Optional[str] = None, path: Optional[str] = None, status: Optional[int] = None) -> List[Dict]:
        params = {"limit": limit, "level": level, "key_name": key_name, "path": path, "status": status}
        return self._request("GET", "/admin/audit-logs", params={k: v for k, v in params.items() if v is not None})

    def get_cost_stats(self) -> List[Dict]:
        return self._request("GET", "/admin/cost-stats")

    # === Webhooks ===
    def get_webhook_config(self) -> Dict:
        return self._request("GET", "/admin/webhook")

    def update_webhook_config(self, config: Dict) -> Dict:
        return self._request("PUT", "/admin/webhook", config)

    # === Prompt Templates ===
    def list_prompts(self) -> List[Dict]:
        return self._request("GET", "/admin/prompts")

    def save_prompt(self, template: Dict) -> Dict:
        return self._request("POST", "/admin/prompts", template)

    def get_prompt(self, template_id: str) -> Dict:
        return self._request("GET", f"/admin/prompts/{template_id}")

    def delete_prompt(self, template_id: str) -> Dict:
        return self._request("DELETE", f"/admin/prompts/{template_id}")

    def add_prompt_version(self, template_id: str, content: str, comment: Optional[str] = None) -> Dict:
        return self._request("POST", f"/admin/prompts/{template_id}/versions", {"content": content, "comment": comment})

    # === Chat ===
    def chat_completion(self, model: str, messages: List[Dict], stream: bool = False, **kwargs) -> Dict:
        return self._request("POST", "/v1/chat/completions", {"model": model, "messages": messages, "stream": stream, **kwargs})
