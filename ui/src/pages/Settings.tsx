import { useState, useEffect, useCallback } from 'react'
import { RefreshCw, Shield, FileText, Gauge, Database } from 'lucide-react'
import { getGatewayConfig, updateGatewayConfig } from '../api/gateway'
import { useToast } from '../components/Toast'
import type { GatewayConfig } from '../api/gateway'

const LOG_LEVELS = ['debug', 'info', 'warn', 'error']
const LOG_FORMATS = ['text', 'json']

export default function SettingsPage() {
  const [config, setConfig] = useState<GatewayConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [savingField, setSavingField] = useState<string | null>(null)
  const { toast } = useToast()

  const fetchData = useCallback(async () => {
    try {
      const data = await getGatewayConfig()
      setConfig(data)
    } catch {} finally { setLoading(false) }
  }, [])

  useEffect(() => { fetchData() }, [fetchData])

  const updateField = async <K extends keyof GatewayConfig>(section: K, value: GatewayConfig[K]) => {
    if (!config) return
    setSaving(true)
    setSavingField(section as string)
    const prev = { ...config }
    setConfig({ ...config, [section]: value })
    try {
      await updateGatewayConfig({ [section]: value })
      toast('success', 'Settings saved', `${section} configuration updated`)
    } catch (err) {
      setConfig(prev)
      toast('error', 'Save failed', err instanceof Error ? err.message : undefined)
    } finally { setSaving(false); setSavingField(null) }
  }

  if (loading) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold text-white">Settings</h1>
        <div className="space-y-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="card"><div className="h-24 bg-surface-800 rounded animate-pulse" /></div>
          ))}
        </div>
      </div>
    )
  }

  if (!config) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold text-white">Settings</h1>
        <div className="card text-center py-12 text-sm text-surface-500">Failed to load configuration.</div>
      </div>
    )
  }

  return (
    <div className="space-y-6 max-w-3xl">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Settings</h1>
          <p className="text-sm text-surface-400 mt-1">Gateway configuration management</p>
        </div>
        <button onClick={fetchData} className="btn-ghost flex items-center gap-2">
          <RefreshCw size={14} /> Refresh
        </button>
      </div>

      {/* Auth Settings */}
      <div className="card">
        <h2 className="card-header flex items-center gap-2">
          <Shield size={14} /> Authentication
        </h2>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-sm font-medium text-white">API Key Authentication</div>
              <div className="text-xs text-surface-500">Require API key for all gateway requests</div>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input type="checkbox" className="sr-only peer"
                checked={config.auth.enabled}
                onChange={(e) => updateField('auth', { enabled: e.target.checked })}
                disabled={saving && savingField === 'auth'} />
              <div className="w-9 h-5 bg-surface-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-brand-600"></div>
            </label>
          </div>
        </div>
      </div>

      {/* Rate Limit Settings */}
      <div className="card">
        <h2 className="card-header flex items-center gap-2">
          <Gauge size={14} /> Rate Limiting
        </h2>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-sm font-medium text-white">Enable Rate Limiting</div>
              <div className="text-xs text-surface-500">Throttle requests to protect upstream providers</div>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input type="checkbox" className="sr-only peer"
                checked={config.rate_limit.enabled}
                onChange={(e) => updateField('rate_limit', {
                  ...config.rate_limit,
                  enabled: e.target.checked,
                })}
                disabled={saving && savingField === 'rate_limit'} />
              <div className="w-9 h-5 bg-surface-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-brand-600"></div>
            </label>
          </div>
          {config.rate_limit.enabled && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-surface-800">
              {config.rate_limit.global && (
                <>
                  <div>
                    <label className="block text-xs font-medium text-surface-400 mb-1.5">Global Limit (req/s)</label>
                    <input className="input w-full" type="number" min="1"
                      value={config.rate_limit.global.limit}
                      onChange={(e) => updateField('rate_limit', {
                        ...config.rate_limit,
                        global: { ...config.rate_limit.global!, limit: parseInt(e.target.value) || 1, window: config.rate_limit.global!.window }
                      })}
                      disabled={saving} />
                  </div>
                  <div>
                    <label className="block text-xs font-medium text-surface-400 mb-1.5">Global Window</label>
                    <input className="input w-full font-mono text-xs" placeholder="1s"
                      value={config.rate_limit.global.window}
                      onChange={(e) => updateField('rate_limit', {
                        ...config.rate_limit,
                        global: { ...config.rate_limit.global!, window: e.target.value }
                      })}
                      disabled={saving} />
                  </div>
                </>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Log Settings */}
      <div className="card">
        <h2 className="card-header flex items-center gap-2">
          <FileText size={14} /> Logging
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Log Level</label>
            <select className="input w-full"
              value={config.log.level}
              onChange={(e) => updateField('log', { ...config.log, level: e.target.value })}
              disabled={saving && savingField === 'log'}>
              {LOG_LEVELS.map((l) => <option key={l} value={l}>{l.toUpperCase()}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Log Format</label>
            <select className="input w-full"
              value={config.log.format}
              onChange={(e) => updateField('log', { ...config.log, format: e.target.value })}
              disabled={saving && savingField === 'log'}>
              {LOG_FORMATS.map((f) => <option key={f} value={f}>{f}</option>)}
            </select>
          </div>
        </div>
      </div>

      {/* Redis Settings */}
      <div className="card">
        <h2 className="card-header flex items-center gap-2">
          <Database size={14} /> Redis
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Address</label>
            <input className="input w-full font-mono text-xs" placeholder="localhost:6379"
              value={config.redis.addr}
              onChange={(e) => updateField('redis', { ...config.redis, addr: e.target.value })}
              disabled={saving} />
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Password</label>
            <input className="input w-full font-mono text-xs" type="password"
              value={config.redis.password}
              onChange={(e) => updateField('redis', { ...config.redis, password: e.target.value })}
              disabled={saving} />
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">DB</label>
            <input className="input w-full" type="number" min="0"
              value={config.redis.db}
              onChange={(e) => updateField('redis', { ...config.redis, db: parseInt(e.target.value) || 0 })}
              disabled={saving} />
          </div>
        </div>
      </div>

      {/* Save indicator */}
      {saving && (
        <div className="flex items-center gap-2 text-xs text-accent-400">
          <RefreshCw size={12} className="animate-spin" /> Saving...
        </div>
      )}
    </div>
  )
}
