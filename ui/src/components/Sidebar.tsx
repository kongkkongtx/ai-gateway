import { NavLink, useLocation } from 'react-router-dom'
import {
  LayoutDashboard,
  Network,
  Route,
  Terminal,
  Key,
  Sliders,
  ScrollText,
  FileText,
} from 'lucide-react'

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/upstreams', icon: Network, label: 'Upstreams' },
  { to: '/routes', icon: Route, label: 'Routes' },
  { to: '/keys', icon: Key, label: 'API Keys' },
  { to: '/playground', icon: Terminal, label: 'Playground' },
  { to: '/audit-logs', icon: ScrollText, label: 'Audit Logs' },
  { to: '/prompts', icon: FileText, label: 'Prompts' },
  { to: '/settings', icon: Sliders, label: 'Settings' },
]

export default function Sidebar() {
  const location = useLocation()

  return (
    <aside className="w-64 bg-surface-900 border-r border-surface-800 flex flex-col h-screen shrink-0">
      {/* Logo */}
      <div className="h-16 px-6 flex items-center gap-3 border-b border-surface-800">
        <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-brand-500 to-brand-700 flex items-center justify-center">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
            <path d="M2 17l10 5 10-5"/>
            <path d="M2 12l10 5 10-5"/>
            <circle cx="12" cy="12" r="2" fill="#818cf8" stroke="none"/>
          </svg>
        </div>
        <div>
          <div className="text-sm font-semibold text-white">AI Gateway</div>
          <div className="text-[10px] text-surface-500">Management Console</div>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-3 py-4 space-y-1">
        {navItems.map(({ to, icon: Icon, label }) => {
          const isActive = location.pathname === to
          return (
            <NavLink
              key={to}
              to={to}
              className={`flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-150 ${
                isActive
                  ? 'bg-brand-600/15 text-brand-400 border border-brand-600/20'
                  : 'text-surface-400 hover:text-white hover:bg-surface-800 border border-transparent'
              }`}
            >
              <Icon size={18} strokeWidth={1.5} />
              {label}
            </NavLink>
          )
        })}
      </nav>

      {/* Footer */}
      <div className="px-4 py-4 border-t border-surface-800">
        <div className="flex items-center gap-2 text-xs text-surface-500">
          <div className="w-2 h-2 rounded-full bg-accent-500" />
          <span>Connected to AI Gateway</span>
        </div>
      </div>
    </aside>
  )
}
