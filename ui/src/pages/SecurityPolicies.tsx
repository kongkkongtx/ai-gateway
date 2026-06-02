import { useState, useEffect, useCallback } from 'react'
import { Shield, Save, AlertTriangle } from 'lucide-react'
import { getSecurityPolicies, updateSecurityPolicies, type SecurityPolicies } from '../api/gateway'

export default function SecurityPoliciesPage() {
  const [policies, setPolicies] = useState<SecurityPolicies | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetch = useCallback(async () => {
    setLoading(true)
    try {
      const p = await getSecurityPolicies()
      setPolicies(p)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load policies')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetch() }, [fetch])

  const handleSave = async () => {
    if (!policies) return
    setSaving(true)
    setSaved(false)
    try {
      await updateSecurityPolicies(policies)
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  const togglePI = (v: boolean) => setPolicies(p => p ? { ...p, prompt_injection: { ...p.prompt_injection, enabled: v } } : p)
  const togglePII = (v: boolean) => setPolicies(p => p ? { ...p, pii: { ...p.pii, enabled: v } } : p)
  const setPIAction = (v: string) => setPolicies(p => p ? { ...p, prompt_injection: { ...p.prompt_injection, action: v } } : p)
  const setPIRisk = (v: string) => setPolicies(p => p ? { ...p, prompt_injection: { ...p.prompt_injection, risk_threshold: v } } : p)
  const setPIIAction = (v: string) => setPolicies(p => p ? { ...p, pii: { ...p.pii, action: v } } : p)

  if (loading) return <div className="text-sm text-surface-500 py-8 text-center">Loading...</div>
  if (!policies) return <div className="text-sm text-red-400">{error}</div>

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Security Policies</h1>
          <p className="text-sm text-surface-400 mt-1">Configure prompt injection detection, PII masking, and IP filtering</p>
        </div>
        <button onClick={handleSave} disabled={saving} className="btn-primary flex items-center gap-2">
          <Save size={16} /> {saving ? 'Saving...' : saved ? 'Saved!' : 'Save Changes'}
        </button>
      </div>

      {error && <div className="bg-red-900/20 border border-red-800/40 rounded-lg p-3 text-sm text-red-300">{error}</div>}
      {saved && <div className="bg-accent-900/20 border border-accent-800/40 rounded-lg p-3 text-sm text-accent-300">Settings saved successfully</div>}

      {/* Prompt Injection */}
      <div className="card">
        <div className="flex items-center justify-between mb-4">
          <h2 className="card-header flex items-center gap-2"><AlertTriangle size={16} /> Prompt Injection Detection</h2>
          <label className="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" checked={policies.prompt_injection.enabled} onChange={e => togglePI(e.target.checked)} className="sr-only peer" />
            <div className="w-9 h-5 bg-surface-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:bg-brand-600 after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all" />
          </label>
        </div>
        {policies.prompt_injection.enabled && (
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs text-surface-500 mb-1">Action</label>
              <select value={policies.prompt_injection.action} onChange={e => setPIAction(e.target.value)} className="input">
                <option value="block">Block Request</option>
                <option value="log">Log Only</option>
                <option value="sanitize">Sanitize Content</option>
              </select>
            </div>
            <div>
              <label className="block text-xs text-surface-500 mb-1">Risk Threshold</label>
              <select value={policies.prompt_injection.risk_threshold} onChange={e => setPIRisk(e.target.value)} className="input">
                <option value="low">Low</option>
                <option value="medium">Medium</option>
                <option value="high">High</option>
                <option value="critical">Critical</option>
              </select>
            </div>
          </div>
        )}
      </div>

      {/* PII Redaction */}
      <div className="card">
        <div className="flex items-center justify-between mb-4">
          <h2 className="card-header flex items-center gap-2"><Shield size={16} /> PII Redaction</h2>
          <label className="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" checked={policies.pii.enabled} onChange={e => togglePII(e.target.checked)} className="sr-only peer" />
            <div className="w-9 h-5 bg-surface-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:bg-brand-600 after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all" />
          </label>
        </div>
        {policies.pii.enabled && (
          <div>
            <label className="block text-xs text-surface-500 mb-1">Action</label>
            <select value={policies.pii.action} onChange={e => setPIIAction(e.target.value)} className="input w-64">
              <option value="mask">Mask (e.g., j***@example.com)</option>
              <option value="hash">Hash</option>
              <option value="block">Block Request</option>
            </select>
          </div>
        )}
      </div>

      {/* IP Filtering */}
      <div className="card">
        <h2 className="card-header flex items-center gap-2"><Shield size={16} /> IP Filtering</h2>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs text-surface-500 mb-1">IP Allowlist (one per line, * for wildcard)</label>
            <textarea
              value={(policies.ip_allowlist || []).join('\n')}
              onChange={e => setPolicies(p => p ? { ...p, ip_allowlist: e.target.value.split('\n').filter(Boolean) } : p)}
              className="input min-h-[100px] font-mono text-sm"
              placeholder="10.0.0.0/8&#10;192.168.*"
            />
          </div>
          <div>
            <label className="block text-xs text-surface-500 mb-1">IP Blocklist (one per line, * for wildcard)</label>
            <textarea
              value={(policies.ip_blocklist || []).join('\n')}
              onChange={e => setPolicies(p => p ? { ...p, ip_blocklist: e.target.value.split('\n').filter(Boolean) } : p)}
              className="input min-h-[100px] font-mono text-sm"
              placeholder="10.0.0.5&#10;203.0.113.*"
            />
          </div>
        </div>
      </div>
    </div>
  )
}
