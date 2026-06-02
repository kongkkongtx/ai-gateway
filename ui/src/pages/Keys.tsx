import { useState, useEffect, useCallback } from 'react'
import { Key, Plus, Trash2, Copy, Check, Clock, RefreshCw, Shield, Eye, EyeOff, RotateCcw, Calendar } from 'lucide-react'
import { getApiKeys, addApiKey, deleteApiKey, rotateApiKey } from '../api/gateway'
import Modal from '../components/Modal'
import ConfirmDialog from '../components/ConfirmDialog'
import { useToast } from '../components/Toast'
import type { ApiKeyConfig } from '../api/gateway'

export default function KeysPage() {
  const [keys, setKeys] = useState<ApiKeyConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
  const [showAdd, setShowAdd] = useState(false)
  const [newKey, setNewKey] = useState('')
  const [newName, setNewName] = useState('')
  const [newRoles, setNewRoles] = useState('')
  const [newTeam, setNewTeam] = useState('')
  const [newExpires, setNewExpires] = useState('')
  const [saving, setSaving] = useState(false)
  const [confirmTarget, setConfirmTarget] = useState<{key: string; name: string} | null>(null)
  const [confirmLoading, setConfirmLoading] = useState(false)
  const [copiedKey, setCopiedKey] = useState<string | null>(null)
  const [showKey, setShowKey] = useState<string | null>(null)
  const [rotatingKey, setRotatingKey] = useState<string | null>(null)
  const { toast } = useToast()

  const fetchData = useCallback(async () => {
    try {
      const data = await getApiKeys()
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
      const team = newTeam.trim() || undefined
      await addApiKey(newKey.trim(), newName.trim(), roles, team, newExpires || undefined)
      toast('success', 'API Key added', `Key "${newName.trim()}" created`)
      setShowAdd(false)
      setNewKey(''); setNewName(''); setNewRoles(''); setNewTeam(''); setNewExpires('')
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

  const handleRotate = async (key: string) => {
    if (!confirm('Rotate this API key? The old key will be immediately revoked.')) return
    setRotatingKey(key)
    try {
      const result = await rotateApiKey(key)
      toast('success', 'Key rotated', 'New key generated')
      await navigator.clipboard.writeText(result.new_key)
      toast('info', 'New key copied', 'The new key has been copied to clipboard')
      fetchData()
    } catch (err) {
      toast('error', 'Rotation failed', err instanceof Error ? err.message : undefined)
    } finally { setRotatingKey(null) }
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
              <Clock size={10} /> {lastUpdated.toLocaleTimeString()}
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

      <div className="card p-0 overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-surface-800">
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">Name</th>
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">API Key</th>
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">Team</th>
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">Roles</th>
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">Expires</th>
              <th className="text-right text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-surface-800/50">
            {loading ? (
              Array.from({ length: 3 }).map((_, i) => (
                <tr key={i}><td colSpan={6} className="px-6 py-4"><div className="h-8 bg-surface-800 rounded animate-pulse" /></td></tr>
              ))
            ) : keys.length === 0 ? (
              <tr><td colSpan={6} className="px-6 py-12 text-center text-surface-500">
                <Key size={24} className="mx-auto mb-2 opacity-50" />
                <p className="text-sm">No API keys configured</p>
                <p className="text-xs mt-1">Add your first API key to get started</p>
              </td></tr>
            ) : (
              keys.map((k) => (
                <tr key={k.key} className="hover:bg-surface-800/30 transition-colors">
                  <td className="px-6 py-4">
                    <span className="text-sm font-medium text-white">{k.name}</span>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <code className="text-xs text-surface-300 bg-surface-950 px-2 py-1 rounded font-mono">
                        {showKey === k.key ? k.key : k.key.slice(0, 12) + '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022'}
                      </code>
                      <button onClick={() => setShowKey(showKey === k.key ? null : k.key)} className="text-surface-500 hover:text-surface-300" title={showKey === k.key ? 'Hide' : 'Show'}>
                        {showKey === k.key ? <EyeOff size={14} /> : <Eye size={14} />}
                      </button>
                      <button onClick={() => copyToClipboard(k.key)} className="text-surface-500 hover:text-surface-300" title="Copy">
                        {copiedKey === k.key ? <Check size={14} className="text-accent-400" /> : <Copy size={14} />}
                      </button>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <span className="text-xs text-surface-400">{k.team || '\u2014'}</span>
                  </td>
                  <td className="px-6 py-4">
                    {k.roles && k.roles.length > 0 ? (
                      <div className="flex flex-wrap gap-1">
                        {k.roles.map((role) => (
                          <span key={role} className="px-2 py-0.5 rounded bg-surface-800 text-xs text-surface-400 font-mono border border-surface-700">{role}</span>
                        ))}
                      </div>
                    ) : <span className="text-xs text-surface-600">\u2014</span>}
                  </td>
                  <td className="px-6 py-4">
                    {k.expires_at ? (
                      <span className="text-xs text-surface-400 flex items-center gap-1">
                        <Calendar size={12} />
                        {new Date(k.expires_at).toLocaleDateString()}
                      </span>
                    ) : <span className="text-xs text-surface-600">Never</span>}
                    {k.last_rotated && (
                      <div className="text-[10px] text-surface-500 mt-0.5">Rotated: {new Date(k.last_rotated).toLocaleDateString()}</div>
                    )}
                  </td>
                  <td className="px-6 py-4 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <button onClick={() => handleRotate(k.key)} disabled={rotatingKey === k.key}
                        className="p-2 rounded-lg text-surface-500 hover:text-yellow-400 hover:bg-yellow-900/20 transition-colors" title="Rotate key">
                        <RotateCcw size={14} className={rotatingKey === k.key ? 'animate-spin' : ''} />
                      </button>
                      <button onClick={() => handleDelete(k.key, k.name)}
                        className="p-2 rounded-lg text-surface-500 hover:text-red-400 hover:bg-red-900/20 transition-colors" title="Delete key">
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="card">
        <h2 className="card-header flex items-center gap-2"><Shield size={14} /> Security Notes</h2>
        <div className="space-y-2 text-sm text-surface-400">
          <p>API keys authenticate requests to the AI Gateway. Keys are stored in the gateway config file.</p>
          <p className="text-xs text-surface-500">Tip: Use the demo key <code className="text-accent-400 bg-surface-800 px-1 rounded">sk-gateway-demo-key</code> for development.</p>
        </div>
      </div>

      <Modal open={showAdd} onClose={() => setShowAdd(false)} title="Add API Key">
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Key Name *</label>
            <input className="input w-full" placeholder="e.g. my-app-key" value={newName} onChange={(e) => setNewName(e.target.value)} />
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">API Key</label>
            <div className="flex gap-2">
              <input className="input flex-1 font-mono text-xs" placeholder="sk-xxxxxxxx..." value={newKey} onChange={(e) => setNewKey(e.target.value)} />
              <button onClick={generateKey} className="btn-ghost text-xs shrink-0">Generate</button>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-surface-400 mb-1.5">Roles (optional)</label>
              <input className="input w-full" placeholder="admin, readonly" value={newRoles} onChange={(e) => setNewRoles(e.target.value)} />
            </div>
            <div>
              <label className="block text-xs font-medium text-surface-400 mb-1.5">Team (optional)</label>
              <input className="input w-full" placeholder="e.g. engineering" value={newTeam} onChange={(e) => setNewTeam(e.target.value)} />
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Expires At (optional)</label>
            <input type="date" className="input w-full" value={newExpires} onChange={(e) => setNewExpires(e.target.value)} />
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
        message={`Delete API key "${confirmTarget?.name ?? ''}"? This action cannot be undone.`}
        confirmLabel="Delete Key"
        variant="danger"
        loading={confirmLoading}
      />
    </div>
  )
}
