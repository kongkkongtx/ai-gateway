import { useState, useEffect, useCallback } from 'react'
import { Key, Plus, Trash2, Copy, Check, Clock, RefreshCw, Shield, Eye, EyeOff } from 'lucide-react'
import { getApiKeys, addApiKey, deleteApiKey } from '../api/gateway'
import Modal from '../components/Modal'
import ConfirmDialog from '../components/ConfirmDialog'
import { useToast } from '../components/Toast'
import type { ApiKeyConfig } from '../api/gateway'

interface KeyWithRoles extends ApiKeyConfig {
  roles?: string[]
}

export default function KeysPage() {
  const [keys, setKeys] = useState<KeyWithRoles[]>([])
  const [loading, setLoading] = useState(true)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
  const [showAdd, setShowAdd] = useState(false)
  const [newKey, setNewKey] = useState('')
  const [newName, setNewName] = useState('')
  const [newRoles, setNewRoles] = useState('')
  const [saving, setSaving] = useState(false)
  const [confirmTarget, setConfirmTarget] = useState<{key: string; name: string} | null>(null)
  const [confirmLoading, setConfirmLoading] = useState(false)
  const [copiedKey, setCopiedKey] = useState<string | null>(null)
  const [showKey, setShowKey] = useState<string | null>(null)
  const { toast } = useToast()

  const fetchData = useCallback(async () => {
    try {
      const data = await getApiKeys() as KeyWithRoles[]
      setKeys(data)
      setLastUpdated(new Date())
    } catch {} finally { setLoading(false) }
  }, [])

  useEffect(() => { fetchData(); const t = setInterval(fetchData, 10000); return () => clearInterval(t) }, [fetchData])

  const handleAdd = async () => {
    if (!newKey.trim() || !newName.trim()) {
      toast('error', 'Validation', 'Key and name are required')
      return
    }
    setSaving(true)
    try {
      const roles = newRoles.trim() ? newRoles.split(',').map((r) => r.trim()).filter(Boolean) : undefined
      await addApiKey(newKey.trim(), newName.trim(), roles)
      toast('success', 'API Key added', `Key "${newName.trim()}" created`)
      setShowAdd(false)
      setNewKey('')
      setNewName('')
      setNewRoles('')
      fetchData()
    } catch (err) {
      toast('error', 'Failed to add key', err instanceof Error ? err.message : undefined)
    } finally { setSaving(false) }
  }

  const handleDelete = async (key: string, name: string) => {
    setConfirmTarget({ key, name })
  }

  const confirmDelete = async () => {
    if (!confirmTarget) return
    setConfirmLoading(true)
    try {
      await deleteApiKey(confirmTarget.key)
      toast('success', 'API Key deleted', `Key "${confirmTarget.name}" removed`)
      setConfirmTarget(null)
      fetchData()
    } catch (err) {
      toast('error', 'Failed to delete key', err instanceof Error ? err.message : undefined)
    } finally { setConfirmLoading(false) }
  }

  const copyToClipboard = async (key: string) => {
    await navigator.clipboard.writeText(key)
    setCopiedKey(key)
    setTimeout(() => setCopiedKey(null), 2000)
  }

  const generateKey = () => {
    const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
    const random = Array.from({ length: 32 }, () => chars[Math.floor(Math.random() * chars.length)]).join('')
    setNewKey('sk-' + random)
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">API Keys</h1>
          <p className="text-sm text-surface-400 mt-1">
            {loading ? 'Loading...' : `${keys.length} key${keys.length !== 1 ? 's' : ''} configured`}
          </p>
        </div>
        <div className="flex items-center gap-3">
          {lastUpdated && (
            <span className="text-[10px] text-surface-500 flex items-center gap-1">
              <Clock size={10} />
              {lastUpdated.toLocaleTimeString()}
            </span>
          )}
          <button onClick={() => setShowAdd(true)} className="btn-primary flex items-center gap-2 text-sm">
            <Plus size={14} /> Add Key
          </button>
          <button onClick={fetchData} className="btn-ghost flex items-center gap-2">
            <RefreshCw size={14} /> Refresh
          </button>
        </div>
      </div>

      {/* Key Table */}
      <div className="card p-0 overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-surface-800">
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">Name</th>
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">API Key</th>
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">Roles</th>
              <th className="text-right text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-surface-800/50">
            {loading ? (
              Array.from({ length: 2 }).map((_, i) => (
                <tr key={i}>
                  {Array.from({ length: 4 }).map((_, j) => (
                    <td key={j} className="px-6 py-4"><div className="h-5 bg-surface-800 rounded animate-pulse" /></td>
                  ))}
                </tr>
              ))
            ) : keys.length === 0 ? (
              <tr><td colSpan={4} className="px-6 py-12 text-center text-sm text-surface-500">No API keys configured. Add one to get started.</td></tr>
            ) : (
              keys.map((k, idx) => (
                <tr key={k.key} className={`hover:bg-surface-900/50 transition-colors ${idx % 2 === 0 ? 'bg-surface-950/20' : ''}`}>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-3">
                      <div className="w-7 h-7 rounded-lg bg-amber-600/10 border border-amber-600/20 flex items-center justify-center">
                        <Key size={14} className="text-amber-400" />
                      </div>
                      <span className="text-sm font-medium text-white">{k.name}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <code className="text-sm font-mono text-surface-300 bg-surface-800 px-2 py-1 rounded">
                        {showKey === k.key ? k.key : k.key.slice(0, 12) + '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022'}
                      </code>
                      <button onClick={() => setShowKey(showKey === k.key ? null : k.key)}
                        className="text-surface-500 hover:text-surface-300 transition-colors"
                        title={showKey === k.key ? 'Hide' : 'Show'}>
                        {showKey === k.key ? <EyeOff size={14} /> : <Eye size={14} />}
                      </button>
                      <button onClick={() => copyToClipboard(k.key)}
                        className="text-surface-500 hover:text-surface-300 transition-colors"
                        title="Copy">
                        {copiedKey === k.key ? <Check size={14} className="text-accent-400" /> : <Copy size={14} />}
                      </button>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    {k.roles && k.roles.length > 0 ? (
                      <div className="flex flex-wrap gap-1">
                        {k.roles.map((role) => (
                          <span key={role} className="px-2 py-0.5 rounded bg-surface-800 text-xs text-surface-400 font-mono border border-surface-700">
                            {role}
                          </span>
                        ))}
                      </div>
                    ) : (
                      <span className="text-xs text-surface-600">\u2014</span>
                    )}
                  </td>
                  <td className="px-6 py-4 text-right">
                    <button onClick={() => handleDelete(k.key, k.name)}
                      className="p-2 rounded-lg text-surface-500 hover:text-red-400 hover:bg-red-900/20 transition-colors"
                      title="Delete key">
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Security Info */}
      <div className="card">
        <h2 className="card-header flex items-center gap-2">
          <Shield size={14} /> Security Notes
        </h2>
        <div className="space-y-2 text-sm text-surface-400">
          <p>API keys authenticate requests to the AI Gateway. Keys are stored in the gateway config file.</p>
          <p className="text-xs text-surface-500">Tip: Use the demo key <code className="text-accent-400 bg-surface-800 px-1 rounded">sk-gateway-demo-key</code> for development.</p>
        </div>
      </div>

      {/* Add Key Modal */}
      <Modal open={showAdd} onClose={() => setShowAdd(false)} title="Add API Key">
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Key Name *</label>
            <input className="input w-full" placeholder="e.g. my-app-key" value={newName}
              onChange={(e) => setNewName(e.target.value)} />
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">API Key</label>
            <div className="flex gap-2">
              <input className="input flex-1 font-mono text-xs" placeholder="sk-xxxxxxxx..." value={newKey}
                onChange={(e) => setNewKey(e.target.value)} />
              <button onClick={generateKey} className="btn-ghost text-xs shrink-0">Generate</button>
            </div>
            <p className="text-[10px] text-surface-500 mt-1">Leave blank to auto-generate, or enter a custom key.</p>
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Roles (optional)</label>
            <input className="input w-full" placeholder="e.g. admin, readonly" value={newRoles}
              onChange={(e) => setNewRoles(e.target.value)} />
            <p className="text-[10px] text-surface-500 mt-1">Comma-separated. e.g. <code className="text-accent-400">admin</code>, <code className="text-accent-400">readonly</code></p>
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button onClick={() => setShowAdd(false)} className="btn-ghost text-sm">Cancel</button>
            <button onClick={handleAdd} disabled={saving} className="btn-primary text-sm">
              {saving ? 'Adding...' : 'Add Key'}
            </button>
          </div>
        </div>
      </Modal>
      <ConfirmDialog
        open={confirmTarget !== null}
        onClose={() => setConfirmTarget(null)}
        onConfirm={confirmDelete}
        title="Delete API Key"
        message={`Delete API key "${confirmTarget?.name ?? ''}"? This action cannot be undone and any applications using this key will lose access.`}
        confirmLabel="Delete Key"
        variant="danger"
        loading={confirmLoading}
      />
    </div>
  )
}
