import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Activity, Network, RefreshCw, AlertTriangle, CheckCircle2,
  Server, TrendingUp, Clock, Zap, Route,
} from 'lucide-react'
import {
  AreaChart, Area, BarChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip, ResponsiveContainer, Legend,
} from "recharts"
import { getHealth, getStatus, getCostStats, getTeamCostStats } from '../api/gateway'
import type { CostStat, TeamStat } from '../api/gateway'
import StatCard from '../components/StatCard'
import StatusBadge from '../components/StatusBadge'
import type { GatewayStatus } from '../types'

interface DataPoint {
  time: string
  requests: number
  latency: number
}

const MAX_POINTS = 30

export default function Dashboard() {
  const { t } = useTranslation()
  const [status, setStatus] = useState<GatewayStatus | null>(null)
  const [health, setHealth] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [refreshing, setRefreshing] = useState(false)
  const [costStats, setCostStats] = useState<CostStat[]>([]); const [teamStats, setTeamStats] = useState<TeamStat[]>([])
  const [chartData, setChartData] = useState<DataPoint[]>([])

  const fetchData = useCallback(async (silent = false) => {
    if (!silent) setLoading(true)
    setError(null)
    try {
      const [h, s] = await Promise.all([getHealth(), getStatus()])
      setHealth(h.status)
      setStatus(s)

      try {
        const cs = await getCostStats()
        setCostStats(cs)
      } catch { /* cost stats unavailable */ }
      try { const ts = await getTeamCostStats(); setTeamStats(ts) } catch { /* team stats unavailable */ }

      const now = new Date().toLocaleTimeString()
      const totalConns = s.upstreams.reduce((sum, u) => sum + (u.active_conns ?? 0), 0)
      setChartData((prev) => {
        const next = [...prev, { time: now, requests: totalConns, latency: s.upstreams.filter(u => u.healthy).length }]
        if (next.length > MAX_POINTS) next.shift()
        return next
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
    const t = setInterval(() => fetchData(true), 5000)
    return () => clearInterval(t)
  }, [fetchData])

  const healthyCount = status?.upstreams.filter((u) => u.healthy).length ?? 0
  const totalUpstreams = status?.upstreams.length ?? 0
  const totalRoutes = status?.routes.length ?? 0
  const isHealthy = health === 'ok'
  const totalTokens = costStats.reduce((s, c) => s + c.total_tokens, 0)

  // Prepare cost data for charts
  const costChartData = costStats.map((c) => ({
    name: c.key_name,
    input: c.input_tokens,
    output: c.output_tokens,
  })).slice(0, 10)

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">{t('dashboard.title', 'Dashboard')}</h1>
          <p className="text-sm text-surface-400 mt-1">{t('dashboard.subtitle', 'Real-time AI Gateway monitoring')}</p>
        </div>
        <div className="flex items-center gap-3">
          {loading && <div className="text-xs text-surface-500 animate-pulse">{t('dashboard.refreshing')}</div>}
          <button onClick={() => { setRefreshing(true); fetchData(true) }} disabled={refreshing} className="btn-ghost flex items-center gap-2">
            <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} />
            {t('common.refresh')}
          </button>
        </div>
      </div>

      {/* Error */}
      {error && (
        <div className="bg-red-900/20 border border-red-800/40 rounded-xl p-4 flex items-center gap-3">
          <AlertTriangle size={18} className="text-red-400 shrink-0" />
          <div>
            <p className="text-sm font-medium text-red-300">{t('dashboard.connection_error')}</p>
            <p className="text-xs text-red-400/80 mt-0.5">{error}</p>
          </div>
        </div>
      )}

      {/* Health Banner */}
      {!error && !loading && (
        <div className={`rounded-xl p-4 flex items-center gap-3 border ${isHealthy ? 'bg-accent-900/20 border-accent-800/40' : 'bg-yellow-900/20 border-yellow-800/40'}`}>
          {isHealthy ? <CheckCircle2 size={20} className="text-accent-400 shrink-0" /> : <AlertTriangle size={20} className="text-yellow-400 shrink-0" />}
          <div>
            <p className="text-sm font-medium text-white">{isHealthy ? t('dashboard.all_ok', 'All systems operational') : t('dashboard.degraded', 'Service degraded')}</p>
            <p className="text-xs text-surface-400 mt-0.5">{t('dashboard.healthy_upstreams', { healthy: healthyCount, total: totalUpstreams })}</p>
          </div>
        </div>
      )}

      {/* Stat Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard icon={<Activity size={18} />} label={t('dashboard.total_requests')} value={chartData.length > 0 ? chartData[chartData.length - 1].requests : 0} color="brand" />
        <StatCard icon={<Network size={18} />} label={t('dashboard.active_upstreams')} value={`${healthyCount}/${totalUpstreams}`} color={healthyCount === totalUpstreams ? 'accent' : 'warning'} />
        <StatCard icon={<Zap size={18} />} label={t('dashboard.tokens_used')} value={totalTokens > 1000 ? `${(totalTokens / 1000).toFixed(1)}K` : totalTokens} color="purple" />
        <StatCard icon={<Route size={18} />} label={t('dashboard.active_routes')} value={totalRoutes} color="cyan" />
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Request Activity Chart */}
        <div className="card">
          <h2 className="card-header flex items-center gap-2">
            <Activity size={16} className="text-brand-400" />
            {t("dashboard.request_activity")}
          </h2>
          <div className="h-64">
            {chartData.length < 2 ? (
              <div className="h-full flex items-center justify-center text-sm text-surface-500">
                <Clock size={16} className="mr-2" /> {t("dashboard.collecting_data")}
              </div>
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={chartData}>
                  <defs>
                    <linearGradient id="colorRequests" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#818cf8" stopOpacity={0.3} />
                      <stop offset="95%" stopColor="#818cf8" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
                  <XAxis dataKey="time" tick={{ fontSize: 10, fill: '#64748b' }} />
                  <YAxis tick={{ fontSize: 10, fill: '#64748b' }} />
                  <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #334155', borderRadius: '8px', fontSize: '12px' }} />
                  <Area type="monotone" dataKey="requests" stroke="#818cf8" fill="url(#colorRequests)" strokeWidth={2} name="Active Conns" />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </div>
        </div>

        {/* Token Usage Chart */}
        <div className="card">
          <h2 className="card-header flex items-center gap-2">
            <TrendingUp size={16} className="text-accent-400" />
            {t("dashboard.token_usage_24h")}
          </h2>
          <div className="h-64">
            {costChartData.length === 0 ? (
              <div className="h-full flex items-center justify-center text-sm text-surface-500">
                <TrendingUp size={16} className="mr-2" /> {t("dashboard.no_usage_data")}
              </div>
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={costChartData} layout="vertical">
                  <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
                  <XAxis type="number" tick={{ fontSize: 10, fill: '#64748b' }} />
                  <YAxis type="category" dataKey="name" width={100} tick={{ fontSize: 10, fill: '#64748b' }} />
                  <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #334155', borderRadius: '8px', fontSize: '12px' }} />
                  <Legend />
                  <Bar dataKey="input" stackId="a" fill="#818cf8" name="Input Tokens" radius={[0, 0, 4, 4]} />
                  <Bar dataKey="output" stackId="a" fill="#34d399" name="Output Tokens" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </div>
        </div>
      </div>

      {/* Upstreams & Routes */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Upstreams */}
        <div className="card">
          <h2 className="card-header flex items-center justify-between">
            <span className="flex items-center gap-2"><Server size={16} /> {t("dashboard.upstreams_title")}</span>
            <span className="text-surface-500 font-normal text-xs">{t("dashboard.healthy_upstreams", { healthy: healthyCount, total: totalUpstreams })}</span>
          </h2>
          <div className="space-y-2">
            {loading ? Array.from({ length: 3 }).map((_, i) => (<div key={i} className="h-10 bg-surface-800 rounded-lg animate-pulse" />))
            : !status?.upstreams.length ? <p className="text-sm text-surface-500 py-4 text-center">{t('upstreams.no_upstreams')}</p>
            : status?.upstreams.map((u) => (
              <div key={u.name} className="flex items-center justify-between py-2.5 px-4 rounded-lg bg-surface-950/50 border border-surface-800/50 hover:border-surface-700 transition-colors">
                <div className="flex items-center gap-3 min-w-0">
                  <StatusBadge status={u.healthy ? 'healthy' : 'unhealthy'} pulse />
                  <div>
                    <div className="text-sm font-medium text-white">{u.name}</div>
                    <div className="text-xs text-surface-500 font-mono truncate max-w-[200px]">{u.endpoint}</div>
                  </div>
                </div>
                <div className="flex items-center gap-4 text-xs text-surface-400">
                  <span className="flex items-center gap-1"><Zap size={12} className="text-yellow-500" />W:{u.weight}</span>
                  <span>{u.provider}</span>
                </div>
              </div>
            ))
            }
          </div>
        </div>

        {/* Routes */}
        <div className="card">
          <h2 className="card-header flex items-center justify-between">
            <span>{t('routes.title')}</span>
            <span className="text-surface-500 font-normal text-xs">{t("dashboard.routes_count", { count: totalRoutes })}</span>
          </h2>
          <div className="space-y-2">
            {loading ? Array.from({ length: 3 }).map((_, i) => (<div key={i} className="h-10 bg-surface-800 rounded-lg animate-pulse" />))
            : !status?.routes.length ? <p className="text-sm text-surface-500 py-4 text-center">{t('routes.no_routes')}</p>
            : status?.routes.map((r) => (
              <div key={r.id} className="flex items-center justify-between py-2 px-4 rounded-lg bg-surface-950/30 hover:bg-surface-950/60 transition-colors">
                <div className="flex items-center gap-3">
                  <div className="w-6 h-6 rounded bg-brand-600/10 border border-brand-600/20 flex items-center justify-center">
                    <Route size={12} className="text-brand-400" />
                  </div>
                  <div>
                    <span className="text-sm font-medium text-white">{r.model}</span>
                    <span className="text-xs text-surface-500 ml-2">&rarr; {r.upstream}</span>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <span className="badge-neutral text-[10px]">P{r.priority}</span>
                  <span className="badge-neutral text-[10px]">{r.id}</span>
                </div>
              </div>
            ))
            }
          </div>
        </div>
      </div>


      {/* Team Cost Breakdown */}
      {teamStats.length > 0 && (
        <div className="card">
          <h2 className="card-header flex items-center justify-between">
            <span className="flex items-center gap-2"><TrendingUp size={16} /> {t("dashboard.team_cost_ranking")}</span>
            <span className="text-surface-500 font-normal text-xs">{t("dashboard.teams_count", { count: teamStats.length })}</span>
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="h-[200px]">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={teamStats.slice(0, 8)} layout="vertical" margin={{ left: 20, right: 20 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
                  <XAxis type="number" tick={{ fontSize: 11, fill: '#94a3b8' }} />
                  <YAxis type="category" dataKey="team" width={80} tick={{ fontSize: 11, fill: '#94a3b8' }} />
                  <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #334155', borderRadius: '8px' }} />
                  <Bar dataKey="total_tokens" fill="#818cf8" radius={[0, 4, 4, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
            <div className="space-y-2">
              {teamStats.slice(0, 8).map((ts, i) => (
                <div key={i} className="flex items-center justify-between py-2 px-3 rounded-lg bg-surface-950/50 border border-surface-800/50">
                  <div>
                    <span className="text-sm text-white font-medium">{ts.team}</span>
                    <span className="text-xs text-surface-500 ml-2">{t("dashboard.keys_count", { count: ts.key_count })}</span>
                  </div>
                  <div className="text-right">
                    <span className="text-sm font-mono text-brand-400">{ts.total_tokens.toLocaleString()}</span>
                    <span className="text-xs text-surface-500 ml-1">tokens</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
      {/* Cost Stats Table */}
      {costStats.length > 0 && (
        <div className="card">
          <h2 className="card-header flex items-center justify-between">
            <span className="flex items-center gap-2"><TrendingUp size={16} /> {t("dashboard.token_details")}</span>
            <span className="text-surface-500 font-normal text-xs">{t("dashboard.keys_detail", { count: costStats.length })}</span>
          </h2>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-xs text-surface-500 border-b border-surface-800">
                  <th className="text-left py-2 pr-4 font-medium">API Key</th>
                  <th className="text-right py-2 pr-4 font-medium">Input</th>
                  <th className="text-right py-2 pr-4 font-medium">Output</th>
                  <th className="text-right py-2 font-medium">Total</th>
                </tr>
              </thead>
              <tbody>
                {costStats.map((stat, i) => (
                  <tr key={i} className="border-b border-surface-800/50 hover:bg-surface-800/30 transition-colors">
                    <td className="py-2 pr-4 text-sm text-white font-mono">{stat.key_name}</td>
                    <td className="py-2 pr-4 text-right text-xs text-surface-300">{stat.input_tokens.toLocaleString()}</td>
                    <td className="py-2 pr-4 text-right text-xs text-surface-300">{stat.output_tokens.toLocaleString()}</td>
                    <td className="py-2 text-right text-xs font-medium text-brand-400">{stat.total_tokens.toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}


