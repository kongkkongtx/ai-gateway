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
