import type { GatewayStatus, HealthResponse, ChatRequest, ChatResponse, UpstreamStatus } from '../types'

const BASE = ''

async function fetchJson<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${url}`, {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`HTTP ${res.status}: ${body}`)
  }
  return res.json()
}

export async function getHealth(): Promise<HealthResponse> {
  return fetchJson<HealthResponse>('/admin/health')
}

export async function getStatus(): Promise<GatewayStatus> {
  return fetchJson<GatewayStatus>('/admin/status')
}

export async function postChatCompletion(req: ChatRequest, apiKey?: string, signal?: AbortSignal): Promise<ChatResponse> {
  return fetchJson<ChatResponse>('/v1/chat/completions', {
    method: 'POST',
    body: JSON.stringify(req),
    headers: apiKey ? { 'Authorization': `Bearer ${apiKey}` } : undefined,
    signal,
  })
}

export async function streamChatCompletion(
  req: ChatRequest,
  apiKey: string | undefined,
  onChunk: (text: string) => void,
  onDone: () => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(`${BASE}/v1/chat/completions`, {
    method: 'POST',
    body: JSON.stringify({ ...req, stream: true }),
    headers: {
      'Content-Type': 'application/json',
      ...(apiKey ? { 'Authorization': `Bearer ${apiKey}` } : {}),
    },
    signal,
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)

  const reader = res.body?.getReader()
  if (!reader) throw new Error('No response body')

  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() ?? ''

    for (const line of lines) {
      if (line.startsWith('data: ')) {
        const data = line.slice(6).trim()
        if (data === '[DONE]') { onDone(); return }
        try {
          const parsed = JSON.parse(data)
          const content = parsed.choices?.[0]?.delta?.content ?? ''
          if (content) onChunk(content)
        } catch { /* skip malformed chunks */ }
      }
    }
  }
  onDone()
}

// Management API types
export interface UpstreamConfig {
  name: string
  endpoint: string
  provider: string
  weight: number
  api_token?: string
  timeout?: string
  model?: string
}

export interface RouteConfig {
  id: string
  model: string
  upstream: string
  fallbacks?: string[]
  priority: number
}

export interface ApiKeyConfig {
  key: string
  name: string
}

// Management API calls
export async function getUpstreams(): Promise<UpstreamStatus[]> {
  return fetchJson('/admin/upstreams')
}

export async function reloadUpstreams(upstreams: UpstreamConfig[]): Promise<{ status: string; count: number }> {
  return fetchJson('/admin/upstreams', {
    method: 'POST',
    body: JSON.stringify(upstreams),
  })
}

export async function getAdminRoutes(): Promise<RouteConfig[]> {
  return fetchJson('/admin/routes')
}

export async function reloadRoutes(routes: RouteConfig[]): Promise<{ status: string; count: number }> {
  return fetchJson('/admin/routes', {
    method: 'POST',
    body: JSON.stringify(routes),
  })
}

export async function getApiKeys(): Promise<ApiKeyConfig[]> {
  return fetchJson('/admin/keys')
}

export async function addApiKey(key: string, name: string, roles?: string[]): Promise<{ status: string }> {
  return fetchJson('/admin/keys', {
    method: 'POST',
    body: JSON.stringify({ key, name, roles }),
  })
}

export async function deleteApiKey(key: string): Promise<{ status: string }> {
  return fetchJson('/admin/keys/' + encodeURIComponent(key), {
    method: 'DELETE',
  })
}

export async function addSingleUpstream(upstream: UpstreamConfig): Promise<{ status: string }> {
  return fetchJson('/admin/upstreams/single', {
    method: 'POST',
    body: JSON.stringify(upstream),
  })
}

export async function deleteUpstream(name: string): Promise<{ status: string }> {
  return fetchJson('/admin/upstreams/' + encodeURIComponent(name), {
    method: 'DELETE',
  })
}

export async function addSingleRoute(route: RouteConfig): Promise<{ status: string }> {
  return fetchJson('/admin/routes/single', {
    method: 'POST',
    body: JSON.stringify(route),
  })
}

export async function deleteRoute(id: string): Promise<{ status: string }> {
  return fetchJson('/admin/routes/' + encodeURIComponent(id), {
    method: 'DELETE',
  })
}

export interface GatewayConfig {
  rate_limit: { enabled: boolean; global?: { limit: number; window: string }; per_key?: { key: string; limit: number; window: string }[] }
  log: { level: string; format: string }
  redis: { addr: string; password: string; db: number }
  auth: { enabled: boolean }
  security?: {
    enabled?: boolean
    prompt_injection: { enabled: boolean; action: string; risk_threshold: string }
    pii: { enabled: boolean; action: string; types?: string[] }
  }
  semantic?: {
    enabled: boolean
    provider: string
    threshold: number
  }
  semantic_cache?: {
    enabled: boolean
    threshold: number
    ttl: string
    max_entries: number
  }
  cost?: {
    enabled: boolean
    budget?: number
    alert_threshold?: number
    model_priority?: string[]
    auto_degrade?: boolean
    default_limit?: { input_tokens: number; output_tokens: number; window: string }
    degrade?: { action: string; cheaper_provider?: string; alert_webhook?: string; alert_threshold?: number }
  }


}

export interface AuditLogEntry {
  timestamp: string
  method: string
  path: string
  status: number
  duration: string
  remote_addr: string
  api_key_name?: string
  level: string
  model?: string
  tokens_in?: number
  tokens_out?: number
}

export async function getAuditLogs(params: { limit?: number; level?: string; key_name?: string; path?: string; status?: number }): Promise<AuditLogEntry[]> {
  const qs = new URLSearchParams()
  if (params.limit) qs.set('limit', String(params.limit))
  if (params.level) qs.set('level', params.level)
  if (params.key_name) qs.set('key_name', params.key_name)
  if (params.path) qs.set('path', params.path)
  if (params.status) qs.set('status', String(params.status))
  return fetchJson('/admin/audit-logs?' + qs.toString())
}

export interface CostStat {
  key_name: string
  input_tokens: number
  output_tokens: number
  total_tokens: number
  record_count: number
}

export async function getCostStats(): Promise<CostStat[]> {
  return fetchJson('/admin/cost-stats')
}


export interface PromptTemplate {
  id: string
  name: string
  description?: string
  role?: string
  route_match?: string
  variables?: { name: string; default?: string; desc?: string }[]
  current_version: number
  versions?: {
    version: number
    content: string
    variables?: { name: string; default?: string; desc?: string }[]
    created_at: string
    created_by?: string
    comment?: string
  }[]
  created_at: string
  updated_at: string
}


export interface WebhookEndpoint {
  name: string
  url: string
  enabled: boolean
  events?: string[]
  retry?: number
}

export interface WebhookConfig {
  endpoints: WebhookEndpoint[]
}

export async function getWebhookConfig(): Promise<WebhookConfig> {
  return fetchJson('/admin/webhook')
}

export async function updateWebhookConfig(cfg: WebhookConfig): Promise<{ status: string }> {
  return fetchJson('/admin/webhook', {
    method: 'PUT',
    body: JSON.stringify(cfg),
  })
}

export async function getPrompts(): Promise<PromptTemplate[]> {
  return fetchJson('/admin/prompts')
}

export async function savePrompt(tmpl: Partial<PromptTemplate>): Promise<{ status: string; id: string }> {
  return fetchJson('/admin/prompts', {
    method: 'POST',
    body: JSON.stringify(tmpl),
  })
}

export async function deletePrompt(id: string): Promise<{ status: string }> {
  return fetchJson('/admin/prompts/' + encodeURIComponent(id), {
    method: 'DELETE',
  })
}

export async function addPromptVersion(id: string, content: string, comment?: string): Promise<{ status: string; version: string }> {
  return fetchJson('/admin/prompts/' + encodeURIComponent(id) + '/versions', {
    method: 'POST',
    body: JSON.stringify({ content, comment }),
  })
}

export async function getGatewayConfig(): Promise<GatewayConfig> {
  return fetchJson('/admin/config')
}

export async function updateGatewayConfig(config: Partial<GatewayConfig>): Promise<{ status: string }> {
  return fetchJson('/admin/config', {
    method: 'PUT',
    body: JSON.stringify(config),
  })
}


