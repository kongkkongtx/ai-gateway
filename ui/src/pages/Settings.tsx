import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { RefreshCw, Shield, FileText, Gauge, Database } from 'lucide-react'
import { getGatewayConfig, updateGatewayConfig, updateWebhookConfig, getWebhookConfig } from '../api/gateway'
import { useToast } from '../components/Toast'

import type { GatewayConfig, WebhookEndpoint } from '../api/gateway'

const LOG_LEVELS = ['debug', 'info', 'warn', 'error']
const LOG_FORMATS = ['text', 'json']

export default function SettingsPage() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const [config, setConfig] = useState<GatewayConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [webhookEndpoints, setWebhookEndpoints] = useState<WebhookEndpoint[]>([])
  const [webhookSaving, setWebhookSaving] = useState(false)
  const [_webhookLoading, setWebhookLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [savingField, setSavingField] = useState<string | null>(null)
  

  const fetchData = useCallback(async () => {
    try {
      const data = await getGatewayConfig()
      setConfig(data)
    } catch {} finally { setLoading(false) }
  }, [])

  const fetchWebhookConfig = useCallback(async () => {
    try {
      const data = await getWebhookConfig()
      setWebhookEndpoints(data.endpoints || [])
    } catch {} finally { setWebhookLoading(false) }
  }, [])

  useEffect(() => { fetchData() }, [fetchData])
  useEffect(() => { fetchWebhookConfig() }, [fetchWebhookConfig])

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

  const handleSaveWebhook = async () => {
    setWebhookSaving(true)
    try {
      await updateWebhookConfig({ endpoints: webhookEndpoints })
      toast('success', 'Webhook Config Saved', '')
    } catch (err) {
      toast('error', 'Save failed', err instanceof Error ? err.message : undefined)
    }
    setWebhookSaving(false)
  }

  if (loading) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold text-white">{t('settings.title')}</h1>
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

      {/* Security Settings */}
      <div className="card">
        <h2 className="card-header flex items-center gap-2">
          <Shield size={14} /> Security
        </h2>
        <div className="space-y-6">
          {/* Prompt Injection */}
          <div>
            <div className="flex items-center justify-between mb-3">
              <div>
                <div className="text-sm font-medium text-white">Prompt Injection Detection</div>
                <div className="text-xs text-surface-500">Detect and block/sanitize prompt injection attempts</div>
              </div>
              <label className="relative inline-flex items-center cursor-pointer">
                <input type="checkbox" className="sr-only peer"
                  checked={config.security?.prompt_injection.enabled ?? false}
                  onChange={(e) => updateField('security', {
                    ...config.security,
                    prompt_injection: { ...config.security?.prompt_injection, enabled: e.target.checked, action: config.security?.prompt_injection.action || 'block', risk_threshold: config.security?.prompt_injection.risk_threshold || 'medium' }
                  } as any)}
                  disabled={saving} />
                <div className="w-9 h-5 bg-surface-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-brand-600"></div>
              </label>
            </div>
            {(config.security?.prompt_injection.enabled) && (
              <div className="grid grid-cols-2 gap-4 pt-3 border-t border-surface-800">
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">Action</label>
                  <select className="input w-full"
                    value={config.security?.prompt_injection.action || 'block'}
                    onChange={(e) => updateField('security', {
                      ...config.security,
                      prompt_injection: { ...config.security?.prompt_injection, action: e.target.value }
                    } as any)}
                    disabled={saving}>
                    <option value="block">BLOCK</option>
                    <option value="log">LOG</option>
                    <option value="sanitize">SANITIZE</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">Risk Threshold</label>
                  <select className="input w-full"
                    value={config.security?.prompt_injection.risk_threshold || 'medium'}
                    onChange={(e) => updateField('security', {
                      ...config.security,
                      prompt_injection: { ...config.security?.prompt_injection, risk_threshold: e.target.value }
                    } as any)}
                    disabled={saving}>
                    <option value="low">LOW</option>
                    <option value="medium">MEDIUM</option>
                    <option value="high">HIGH</option>
                    <option value="critical">CRITICAL</option>
                  </select>
                </div>
              </div>
            )}
          </div>

          {/* PII Redaction */}
          <div className="border-t border-surface-800 pt-4">
            <div className="flex items-center justify-between mb-3">
              <div>
                <div className="text-sm font-medium text-white">PII Redaction</div>
                <div className="text-xs text-surface-500">Automatically detect and mask sensitive information</div>
              </div>
              <label className="relative inline-flex items-center cursor-pointer">
                <input type="checkbox" className="sr-only peer"
                  checked={config.security?.pii.enabled ?? false}
                  onChange={(e) => updateField('security', {
                    ...config.security,
                    pii: { ...config.security?.pii, enabled: e.target.checked, action: config.security?.pii.action || 'mask' }
                  } as any)}
                  disabled={saving} />
                <div className="w-9 h-5 bg-surface-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-brand-600"></div>
              </label>
            </div>
            {(config.security?.pii.enabled) && (
              <div className="grid grid-cols-2 gap-4 pt-3 border-t border-surface-800">
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">Action</label>
                  <select className="input w-full"
                    value={config.security?.pii.action || 'mask'}
                    onChange={(e) => updateField('security', {
                      ...config.security,
                      pii: { ...config.security?.pii, action: e.target.value }
                    } as any)}
                    disabled={saving}>
                    <option value="mask">MASK</option>
                    <option value="hash">HASH</option>
                    <option value="block">BLOCK</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">Types</label>
                  <input className="input w-full font-mono text-xs" placeholder="email, phone, ip, ssn"
                    value={config.security?.pii.types?.join(', ') || ''}
                    onChange={(e) => updateField('security', {
                      ...config.security,
                      pii: { ...config.security?.pii, types: e.target.value.split(',').map((t: string) => t.trim()).filter(Boolean) }
                    } as any)}
                    disabled={saving} />
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Semantic Settings */}
      <div className="card">
        <h2 className="card-header flex items-center gap-2">
          <Database size={14} /> Semantic
        </h2>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-sm font-medium text-white">Semantic Routing</div>
              <div className="text-xs text-surface-500">Route requests based on query content embedding similarity</div>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input type="checkbox" className="sr-only peer"
                checked={config.semantic?.enabled ?? false}
                onChange={(e) => updateField('semantic', {
                  ...config.semantic, enabled: e.target.checked, provider: config.semantic?.provider || '', threshold: config.semantic?.threshold || 0.75
                } as any)}
                disabled={saving} />
              <div className="w-9 h-5 bg-surface-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-brand-600"></div>
            </label>
          </div>
          {(config.semantic?.enabled) && (
            <div className="grid grid-cols-2 gap-4 pt-3 border-t border-surface-800">
              <div>
                <label className="block text-xs font-medium text-surface-400 mb-1.5">Embedding Provider</label>
                <input className="input w-full font-mono text-xs" placeholder="deepseek-primary"
                  value={config.semantic?.provider || ''}
                  onChange={(e) => updateField('semantic', { ...config.semantic, provider: e.target.value } as any)}
                  disabled={saving} />
              </div>
              <div>
                <label className="block text-xs font-medium text-surface-400 mb-1.5">Similarity Threshold</label>
                <input className="input w-full" type="number" min="0" max="1" step="0.05"
                  value={config.semantic?.threshold ?? 0.75}
                  onChange={(e) => updateField('semantic', { ...config.semantic, threshold: parseFloat(e.target.value) || 0.75 } as any)}
                  disabled={saving} />
              </div>
            </div>
          )}

          {/* Semantic Cache */}
          <div className="border-t border-surface-800 pt-4">
            <div className="flex items-center justify-between mb-3">
              <div>
                <div className="text-sm font-medium text-white">Semantic Cache</div>
                <div className="text-xs text-surface-500">Cache responses for semantically similar queries</div>
              </div>
              <label className="relative inline-flex items-center cursor-pointer">
                <input type="checkbox" className="sr-only peer"
                  checked={config.semantic_cache?.enabled ?? false}
                  onChange={(e) => updateField('semantic_cache', {
                    ...config.semantic_cache, enabled: e.target.checked, threshold: config.semantic_cache?.threshold || 0.92, ttl: config.semantic_cache?.ttl || '10m', max_entries: config.semantic_cache?.max_entries || 1000
                  } as any)}
                  disabled={saving} />
                <div className="w-9 h-5 bg-surface-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-brand-600"></div>
              </label>
            </div>
            {(config.semantic_cache?.enabled) && (
              <div className="grid grid-cols-3 gap-4 pt-3 border-t border-surface-800">
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">Cache Threshold</label>
                  <input className="input w-full" type="number" min="0" max="1" step="0.01"
                    value={config.semantic_cache?.threshold ?? 0.92}
                    onChange={(e) => updateField('semantic_cache', { ...config.semantic_cache, threshold: parseFloat(e.target.value) || 0.92 } as any)}
                    disabled={saving} />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">TTL</label>
                  <input className="input w-full font-mono text-xs" placeholder="10m"
                    value={config.semantic_cache?.ttl || '10m'}
                    onChange={(e) => updateField('semantic_cache', { ...config.semantic_cache, ttl: e.target.value } as any)}
                    disabled={saving} />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">Max Entries</label>
                  <input className="input w-full" type="number" min="0"
                    value={config.semantic_cache?.max_entries ?? 1000}
                    onChange={(e) => updateField('semantic_cache', { ...config.semantic_cache, max_entries: parseInt(e.target.value) || 0 } as any)}
                    disabled={saving} />
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Cost Control Settings */}
      <div className="card">
        <h2 className="card-header flex items-center gap-2">
          <Gauge size={14} /> Cost Control
        </h2>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-sm font-medium text-white">Enable Cost Control</div>
              <div className="text-xs text-surface-500">Enforce token quotas and budget limits per API key</div>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input type="checkbox" className="sr-only peer"
                checked={config.cost?.enabled ?? false}
                onChange={(e) => updateField('cost', {
                  ...config.cost, enabled: e.target.checked,
                  default_limit: config.cost?.default_limit || { input_tokens: 1000000, output_tokens: 500000, window: '1h' },
                  degrade: config.cost?.degrade || { action: 'warn', cheaper_provider: '', alert_webhook: '', alert_threshold: 0.8 }
                } as any)}
                disabled={saving} />
              <div className="w-9 h-5 bg-surface-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-brand-600"></div>
            </label>
          </div>
          {(config.cost?.enabled) && (
            <div className="space-y-4 pt-3 border-t border-surface-800">
              <div className="grid grid-cols-3 gap-4">
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">Input Token Limit</label>
                  <input className="input w-full" type="number" min="0"
                    value={config.cost?.default_limit?.input_tokens ?? 1000000}
                    onChange={(e) => updateField('cost', {
                      ...config.cost, default_limit: { ...config.cost?.default_limit, input_tokens: parseInt(e.target.value) || 0 }
                    } as any)}
                    disabled={saving} />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">Output Token Limit</label>
                  <input className="input w-full" type="number" min="0"
                    value={config.cost?.default_limit?.output_tokens ?? 500000}
                    onChange={(e) => updateField('cost', {
                      ...config.cost, default_limit: { ...config.cost?.default_limit, output_tokens: parseInt(e.target.value) || 0 }
                    } as any)}
                    disabled={saving} />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">Window</label>
                  <input className="input w-full font-mono text-xs" placeholder="1h"
                    value={config.cost?.default_limit?.window || '1h'}
                    onChange={(e) => updateField('cost', {
                      ...config.cost, default_limit: { ...config.cost?.default_limit, window: e.target.value }
                    } as any)}
                    disabled={saving} />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4 pt-2 border-t border-surface-800">
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">Degrade Action</label>
                  <select className="input w-full"
                    value={config.cost?.degrade?.action || 'warn'}
                    onChange={(e) => updateField('cost', {
                      ...config.cost, degrade: { ...config.cost?.degrade, action: e.target.value }
                    } as any)}
                    disabled={saving}>
                    <option value="warn">WARN</option>
                    <option value="block">BLOCK</option>
                    <option value="degrade">DEGRADE</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">Alert Threshold</label>
                  <input className="input w-full" type="number" min="0" max="1" step="0.1"
                    value={config.cost?.degrade?.alert_threshold ?? 0.8}
                    onChange={(e) => updateField('cost', {
                      ...config.cost, degrade: { ...config.cost?.degrade, alert_threshold: parseFloat(e.target.value) || 0.8 }
                    } as any)}
                    disabled={saving} />
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Webhook Settings */}
      <div className="card">
        <h2 className="card-header flex items-center gap-2">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8a6 6 0 0 1-6 6"/><path d="M6 8a6 6 0 0 1 12 0c0 5-3 8-6 8"/><path d="M6 8a6 6 0 0 1 0 0Z"/><path d="M12 16v4"/><path d="M8 22h8"/></svg>
          Webhook Notifications
        </h2>
        <div className="space-y-4">
          <div className="text-xs text-surface-500 mb-2">Configure webhook endpoints for event notifications (cost alerts, security events, upstream status changes)</div>
          {webhookEndpoints.map((ep, i) => (
            <div key={i} className="p-3 rounded-lg bg-surface-950/50 border border-surface-800/50 space-y-3">
              <div className="flex items-center justify-between">
                <input className="input flex-1 mr-2 text-sm" value={ep.name} onChange={(e) => { const next = [...webhookEndpoints]; next[i] = {...next[i], name: e.target.value}; setWebhookEndpoints(next); }} placeholder="Endpoint name" />
                <label className="relative inline-flex items-center cursor-pointer shrink-0">
                  <input type="checkbox" className="sr-only peer" checked={ep.enabled} onChange={(e) => { const next = [...webhookEndpoints]; next[i] = {...next[i], enabled: e.target.checked}; setWebhookEndpoints(next); }} />
                  <div className="w-9 h-5 bg-surface-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-brand-600"></div>
                </label>
              </div>
              <input className="input w-full font-mono text-xs" value={ep.url} onChange={(e) => { const next = [...webhookEndpoints]; next[i] = {...next[i], url: e.target.value}; setWebhookEndpoints(next); }} placeholder="https://hooks.example.com/alert" />
              <div className="flex items-center gap-4">
                <div className="flex-1">
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">Events</label>
                  <input className="input w-full font-mono text-xs" value={(ep.events || []).join(', ')} onChange={(e) => { const next = [...webhookEndpoints]; next[i] = {...next[i], events: e.target.value.split(',').map(s => s.trim()).filter(Boolean)}; setWebhookEndpoints(next); }} placeholder="cost.alert, security.block, upstream.down" />
                </div>
                <div className="w-24">
                  <label className="block text-xs font-medium text-surface-400 mb-1.5">Retries</label>
                  <input className="input w-full" type="number" min="0" max="10" value={ep.retry ?? 3} onChange={(e) => { const next = [...webhookEndpoints]; next[i] = {...next[i], retry: parseInt(e.target.value) || 3}; setWebhookEndpoints(next); }} />
                </div>
              </div>
            </div>
          ))}
          <button onClick={() => setWebhookEndpoints([...webhookEndpoints, { name: '', url: '', enabled: true, events: [], retry: 3 }])} className="btn-ghost text-xs flex items-center gap-1">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
            Add Endpoint
          </button>
          <div className="flex justify-end pt-2">
            <button onClick={handleSaveWebhook} disabled={webhookSaving} className="btn-primary text-sm">{webhookSaving ? 'Saving...' : 'Save Webhook Config'}</button>
          </div>
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


