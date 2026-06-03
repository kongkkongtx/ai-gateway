import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Route, RefreshCw, ArrowRight, Hash, Layers, Clock, Edit3, Save, AlertTriangle, Plus, Trash2 } from 'lucide-react'
import { getAdminRoutes, addSingleRoute, deleteRoute, reloadRoutes } from '../api/gateway'
import Modal from '../components/Modal'
import ConfirmDialog from '../components/ConfirmDialog'
import { useToast } from '../components/Toast'
import type { RouteConfig } from '../api/gateway'

const emptyRoute = (): RouteConfig => ({ id: '', model: '', upstream: '', fallbacks: [], priority: 0 })

export default function RoutesPage() {
  const { t } = useTranslation()
  const [routes, setRoutes] = useState<RouteConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
  const [jsonEditor, setJsonEditor] = useState(false)
  const [editJson, setEditJson] = useState('')
  const [saving, setSaving] = useState(false)
  const [editError, setEditError] = useState<string | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [formData, setFormData] = useState<RouteConfig>(emptyRoute())
  const [editingId, setEditingId] = useState<string | null>(null)
  const [confirmTarget, setConfirmTarget] = useState<string | null>(null)
  const [confirmLoading, setConfirmLoading] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [fallbackInput, setFallbackInput] = useState('')
  const { toast } = useToast()

  const fetchData = useCallback(async () => {
    try {
      const data = await getAdminRoutes()
      setRoutes(data.sort((a, b) => b.priority - a.priority))
      setLastUpdated(new Date())
    } catch {} finally { setLoading(false) }
  }, [])

  useEffect(() => { fetchData(); const t = setInterval(fetchData, 5000); return () => clearInterval(t) }, [fetchData])

  const openJsonEditor = () => {
    setEditJson(JSON.stringify(routes, null, 2))
    setEditError(null)
    setJsonEditor(true)
  }

  const openAddForm = () => {
    setFormData(emptyRoute())
    setEditingId(null)
    setFormError(null)
    setFallbackInput('')
    setShowForm(true)
  }

  const openEditForm = (r: RouteConfig) => {
    setFormData({ ...r, fallbacks: r.fallbacks || [] })
    setEditingId(r.id)
    setFormError(null)
    setFallbackInput('')
    setShowForm(true)
  }

  const addFallback = () => {
    const fb = fallbackInput.trim()
    if (!fb) return
    if (formData.fallbacks?.includes(fb)) { setFallbackInput(''); return }
    if (fb === formData.upstream) { toast('error', 'Validation', 'Fallback cannot be the same as primary upstream'); return }
    setFormData({ ...formData, fallbacks: [...(formData.fallbacks || []), fb] })
    setFallbackInput('')
  }

  const removeFallback = (idx: number) => {
    setFormData({ ...formData, fallbacks: formData.fallbacks?.filter((_, i) => i !== idx) })
  }

  const handleFormSave = async () => {
    setFormError(null)
    if (!formData.id.trim() || !formData.model.trim() || !formData.upstream.trim()) {
      setFormError('ID, model pattern, and upstream are required')
      return
    }
    setSaving(true)
    try {
      await addSingleRoute(formData)
      toast('success', editingId ? 'Route updated' : 'Route added', `Route "${formData.id}" saved`)
      setShowForm(false)
      fetchData()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : 'Failed to save')
    } finally { setSaving(false) }
  }

  const handleDelete = async (id: string) => {
    setConfirmTarget(id)
  }

  const confirmDelete = async () => {
    if (!confirmTarget) return
    setConfirmLoading(true)
    try {
      await deleteRoute(confirmTarget)
      toast('success', 'Route deleted', `Route "${confirmTarget}" removed`)
      setConfirmTarget(null)
      fetchData()
    } catch (err) {
      toast('error', 'Delete failed', err instanceof Error ? err.message : undefined)
    } finally { setConfirmLoading(false) }
  }

  const saveJsonConfig = async () => {
    setSaving(true)
    setEditError(null)
    try {
      const parsed: RouteConfig[] = JSON.parse(editJson)
      if (!Array.isArray(parsed)) throw new Error('Root must be an array')
      for (const r of parsed) {
        if (!r.id || !r.model || !r.upstream) {
          throw new Error('Route missing required field (id, model, or upstream)')
        }
      }
      const result = await reloadRoutes(parsed)
      toast('success', 'Routes updated', `${result.count} rules applied`)
      setJsonEditor(false)
      fetchData()
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Invalid JSON'
      setEditError(msg)
      toast('error', 'Save failed', msg)
    } finally { setSaving(false) }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">{t('routes.title')}</h1>
          <p className="text-sm text-surface-400 mt-1">
            {loading ? t('common.loading') : t('routes.rules_count', { count: routes.length })}
          </p>
        </div>
        <div className="flex items-center gap-3">
          {lastUpdated && (
            <span className="text-[10px] text-surface-500 flex items-center gap-1">
              <Clock size={10} />
              {lastUpdated.toLocaleTimeString()}
            </span>
          )}
          <button onClick={openAddForm} className="btn-primary flex items-center gap-1.5 text-xs">
            <Plus size={14} /> {t('routes.add')}
          </button>
          <button onClick={openJsonEditor} className="btn-ghost flex items-center gap-1.5 text-xs">
            <Edit3 size={14} /> JSON Edit
          </button>
          <button onClick={fetchData} className="btn-ghost flex items-center gap-2">
            <RefreshCw size={14} /> Refresh
          </button>
        </div>
      </div>

      {/* Table */}
      <div className="card p-0 overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-surface-800">
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">ID</th>
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">Model Pattern</th>
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">Upstream</th>
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">Fallbacks</th>
              <th className="text-center text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">Priority</th>
              <th className="text-right text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-surface-800/50">
            {loading ? (
              Array.from({ length: 4 }).map((_, i) => (
                <tr key={i}>
                  {Array.from({ length: 6 }).map((_, j) => (
                    <td key={j} className="px-6 py-4"><div className="h-5 bg-surface-800 rounded animate-pulse" /></td>
                  ))}
                </tr>
              ))
            ) : routes.length === 0 ? (
              <tr><td colSpan={6} className="px-6 py-12 text-center text-sm text-surface-500">{t('routes.no_routes')}</td></tr>
            ) : (
              routes.map((r, idx) => (
                <tr key={r.id} className={`hover:bg-surface-900/50 transition-colors ${idx % 2 === 0 ? 'bg-surface-950/20' : ''}`}>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-3">
                      <div className="w-7 h-7 rounded-lg bg-brand-600/10 border border-brand-600/20 flex items-center justify-center">
                        <Route size={14} className="text-brand-400" />
                      </div>
                      <span className="text-sm text-surface-300 font-mono">{r.id}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <span className="px-2.5 py-1 rounded-md bg-surface-800 text-sm font-mono text-accent-400 border border-accent-700/30">
                      {r.model}
                    </span>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2 text-sm">
                      <ArrowRight size={12} className="text-surface-500 shrink-0" />
                      <span className="text-white font-medium">{r.upstream}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    {r.fallbacks && r.fallbacks.length > 0 ? (
                      <div className="flex flex-wrap gap-1">
                        {r.fallbacks.map((fb, i) => (
                          <span key={i} className="px-2 py-0.5 rounded bg-surface-800 text-xs text-surface-400 font-mono border border-surface-700">
                            {fb}
                          </span>
                        ))}
                      </div>
                    ) : (
                      <span className="text-xs text-surface-600">—</span>
                    )}
                  </td>
                  <td className="px-6 py-4 text-center">
                    <div className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md bg-surface-800 text-xs font-mono text-surface-300">
                      <Hash size={10} />
                      {r.priority}
                    </div>
                  </td>
                  <td className="px-6 py-4 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <button onClick={() => openEditForm(r)}
                        className="p-2 rounded-lg text-surface-500 hover:text-accent-400 hover:bg-accent-900/20 transition-colors"
                        title="Edit route">
                        <Edit3 size={14} />
                      </button>
                      <button onClick={() => handleDelete(r.id)}
                        className="p-2 rounded-lg text-surface-500 hover:text-red-400 hover:bg-red-900/20 transition-colors"
                        title="Delete route">
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

      {/* Add/Edit Route Form Modal */}
      <Modal open={showForm} onClose={() => setShowForm(false)}
        title={editingId ? `Edit Route: ${editingId}` : 'Add Route'}>
        <div className="space-y-4">
          {formError && (
            <div className="flex items-center gap-2 bg-red-900/20 border border-red-800/40 rounded-lg px-4 py-3">
              <AlertTriangle size={14} className="text-red-400 shrink-0" />
              <span className="text-sm text-red-300">{formError}</span>
            </div>
          )}
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Route ID *</label>
            <input className="input w-full font-mono text-xs" placeholder="my-route"
              value={formData.id} disabled={!!editingId}
              onChange={(e) => setFormData({...formData, id: e.target.value})} />
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Model Pattern *</label>
            <input className="input w-full font-mono text-xs" placeholder="gpt-4*"
              value={formData.model}
              onChange={(e) => setFormData({...formData, model: e.target.value})} />
            <p className="text-[10px] text-surface-500 mt-1">Use <code className="text-accent-400">*</code> as wildcard, e.g. <code className="text-accent-400">deepseek-*</code></p>
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Target Upstream *</label>
            <input className="input w-full" placeholder="my-upstream"
              value={formData.upstream}
              onChange={(e) => setFormData({...formData, upstream: e.target.value})} />
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Fallback Upstreams</label>
            <div className="flex gap-2 mb-2">
              <input className="input flex-1 font-mono text-xs" placeholder="backup-upstream"
                value={fallbackInput}
                onChange={(e) => setFallbackInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addFallback() } }} />
              <button onClick={addFallback} className="btn-ghost text-xs shrink-0">Add</button>
            </div>
            {formData.fallbacks && formData.fallbacks.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {formData.fallbacks.map((fb, i) => (
                  <span key={i} className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-surface-800 text-xs text-surface-300 font-mono border border-surface-700">
                    {fb}
                    <button onClick={() => removeFallback(i)} className="text-surface-500 hover:text-red-400">&times;</button>
                  </span>
                ))}
              </div>
            )}
            <p className="text-[10px] text-surface-500 mt-1">Optional. Tried in order when the primary upstream fails.</p>
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Priority</label>
            <input className="input w-full" type="number"
              value={formData.priority}
              onChange={(e) => setFormData({...formData, priority: parseInt(e.target.value) || 0})} />
            <p className="text-[10px] text-surface-500 mt-1">Higher values take precedence when matching models.</p>
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button onClick={() => setShowForm(false)} className="btn-ghost text-sm">Cancel</button>
            <button onClick={handleFormSave} disabled={saving} className="btn-primary flex items-center gap-2 text-sm">
              <Save size={14} /> {saving ? 'Saving...' : editingId ? 'Update Route' : 'Save Route'}
            </button>
          </div>
        </div>
      </Modal>

      {/* JSON Editor Modal */}
      <Modal open={jsonEditor} onClose={() => setJsonEditor(false)} title="Edit Routes Configuration">
        <div className="space-y-4">
          {editError && (
            <div className="flex items-center gap-2 bg-red-900/20 border border-red-800/40 rounded-lg px-4 py-3">
              <AlertTriangle size={14} className="text-red-400 shrink-0" />
              <span className="text-sm text-red-300">{editError}</span>
            </div>
          )}
          <p className="text-xs text-surface-500">
            Edit route rules as JSON array. Supports <code className="text-accent-400 bg-surface-800 px-1 rounded">fallbacks</code> for multi-upstream redundancy.
          </p>
          <textarea className="input w-full font-mono text-xs h-64 resize-y"
            value={editJson}
            onChange={(e) => setEditJson(e.target.value)}
            spellCheck={false} />
          <div className="flex justify-end gap-3">
            <button onClick={() => setJsonEditor(false)} className="btn-ghost text-sm">Cancel</button>
            <button onClick={saveJsonConfig} disabled={saving} className="btn-primary flex items-center gap-2 text-sm">
              <Save size={14} /> {saving ? 'Saving...' : 'Save Changes'}
            </button>
          </div>
        </div>
      </Modal>

      {/* Routing Legend */}
      <div className="card">
        <h2 className="card-header flex items-center gap-2">
          <Layers size={14} /> Route Matching
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 text-sm">
          <div className="space-y-2">
            <div className="text-xs font-semibold text-surface-400 uppercase">Match Mechanism</div>
            <p className="text-surface-400 leading-relaxed">Routes are matched by model name using glob patterns. Higher priority routes are evaluated first.</p>
          </div>
          <div className="space-y-2">
            <div className="text-xs font-semibold text-surface-400 uppercase">Wildcards</div>
            <p className="text-surface-400 leading-relaxed"><code className="text-accent-400 bg-surface-800 px-1 rounded">*</code> matches any characters, e.g. <code className="text-accent-400 bg-surface-800 px-1 rounded">gpt-4*</code></p>
          </div>
          <div className="space-y-2">
            <div className="text-xs font-semibold text-surface-400 uppercase">Fallbacks</div>
            <p className="text-surface-400 leading-relaxed">When the primary upstream fails, fallbacks are tried in order for automatic failover.</p>
          </div>
        </div>
      </div>
      <ConfirmDialog
        open={confirmTarget !== null}
        onClose={() => setConfirmTarget(null)}
        onConfirm={confirmDelete}
        title="Delete Route"
        message={`Delete route "${confirmTarget ?? ''}"? This will remove the routing rule permanently.`}
        confirmLabel="Delete"
        variant="danger"
        loading={confirmLoading}
      />
    </div>
  )
}
