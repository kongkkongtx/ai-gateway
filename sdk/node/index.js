const BASE = process.env.GATEWAY_URL || "http://localhost:8080";

class GatewayClient {
  constructor(baseUrl = BASE, apiKey = null) {
    this.baseUrl = baseUrl.replace(/\/$/, "");
    this.apiKey = apiKey;
  }

  async request(method, path, body = undefined, params = {}) {
    const url = new URL(this.baseUrl + path);
    Object.entries(params).filter(([_, v]) => v != null).forEach(([k, v]) => url.searchParams.set(k, v));

    const headers = { "Content-Type": "application/json" };
    if (this.apiKey) headers["Authorization"] = `Bearer ${this.apiKey}`;

    const res = await fetch(url, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });

    if (!res.ok) {
      const text = await res.text();
      throw new Error(`HTTP ${res.status}: ${text}`);
    }

    const text = await res.text();
    return text ? JSON.parse(text) : {};
  }

  health() { return this.request("GET", "/admin/health"); }
  status() { return this.request("GET", "/admin/status"); }
  listUpstreams() { return this.request("GET", "/admin/upstreams"); }
  addUpstream(config) { return this.request("POST", "/admin/upstreams/single", config); }
  deleteUpstream(name) { return this.request("DELETE", `/admin/upstreams/${encodeURIComponent(name)}`); }
  listRoutes() { return this.request("GET", "/admin/routes"); }
  addRoute(config) { return this.request("POST", "/admin/routes/single", config); }
  deleteRoute(id) { return this.request("DELETE", `/admin/routes/${encodeURIComponent(id)}`); }
  listKeys() { return this.request("GET", "/admin/keys"); }
  addKey(key, name, roles = ["admin"]) { return this.request("POST", "/admin/keys", { key, name, roles }); }
  deleteKey(key) { return this.request("DELETE", `/admin/keys/${encodeURIComponent(key)}`); }
  getConfig() { return this.request("GET", "/admin/config"); }
  updateConfig(config) { return this.request("PUT", "/admin/config", config); }
  queryAuditLogs(params = {}) { return this.request("GET", "/admin/audit-logs", undefined, params); }
  getCostStats() { return this.request("GET", "/admin/cost-stats"); }
  getWebhookConfig() { return this.request("GET", "/admin/webhook"); }
  updateWebhookConfig(config) { return this.request("PUT", "/admin/webhook", config); }
  listPrompts() { return this.request("GET", "/admin/prompts"); }
  savePrompt(template) { return this.request("POST", "/admin/prompts", template); }
  deletePrompt(id) { return this.request("DELETE", `/admin/prompts/${encodeURIComponent(id)}`); }
  addPromptVersion(id, content, comment) {
    return this.request("POST", `/admin/prompts/${encodeURIComponent(id)}/versions`, { content, comment });
  }
  chatCompletion(model, messages, options = {}) {
    return this.request("POST", "/v1/chat/completions", { model, messages, ...options });
  }
}

module.exports = { GatewayClient, default: GatewayClient };
