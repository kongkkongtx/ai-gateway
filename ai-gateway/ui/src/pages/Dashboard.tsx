import { useState, useEffect, useCallback, useRef } from 'react'
import {
  Activity,
  Network,
  Route,
  Zap,
  RefreshCw,
  AlertTriangle,
  CheckCircle2,
  Server,
  TrendingUp,
  Clock,
} from 'lucide-react'
import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from "recharts"
import { getHealth, getStatus } from '../api/gateway'
import StatCard from '../components/StatCard'
import StatusBadge from '../components/StatusBadge'
import type { GatewayStatus } from '../types'

interface DataPoint {
  time: string
  requests: number
  latency: number
}

const MAX_POINTS = 30 // 30 data points = 2.5 min at 5s interval

export default function Dashboard() {
  const [status, setStatus] = useState<GatewayStatus | null>(null)
  const [health, setHealth] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [refreshing, setRefreshing] = useState(false)
  const [chartData, setChartData] = useState<DataPoint[]>([])
  const lastHealthyCount = useRef(0)
  const lastTotalCount = useRef(0)

  const fetchData = useCallback(async (silent = false) => {
    if (!silent) setLoading(true)
    setError(null)
    try {
      const [h, s] = await Promise.all([getHealth(), getStatus()])
      setHealth(h.status)
      setStatus(s)

      // Track metrics for chart
      const now = new Date().toLocaleTimeString()
      const healthyCount = s.upstreams.filter((u) => u.healthy).length
      const totalConns = s.upstreams.reduce((sum, u) => sum + (u.active_conns ?? 0), 0)

      setChartData((prev) => {
        const next = [
          ...prev,
          { time: now, requests: totalConns, latency: healthyCount },
        ]
        if (next.length > MAX_POINTS) next.shift()
        return next
      })

      lastHealthyCount.current = healthyCount
      lastTotalCount.current = s.upstreams.length
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

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Dashboard</h1>
          <p className="text-sm text-surface-400 mt-1">Real-time AI Gateway monitoring</p>
        </div>
        <div className="flex items-center gap-3">
          {loading && <div className="text-xs text-surface-500 animate-pulse">Refreshing...</div>}
          <button
            onClick={() => { setRefreshing(true); fetchData(true) }}
            disabled={refreshing}
            className="btn-ghost flex items-center gap-2"
          >
            <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} />
            Refresh
          </button>
        </div>
      </div>

      {/* Error */}
      {error && (
        <div className="bg-red-900/20 border border-red-800/40 rounded-xl p-4 flex items-center gap-3">
          <AlertTriangle size={18} className="text-red-400 shrink-0" />
          <div>
            <p className="text-sm font-medium text-red-300">Connection Error</p>
            <p className="text-xs text-red-400/80 mt-0.5">Cannot reach AI Gateway &mdash; {error}</p>
          </div>
        </div>
      )}

      {/* Health Banner */}
      {!error && !loading && (
        <div className={`rounded-xl p-4 flex items-center gap-3 border ${
          isHealthy ? 'bg-accent-900/20 border-accent-800/40' : 'bg-yellow-900/20 border-yellow-800/40'
        }`}>
          {isHealthy ? (
            <CheckCircle2 size={20} className="text-accent-400 shrink-0" />
          ) : (
            <AlertTriangle size={20} className="text-yellow-400 shrink-0" />
          )}
          <div>
            <p className="text-sm font-medium text-white">
              {isHealthy ? 'All systems operational' : 'Service degraded'}
            </p>
            <p className="text-xs text-surface-400 mt-0.5">Gateway health check passed</p>
          </div>
        </div>
      )}

      {/* Stat Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          label="Status"
          value={isHealthy ? 'Healthy' : 'Degraded'}
          icon={<Activity size={20} />}
          loading={loading}
        />
        <StatCard
          label="Upstreams"
          value={`${healthyCount}/${totalUpstreams}`}
          icon={<Server size={20} />}
          loading={loading}
        />
        <StatCard
          label="Routes"
          value={totalRoutes}
          icon={<Route size={20} />}
          loading={loading}
        />
        <StatCard
          label="Strategy"
          value="Weighted Random"
          icon={<Network size={20} />}
          loading={loading}
        />
      </div>

      {/* Metrics Chart */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div className="card">
          <h2 className="card-header flex items-center gap-2">
            <TrendingUp size={14} /> Active Connections
          </h2>
          {chartData.length === 0 ? (
            <div className="h-48 flex items-center justify-center text-surface-500 text-sm">Collecting data...</div>
          ) : (
            <div className="h-48">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={chartData}>
                  <defs>
                    <linearGradient id="reqGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#818cf8" stopOpacity={0.3} />
                      <stop offset="95%" stopColor="#818cf8" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                  <XAxis dataKey="time" tick={{ fill: '#64748b', fontSize: 10 }} interval="preserveStartEnd" />
                  <YAxis tick={{ fill: '#64748b', fontSize: 10 }} allowDecimals={false} />
                  <Tooltip
                    contentStyle={{ background: '#0f172a', border: '1px solid #334155', borderRadius: '8px' }}
                    labelStyle={{ color: '#94a3b8' }}
                  />
                  <Area type="monotone" dataKey="requests" stroke="#818cf8" fill="url(#reqGradient)" strokeWidth={2} dot={false} />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          )}
        </div>

        <div className="card">
          <h2 className="card-header flex items-center gap-2">
            <Clock size={14} /> Avg Active Connections
          </h2>
          {chartData.length === 0 ? (
            <div className="h-48 flex items-center justify-center text-surface-500 text-sm">Collecting data...</div>
          ) : (
            <div className="h-48">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={chartData}>
                  <defs>
                    <linearGradient id="latGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#34d399" stopOpacity={0.3} />
                      <stop offset="95%" stopColor="#34d399" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                  <XAxis dataKey="time" tick={{ fill: '#64748b', fontSize: 10 }} interval="preserveStartEnd" />
                  <YAxis tick={{ fill: '#64748b', fontSize: 10 }} />
                  <Tooltip
                    contentStyle={{ background: '#0f172a', border: '1px solid #334155', borderRadius: '8px' }}
                    labelStyle={{ color: '#94a3b8' }}
                    formatter={(value: number) => [`${value} ms`, 'Latency']}
                  />
                  <Area type="monotone" dataKey="requests" stroke="#34d399" fill="url(#latGradient)" strokeWidth={2} dot={false} />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          )}
        </div>
      </div>

      {/* Upstream Health Summary */}
      <div className="card">
        <h2 className="card-header flex items-center justify-between">
          <span>Upstream Health</span>
          <span className="text-surface-500 font-normal normal-case text-xs">{totalUpstreams} nodes</span>
        </h2>
        <div className="space-y-3">
          {loading ? (
            Array.from({ length: 2 }).map((_, i) => (
              <div key={i} className="h-12 bg-surface-800 rounded-lg animate-pulse" />
            ))
          ) : !status?.upstreams.length ? (
            <p className="text-sm text-surface-500 py-4 text-center">No upstreams configured</p>
          ) : (
            status?.upstreams.map((u) => (
              <div key={u.name} className="flex items-center justify-between py-2.5 px-4 rounded-lg bg-surface-950/50 border border-surface-800/50 hover:border-surface-700 transition-colors">
                <div className="flex items-center gap-3 min-w-0">
                  <StatusBadge status={u.healthy ? 'healthy' : 'unhealthy'} pulse />
                  <div>
                    <div className="text-sm font-medium text-white">{u.name}</div>
                    <div className="text-xs text-surface-500 font-mono truncate max-w-[300px]">{u.endpoint}</div>
                  </div>
                </div>
                <div className="flex items-center gap-6 text-xs text-surface-400">
                  <span className="flex items-center gap-1">
                    <Zap size={12} className="text-yellow-500" />
                    W:{u.weight}
                  </span>
                  <span>{u.provider}</span>
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Routes Summary */}
      <div className="card">
        <h2 className="card-header flex items-center justify-between">
          <span>Routes Overview</span>
          <span className="text-surface-500 font-normal normal-case text-xs">{totalRoutes} rules</span>
        </h2>
        <div className="space-y-2">
          {loading ? (
            Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="h-10 bg-surface-800 rounded-lg animate-pulse" />
            ))
          ) : !status?.routes.length ? (
            <p className="text-sm text-surface-500 py-4 text-center">No routes configured</p>
          ) : (
            status?.routes.map((r) => (
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
          )}
        </div>
      </div>
    </div>
  )
}
