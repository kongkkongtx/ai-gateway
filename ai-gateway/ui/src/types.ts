export interface UpstreamStatus {
  name: string
  endpoint: string
  provider: string
  healthy: boolean
  weight: number
  active_conns: number
}

export interface RouteInfo {
  id: string
  model: string
  upstream: string
  priority: number
}

export interface GatewayStatus {
  status: string
  upstreams: UpstreamStatus[]
  routes: RouteInfo[]
}

export interface HealthResponse {
  status: string
  timestamp: string
}

export interface MetricsData {
  requestsPerMinute: number
  avgLatency: number
  totalTokens: number
  activeRequests: number
}

export interface ChatRequest {
  model: string
  messages: { role: string; content: string }[]
  stream?: boolean
  temperature?: number
  max_tokens?: number
  top_p?: number
  stop?: string[]
  presence_penalty?: number
  frequency_penalty?: number
}

export interface ChatResponse {
  id: string
  object: string
  created: number
  model: string
  choices: {
    index: number
    message: { role: string; content: string }
    finish_reason: string
  }[]
  usage: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
  }
}
