import { useState, useEffect, useCallback } from 'react'
'react-i18next'
import { useTranslation } from 'react-i18next'
import { RefreshCw, Server, Zap, Activity, ExternalLink, Clock, Plus, Trash2, Edit3, Save, AlertTriangle } from 'lucide-react'
import { getUpstreams, addSingleUpstream, deleteUpstream } from '../api/gateway'
import StatusBadge from '../components/StatusBadge'
import Modal from '../components/Modal'
import ConfirmDialog from '../components/ConfirmDialog'
import { useToast } from '../components/Toast'
import type { UpstreamStatus } from '../types'
import type { UpstreamConfig } from '../api/gateway'

const PROVIDERS = ['openai', 'anthropic', 'azure', 'google']

const emptyForm = (): UpstreamConfig => ({
  name: '', endpoint: '', provider: 'openai', weight: 1, api_token: '', timeout: '30s', model: '',
})

export default function UpstreamsPage() {
  const [upstreams, setUpstreams] = useState<UpstreamStatus[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [showEditor, setShowEditor] = useState(false)
  const [editForm, setEditForm] = useState<UpstreamConfig>(emptyForm())
  const [editingName, setEditingName] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [confirmTarget, setConfirmTarget] = useState<string | null>(null)
  const [confirmLoading, setConfirmLoading] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const { t } = useTranslation()
  const { toast } = useToast()

  const fetchData = useCallback(async (silent = false) => {
    if (!silent) setLoading(true)
    try {
      const s = await getUpstreams()
      setUpstreams(s)
      setLastUpdated(new Date())
    } catch {} finally { setLoading(false); setRefreshing(false) }
  }, [])

  useEffect(() => {
    fetchData()
    if (!autoRefresh) return
    const t = setInterval(() => fetchData(true), 5000)
    return () => clearInterval(t)
  }, [fetchData, autoRefresh])

  const openAdd = () => {
    setEditForm(emptyForm())
    setEditingName(null)
    setFormError(null)
    setShowEditor(true)
  }

  const openEdit = (u: UpstreamStatus) => {
    setEditForm({
      name: u.name, endpoint: u.endpoint, provider: u.provider, weight: u.weight,
      api_token: '', timeout: '30s', model: '',
    })
    setEditingName(u.name)
    setFormError(null)
    setShowEditor(true)
  }

  const handleSave = async () => {
    setFormError(null)
    if (!editForm.name.trim() || !editForm.endpoint.trim() || !editForm.provider.trim()) {
      setFormError('Name, endpoint, and provider are required')
      return
    }
    setSaving(true)
    try {
      await addSingleUpstream(editForm)
      toast('success', 'Upstream saved', `"${editForm.name}" has been ${editingName ? 'updated' : 'added'}`)
      setShowEditor(false)
      fetchData()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : 'Failed to save')
      toast('error', 'Save failed', err instanceof Error ? err.message : undefined)
    } finally { setSaving(false) }
  }

  const handleDelete = (name: string) => {
    setConfirmTarget(name)
  }

  const confirmDelete = async () => {
    if (!confirmTarget) return
    setConfirmLoading(true)
    try {
      await deleteUpstream(confirmTarget)
      toast('success', 'Upstream deleted', `"${confirmTarget}" removed`)
      setConfirmTarget(null)
      fetchData()
    } catch (err) {
      toast('error', 'Delete failed', err instanceof Error ? err.message : undefined)
    } finally { setConfirmLoading(false) }
  }

  const healthyCount = upstreams.filter((u) => u.healthy).length

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">{t("upstreams.title")}</h1>
          <p className="text-sm text-surface-400 mt-1">
            {loading ? t('common.loading') : `${healthyCount}/${upstreams.length} healthy`}
          </p>
        </div>
        <div className="flex items-center gap-3">
          {lastUpdated && (
            <span className="text-[10px] text-surface-500 flex items-center gap-1">
              <Clock size={10} />
              {lastUpdated.toLocaleTimeString()}
            </span>
          )}
          <button onClick={openAdd} className="btn-primary flex items-center gap-1.5 text-xs">
            <Plus size={14} /> {t("upstreams.add")}
          </button>
          <button onClick={() => { setAutoRefresh(!autoRefresh) }}
            className={`text-xs px-2 py-1 rounded-md transition-colors ${
              autoRefresh ? 'bg-accent-900/30 text-accent-400' : 'bg-surface-800 text-surface-500'
            }`}>Auto</button>
          <button onClick={() => { setRefreshing(true); fetchData(false) }} disabled={refreshing}
            className="btn-ghost flex items-center gap-2">
            <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} /> {t("common.refresh")}
          </button>
        </div>
      </div>

      {/* Table */}
      <div className="card p-0 overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-surface-800">
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">{t("upstreams.table_upstream")}</th>
              <th className="text-left text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">{t("upstreams.table_endpoint")}</th>
              <th className="text-center text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">{t("upstreams.table_provider")}</th>
              <th className="text-center text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">{t("upstreams.table_weight")}</th>
              <th className="text-center text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">{t("upstreams.table_conns")}</th>
              <th className="text-center text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">{t("upstreams.table_status")}</th>
              <th className="text-right text-xs font-semibold text-surface-400 uppercase tracking-wider px-6 py-4">{t("upstreams.table_actions")}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-surface-800/50">
            {loading ? (
              Array.from({ length: 2 }).map((_, i) => (
                <tr key={i}>
                  {Array.from({ length: 7 }).map((_, j) => (
                    <td key={j} className="px-6 py-4"><div className="h-5 bg-surface-800 rounded animate-pulse" /></td>
                  ))}
                </tr>
              ))
            ) : upstreams.length === 0 ? (
              <tr><td colSpan={7} className="px-6 py-12 text-center text-sm text-surface-500">{t("upstreams.no_upstreams")}</td></tr>
            ) : (
              upstreams.map((u, idx) => (
                <tr key={u.name} className={`hover:bg-surface-900/50 transition-colors ${idx % 2 === 0 ? 'bg-surface-950/20' : ''}`}>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-3">
                      <div className={`p-2 rounded-lg border ${u.healthy ? 'bg-brand-600/10 text-brand-400 border-brand-600/20' : 'bg-red-900/10 text-red-400 border-red-800/20'}`}>
                        <Server size={16} />
                      </div>
                      <div>
                        <div className="text-sm font-medium text-white">{u.name}</div>
                        <div className="text-[10px] text-surface-500 font-mono">{u.provider}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-1.5 text-sm text-surface-300 font-mono">
                      <span className="truncate max-w-[200px]">{u.endpoint}</span>
                      <ExternalLink size={12} className="text-surface-500 shrink-0" />
                    </div>
                  </td>
                  <td className="px-6 py-4 text-center"><span className="badge-neutral text-xs">{u.provider}</span></td>
                  <td className="px-6 py-4">
                    <div className="flex items-center justify-center gap-1 text-sm">
                      <Zap size={12} className="text-yellow-500" />
                      <span className="text-white font-medium">{u.weight}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-center">
                    <div className="flex items-center justify-center gap-1 text-sm">
                      <Activity size={12} className={u.active_conns > 0 ? 'text-brand-400' : 'text-surface-600'} />
                      <span className={u.active_conns > 0 ? 'text-surface-300' : 'text-surface-600'}>{u.active_conns ?? 0}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-center">
                    <StatusBadge status={u.healthy ? 'healthy' : 'unhealthy'} pulse />
                  </td>
                  <td className="px-6 py-4 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <button onClick={() => openEdit(u)} className="p-2 rounded-lg text-surface-500 hover:text-accent-400 hover:bg-accent-900/20 transition-colors" title={t("common.edit", "Edit")}><Edit3 size={14} /></button>
                      <button onClick={() => handleDelete(u.name)} className="p-2 rounded-lg text-surface-500 hover:text-red-400 hover:bg-red-900/20 transition-colors" title={t("common.delete")}><Trash2 size={14} /></button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Add/Edit Modal */}
      <Modal open={showEditor} onClose={() => setShowEditor(false)}
        title={editingName ? `Edit Upstream: ${editingName}` : 'Add Upstream'}>
        <div className="space-y-4">
          {formError && (
            <div className="flex items-center gap-2 bg-red-900/20 border border-red-800/40 rounded-lg px-4 py-3">
              <AlertTriangle size={14} className="text-red-400 shrink-0" />
              <span className="text-sm text-red-300">{formError}</span>
            </div>
          )}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-surface-400 mb-1.5">{t("upstreams.field_name")}</label>
              <input className="input w-full" placeholder="my-upstream"
                value={editForm.name} disabled={!!editingName}
                onChange={(e) => setEditForm({...editForm, name: e.target.value})} />
            </div>
            <div>
              <label className="block text-xs font-medium text-surface-400 mb-1.5">{t("upstreams.field_provider")}</label>
              <select className="input w-full" value={editForm.provider}
                onChange={(e) => setEditForm({...editForm, provider: e.target.value})}>
                {PROVIDERS.map((p) => <option key={p} value={p}>{p}</option>)}
              </select>
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">{t("upstreams.field_endpoint")}</label>
            <input className="input w-full font-mono text-xs" placeholder="https://api.openai.com"
              value={editForm.endpoint}
              onChange={(e) => setEditForm({...editForm, endpoint: e.target.value})} />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-surface-400 mb-1.5">{t("upstreams.field_api_token")}</label>
              <input className="input w-full font-mono text-xs" type="password" placeholder="sk-..."
                value={editForm.api_token || ''}
                onChange={(e) => setEditForm({...editForm, api_token: e.target.value})} />
            </div>
            <div>
              <label className="block text-xs font-medium text-surface-400 mb-1.5">{t("upstreams.field_model")}</label>
              <input className="input w-full font-mono text-xs" placeholder="gpt-4o"
                value={editForm.model || ''}
                onChange={(e) => setEditForm({...editForm, model: e.target.value})} />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-surface-400 mb-1.5">{t("upstreams.field_weight")}</label>
              <input className="input w-full" type="number" min="1"
                value={editForm.weight}
                onChange={(e) => setEditForm({...editForm, weight: parseInt(e.target.value) || 1})} />
            </div>
            <div>
              <label className="block text-xs font-medium text-surface-400 mb-1.5">{t("upstreams.field_timeout")}</label>
              <input className="input w-full font-mono text-xs" placeholder="30s"
                value={editForm.timeout || '30s'}
                onChange={(e) => setEditForm({...editForm, timeout: e.target.value})} />
            </div>
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button onClick={() => setShowEditor(false)} className="btn-ghost text-sm">{t("common.cancel")}</button>
            <button onClick={handleSave} disabled={saving} className="btn-primary flex items-center gap-2 text-sm">
              <Save size={14} /> {saving ? t("upstreams.saving") : t("common.save")}
            </button>
          </div>
        </div>
      </Modal>
      <ConfirmDialog
        open={confirmTarget !== null}
        onClose={() => setConfirmTarget(null)}
        onConfirm={confirmDelete}
        title={t("upstreams.delete_title")}
        message={`Delete upstream "${confirmTarget ?? ""}"? Routes referencing this upstream must be updated first.`}
        confirmLabel={t("common.delete")}
        variant="danger"
        loading={confirmLoading}
      />
    </div>
  )
}
