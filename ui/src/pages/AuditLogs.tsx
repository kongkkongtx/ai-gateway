import { useState, useEffect, useCallback } from 'react'
import { RefreshCw, Search, Filter, AlertTriangle, Info, AlertCircle } from 'lucide-react'
import { getAuditLogs } from '../api/gateway'
import type { AuditLogEntry } from '../api/gateway'
const LOG_LEVELS = ['', 'info', 'warn', 'error']
const LEVEL_ICONS: Record<string, typeof AlertTriangle> = {
  info: Info,
  warn: AlertTriangle,
  error: AlertCircle,
}
const LEVEL_COLORS: Record<string, string> = {
  info: 'text-blue-400 bg-blue-400/10',
  warn: 'text-yellow-400 bg-yellow-400/10',
  error: 'text-red-400 bg-red-400/10',
}
export default function AuditLogsPage() {
  const [entries, setEntries] = useState<AuditLogEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [filterLevel, setFilterLevel] = useState('')
  const [filterKey, setFilterKey] = useState('')
  const [filterPath, setFilterPath] = useState('')
  const fetchData = useCallback(async (silent = false) => {
    if (!silent) setLoading(true)
    setError(null)
    try {
      const data = await getAuditLogs({
        limit: 200,
        level: filterLevel || undefined,
        key_name: filterKey || undefined,
        path: filterPath || undefined,
      })
      setEntries(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [filterLevel, filterKey, filterPath])
  useEffect(() => { fetchData() }, [fetchData])
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Audit Logs</h1>
          <p className="text-sm text-surface-400 mt-1">Recent API request history</p>
        </div>
        <button
          onClick={() => { setRefreshing(true); fetchData(true) }}
          disabled={refreshing}
          className="btn-ghost flex items-center gap-2"
        >
          <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>
      {error && (
        <div className="bg-red-900/20 border border-red-800/40 rounded-xl p-4 flex items-center gap-3">
          <AlertTriangle size={18} className="text-red-400 shrink-0" />
          <p className="text-sm text-red-300">{error}</p>
        </div>
      )}
      {/* Filters */}
      <div className="card">
        <div className="flex items-center gap-2 mb-3">
          <Filter size={14} className="text-surface-400" />
          <span className="text-sm font-medium text-white">Filters</span>
        </div>
        <div className="flex flex-wrap gap-4">
          <div className="flex-1 min-w-[140px]">
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Level</label>
            <select
              className="input w-full"
              value={filterLevel}
              onChange={(e) => setFilterLevel(e.target.value)}
            >
              {LOG_LEVELS.map((l) => (
                <option key={l} value={l}>{l || 'All Levels'}</option>
              ))}
            </select>
          </div>
          <div className="flex-1 min-w-[140px]">
            <label className="block text-xs font-medium text-surface-400 mb-1.5">API Key</label>
            <input
              className="input w-full font-mono text-xs"
              placeholder="Filter by key name..."
              value={filterKey}
              onChange={(e) => setFilterKey(e.target.value)}
            />
          </div>
          <div className="flex-1 min-w-[140px]">
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Path</label>
            <input
              className="input w-full font-mono text-xs"
              placeholder="/v1/chat/completions"
              value={filterPath}
              onChange={(e) => setFilterPath(e.target.value)}
            />
          </div>
        </div>
      </div>
      {/* Log Entries */}
      <div className="card">
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-sm font-medium text-white">Request Logs</h2>
          <span className="text-xs text-surface-500">{entries.length} entries</span>
        </div>
        <div className="space-y-1">
          {loading ? (
            Array.from({ length: 8 }).map((_, i) => (
              <div key={i} className="h-12 bg-surface-800 rounded-lg animate-pulse" />
            ))
          ) : entries.length === 0 ? (
            <div className="flex flex-col items-center py-12 text-surface-500">
              <Search size={32} className="mb-3 opacity-50" />
              <p className="text-sm">No audit logs found</p>
              <p className="text-xs mt-1">Try adjusting your filters or wait for API requests</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-xs text-surface-500 border-b border-surface-800">
                    <th className="text-left py-2 pr-4 font-medium">Time</th>
                    <th className="text-left py-2 pr-4 font-medium">Level</th>
                    <th className="text-left py-2 pr-4 font-medium">Method</th>
                    <th className="text-left py-2 pr-4 font-medium">Path</th>
                    <th className="text-left py-2 pr-4 font-medium">Status</th>
                    <th className="text-left py-2 pr-4 font-medium">Duration</th>
                    <th className="text-left py-2 pr-4 font-medium">Model</th>
                    <th className="text-left py-2 pr-4 font-medium">Tokens</th>
                    <th className="text-left py-2 font-medium">API Key</th>
                  </tr>
                </thead>
                <tbody>
                  {entries.map((e, i) => {
                    const LevelIcon = LEVEL_ICONS[e.level] || Info
                    const levelColor = LEVEL_COLORS[e.level] || 'text-surface-400'
                    return (
                      <tr key={i} className="border-b border-surface-800/50 hover:bg-surface-800/30 transition-colors">
                        <td className="py-2 pr-4 text-xs font-mono text-surface-300 whitespace-nowrap">
                          {new Date(e.timestamp).toLocaleString()}
                        </td>
                        <td className="py-2 pr-4">
                          <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-medium ${levelColor}`}>
                            <LevelIcon size={10} />
                            {e.level}
                          </span>
                        </td>
                        <td className="py-2 pr-4">
                          <span className="badge-neutral text-[10px]">{e.method}</span>
                        </td>
                        <td className="py-2 pr-4 text-xs font-mono text-surface-300 max-w-[200px] truncate">
                          {e.path}
                        </td>
                        <td className="py-2 pr-4">
                          <span className={`text-xs font-mono ${
                            e.status >= 500 ? 'text-red-400' :
                            e.status >= 400 ? 'text-yellow-400' :
                            'text-accent-400'
                          }`}>
                            {e.status}
                          </span>
                        </td>
                        <td className="py-2 pr-4 text-xs font-mono text-surface-400">
                          {e.duration}
                        </td>
                        <td className="py-2 pr-4 text-xs font-mono text-surface-400 max-w-[100px] truncate">
                          {e.model || '-'}
                        </td>
                        <td className="py-2 pr-4 text-xs text-surface-400 whitespace-nowrap">
                          {(e.tokens_in || e.tokens_out) ? ((e.tokens_in || 0) + String.fromCharCode(8594) + (e.tokens_out || 0)) : '-'}
                        </td>
                        <td className="py-2 text-xs text-surface-400 max-w-[120px] truncate">
                          {e.api_key_name || '-'}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
