import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { UserPlus, Trash2, Users as UsersIcon } from 'lucide-react'
import { getUsers, createUser, deleteUser, updateUserRole, type UserInfo } from '../api/gateway'

export default function UsersPage() {
  const { t } = useTranslation()
  const [users, setUsers] = useState<UserInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [newUser, setNewUser] = useState({ username: '', password: '', role: 'viewer', team: '' })
  const [createError, setCreateError] = useState<string | null>(null)
  const [createLoading, setCreateLoading] = useState(false)

  const fetch = useCallback(async () => {
    setLoading(true)
    try {
      const u = await getUsers()
      setUsers(u)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load users')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetch() }, [fetch])

  const handleDelete = async (username: string) => {
    if (!confirm(`Delete user "${username}"?`)) return
    try {
      await deleteUser(username)
      setUsers(prev => prev.filter(u => u.username !== username))
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Delete failed')
    }
  }

  const handleRoleChange = async (username: string, role: string) => {
    try {
      await updateUserRole(username, role)
      setUsers(prev => prev.map(u => u.username === username ? { ...u, role } : u))
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Role update failed')
    }
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setCreateError(null)
    setCreateLoading(true)
    try {
      await createUser(newUser.username, newUser.password, newUser.role, newUser.team || undefined)
      setShowCreate(false)
      setNewUser({ username: '', password: '', role: 'viewer', team: '' })
      fetch()
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : 'Create failed')
    } finally {
      setCreateLoading(false)
    }
  }



  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">{t('users.title')}</h1>
          <p className="text-sm text-surface-400 mt-1">{t('users.subtitle')}</p>
        </div>
        <button onClick={() => setShowCreate(true)} className="btn-primary flex items-center gap-2">
          <UserPlus size={16} /> {t('users.add')}
        </button>
      </div>

      {error && <div className="bg-red-900/20 border border-red-800/40 rounded-lg p-3 text-sm text-red-300">{error}</div>}

      {showCreate && (
        <div className="card">
          <h2 className="card-header">Create New User</h2>
          <form onSubmit={handleCreate} className="space-y-3">
            {createError && <div className="bg-red-900/20 border border-red-800/40 rounded-lg p-2 text-sm text-red-300">{createError}</div>}
            <div className="grid grid-cols-4 gap-3">
              <input type="text" placeholder="Username" value={newUser.username}
                onChange={e => setNewUser(p => ({ ...p, username: e.target.value }))} className="input" required />
              <input type="password" placeholder="Password (min 8 chars)" value={newUser.password}
                onChange={e => setNewUser(p => ({ ...p, password: e.target.value }))} className="input" required />
              <select value={newUser.role} onChange={e => setNewUser(p => ({ ...p, role: e.target.value }))} className="input">
                <option value="viewer">Viewer</option>
                <option value="editor">Editor</option>
                <option value="admin">Admin</option>
              </select>
              <input type="text" placeholder="Team (optional)" value={newUser.team}
                onChange={e => setNewUser(p => ({ ...p, team: e.target.value }))} className="input" />
            </div>
            <div className="flex gap-2 justify-end">
              <button type="button" onClick={() => setShowCreate(false)} className="btn-ghost">Cancel</button>
              <button type="submit" disabled={createLoading} className="btn-primary">{createLoading ? 'Creating...' : 'Create'}</button>
            </div>
          </form>
        </div>
      )}

      <div className="card">
        <h2 className="card-header flex items-center gap-2"><UsersIcon size={16} /> All Users ({users.length})</h2>
        {loading ? <div className="text-sm text-surface-500 py-4">Loading...</div> :
          users.length === 0 ? <div className="text-sm text-surface-500 py-4">No users found</div> :
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-xs text-surface-500 border-b border-surface-800">
                  <th className="text-left py-2 pr-4 font-medium">Username</th>
                  <th className="text-left py-2 pr-4 font-medium">Role</th>
                  <th className="text-left py-2 pr-4 font-medium">Team</th>
                  <th className="text-left py-2 pr-4 font-medium">Created</th>
                  <th className="text-right py-2 font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {users.map(u => (
                  <tr key={u.username} className="border-b border-surface-800/50 hover:bg-surface-800/30">
                    <td className="py-2 pr-4 text-white font-medium">{u.username}</td>
                    <td className="py-2 pr-4">
                      <select value={u.role} onChange={e => handleRoleChange(u.username, e.target.value)} className={`text-xs border-0 bg-transparent cursor-pointer ${u.role === 'admin' ? 'text-red-300' : u.role === 'editor' ? 'text-yellow-300' : 'text-surface-300'}`}>
                        <option value="admin">Admin</option>
                        <option value="editor">Editor</option>
                        <option value="viewer">Viewer</option>
                      </select>
                    </td>
                    <td className="py-2 pr-4 text-surface-400">{u.team || '-'}</td>
                    <td className="py-2 pr-4 text-surface-500 text-xs">{u.created_at ? new Date(u.created_at).toLocaleDateString() : '-'}</td>
                    <td className="py-2 text-right">
                      <button onClick={() => handleDelete(u.username)} className="p-1.5 rounded hover:bg-red-900/20 text-surface-500 hover:text-red-400 transition-colors" title="Delete user">
                        <Trash2 size={14} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        }
      </div>
    </div>
  )
}

