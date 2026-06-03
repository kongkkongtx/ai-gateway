import { useState, useEffect, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { Plus, FileText, Trash2, History, RefreshCw, Code } from "lucide-react"
import { getPrompts, savePrompt, deletePrompt, addPromptVersion } from "../api/gateway"
import Modal from "../components/Modal"
import ConfirmDialog from "../components/ConfirmDialog"
import { useToast } from "../components/Toast"

interface PromptTemplate {
  id: string
  name: string
  description?: string
  role?: string
  route_match?: string
  variables?: { name: string; default?: string; desc?: string }[]
  current_version: number
  versions?: {
    version: number
    content: string
    variables?: { name: string; default?: string; desc?: string }[]
    created_at: string
    created_by?: string
    comment?: string
  }[]
  created_at: string
  updated_at: string
}

export default function PromptsPage() {
  const { t } = useTranslation()
  const [templates, setTemplates] = useState<PromptTemplate[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showAdd, setShowAdd] = useState(false)
  const [showVersion, setShowVersion] = useState<PromptTemplate | null>(null)
  const [newVersion, setNewVersion] = useState("")
  const [newVersionComment, setNewVersionComment] = useState("")
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [editTemplate, setEditTemplate] = useState<PromptTemplate | null>(null)
  const [formName, setFormName] = useState("")
  const [formDesc, setFormDesc] = useState("")
  const [formRole, setFormRole] = useState("system")
  const [formRoute, setFormRoute] = useState("")
  const [formVars, setFormVars] = useState("")
  const { toast } = useToast()

  const fetchData = useCallback(async (silent = false) => {
    if (!silent) setLoading(true)
    setError(null)
    try {
      const data = await getPrompts()
      setTemplates(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to fetch")
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [])

  useEffect(() => { fetchData() }, [fetchData])

  const openAdd = () => {
    setEditTemplate(null)
    setFormName("")
    setFormDesc("")
    setFormRole("system")
    setFormRoute("")
    setFormVars("")
    setShowAdd(true)
  }

  const openEdit = (t: PromptTemplate) => {
    setEditTemplate(t)
    setFormName(t.name)
    setFormDesc(t.description || "")
    setFormRole(t.role || "system")
    setFormRoute(t.route_match || "")
    setFormVars((t.variables || []).map((v) => v.name + (v.default ? "=" + v.default : "")).join("\n"))
    setShowAdd(true)
  }

  const handleSave = async () => {
    if (!formName.trim()) { toast("error", "Validation", "Name is required"); return }
    setSaving(true)
    try {
      const variables = formVars.trim()
        ? formVars.split("\n").filter(Boolean).map((line) => {
            const parts = line.split("=")
            return { name: parts[0].trim(), default: parts[1]?.trim() || "" }
          })
        : []

      if (editTemplate) {
        editTemplate.name = formName
        editTemplate.description = formDesc
        editTemplate.role = formRole
        editTemplate.route_match = formRoute
        editTemplate.variables = variables
        await savePrompt(editTemplate)
        toast("success", "Saved", "Template updated")
      } else {
        await savePrompt({
          name: formName,
          description: formDesc,
          role: formRole,
          route_match: formRoute,
          variables: variables,
        } as PromptTemplate)
        toast("success", "Created", "New template created")
      }
      setShowAdd(false)
      fetchData(true)
    } catch (err) {
      toast("error", "Error", String(err))
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await deletePrompt(id)
      toast("success", "Deleted", "Template removed")
      setConfirmDelete(null)
      fetchData(true)
    } catch (err) {
      toast("error", "Error", String(err))
    }
  }

  const handleAddVersion = async () => {
    if (!showVersion || !newVersion.trim()) return
    setSaving(true)
    try {
      await addPromptVersion(showVersion.id, newVersion, newVersionComment)
      toast("success", "Version Added", `Version ${showVersion.current_version + 1} created`)
      setNewVersion("")
      setNewVersionComment("")
      fetchData(true)
    } catch (err) {
      toast("error", "Error", String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">{t('prompts.title')}</h1>
          <p className="text-sm text-surface-400 mt-1">Manage system prompt templates with versioning and variable injection</p>
        </div>
        <div className="flex items-center gap-3">
          <button onClick={() => { setRefreshing(true); fetchData(true) }} disabled={refreshing} className="btn-ghost flex items-center gap-2">
            <RefreshCw size={14} className={refreshing ? "animate-spin" : ""} /> Refresh
          </button>
          <button onClick={openAdd} className="btn-primary flex items-center gap-2 text-sm">
            <Plus size={14} /> New Template
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-900/20 border border-red-800/40 rounded-xl p-4 text-sm text-red-300">{error}</div>
      )}

      <div className="card">
        {loading ? (
          <div className="space-y-3">
            {Array.from({ length: 3 }).map((_, i) => <div key={i} className="h-16 bg-surface-800 rounded-lg animate-pulse" />)}
          </div>
        ) : templates.length === 0 ? (
          <div className="flex flex-col items-center py-16 text-surface-500">
            <FileText size={40} className="mb-3 opacity-30" />
            <p className="text-sm">No prompt templates yet</p>
            <p className="text-xs mt-1">Create your first template to manage system prompts centrally</p>
          </div>
        ) : (
          <div className="space-y-2">
            {templates.map((t) => (
              <div key={t.id} className="flex items-center justify-between py-3 px-4 rounded-lg bg-surface-950/50 border border-surface-800/50 hover:border-surface-700 transition-colors">
                <div className="flex items-center gap-3 min-w-0 flex-1">
                  <div className="w-8 h-8 rounded-lg bg-brand-600/10 border border-brand-600/20 flex items-center justify-center shrink-0">
                    <FileText size={14} className="text-brand-400" />
                  </div>
                  <div className="min-w-0">
                    <div className="text-sm font-medium text-white truncate">{t.name}</div>
                    <div className="flex items-center gap-2 mt-0.5">
                      {(t.description) && <span className="text-xs text-surface-500 truncate">{t.description}</span>}
                      <span className="badge-neutral text-[10px]">v{t.current_version}</span>
                      {t.role && <span className="badge-neutral text-[10px]">{t.role}</span>}
                      {t.route_match && <span className="badge-neutral text-[10px] font-mono">{t.route_match}</span>}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2 shrink-0 ml-4">
                  <button onClick={() => setShowVersion(t)} className="btn-ghost p-1.5" title="Manage versions">
                    <History size={14} />
                  </button>
                  <button onClick={() => openEdit(t)} className="btn-ghost p-1.5" title="Edit template">
                    <Code size={14} />
                  </button>
                  <button onClick={() => setConfirmDelete(t.id)} className="btn-ghost p-1.5 text-red-400 hover:text-red-300" title="Delete">
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Add/Edit Modal */}
      <Modal open={showAdd} onClose={() => setShowAdd(false)} title={editTemplate ? "Edit Template" : "New Template"}>
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Name *</label>
            <input className="input w-full" value={formName} onChange={(e) => setFormName(e.target.value)} placeholder="e.g. customer-support-agent" />
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">Description</label>
            <input className="input w-full" value={formDesc} onChange={(e) => setFormDesc(e.target.value)} placeholder="Brief description of this template" />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-surface-400 mb-1.5">Role</label>
              <select className="input w-full" value={formRole} onChange={(e) => setFormRole(e.target.value)}>
                <option value="system">system</option>
                <option value="user">user</option>
                <option value="assistant">assistant</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-surface-400 mb-1.5">Route Match (glob)</label>
              <input className="input w-full font-mono text-xs" value={formRoute} onChange={(e) => setFormRoute(e.target.value)} placeholder="gpt-4*" />
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium text-surface-400 mb-1.5">
              Variables <span className="text-surface-600 font-normal">(one per line, e.g. name=AI Assistant)</span>
            </label>
            <textarea className="input w-full font-mono text-xs h-20" value={formVars} onChange={(e) => setFormVars(e.target.value)} placeholder="company_name=Acme Corp&#10;tone=friendly" />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button onClick={() => setShowAdd(false)} className="btn-ghost text-sm">Cancel</button>
            <button onClick={handleSave} disabled={saving} className="btn-primary text-sm">{saving ? "Saving..." : "Save"}</button>
          </div>
        </div>
      </Modal>

      {/* Version Management Modal */}
      <Modal open={!!showVersion} onClose={() => setShowVersion(null)} title={showVersion ? `Versions: ${showVersion.name}` : ""}>
        {showVersion && (
          <div className="space-y-4">
            <div className="text-xs text-surface-400">Current version: v{showVersion.current_version}</div>
            <div>
              <label className="block text-xs font-medium text-surface-400 mb-1.5">New Version Content *</label>
              <textarea className="input w-full font-mono text-xs h-32" value={newVersion} onChange={(e) => setNewVersion(e.target.value)} placeholder="Your system prompt content with {{variable}} placeholders..." />
            </div>
            <div>
              <label className="block text-xs font-medium text-surface-400 mb-1.5">Change Comment</label>
              <input className="input w-full" value={newVersionComment} onChange={(e) => setNewVersionComment(e.target.value)} placeholder="What changed in this version?" />
            </div>
            <button onClick={handleAddVersion} disabled={saving || !newVersion.trim()} className="btn-primary text-sm w-full">{saving ? "Creating..." : "Create Version"}</button>

            {showVersion.versions && showVersion.versions.length > 0 && (
              <div className="border-t border-surface-800 pt-4 mt-4">
                <p className="text-xs font-medium text-surface-400 mb-3">Version History</p>
                <div className="space-y-2 max-h-60 overflow-y-auto">
                  {[...showVersion.versions].reverse().map((v) => (
                    <div key={v.version} className="p-3 rounded-lg bg-surface-950/50 border border-surface-800/50">
                      <div className="flex items-center justify-between mb-1">
                        <span className="text-xs font-medium text-brand-400">v{v.version}</span>
                        <span className="text-[10px] text-surface-500">{new Date(v.created_at).toLocaleString()}</span>
                      </div>
                      {v.comment && <p className="text-xs text-surface-400 mb-1">{v.comment}</p>}
                      <pre className="text-[10px] text-surface-500 font-mono whitespace-pre-wrap line-clamp-3">{v.content}</pre>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </Modal>

      <ConfirmDialog
        open={!!confirmDelete}
        title="Delete Template"
        message="Are you sure you want to delete this prompt template? This action cannot be undone."
        confirmLabel="Delete"
        onConfirm={() => { if (confirmDelete) handleDelete(confirmDelete) }}
        onClose={() => setConfirmDelete(null)}
        loading={false}
      />
    </div>
  )
}



