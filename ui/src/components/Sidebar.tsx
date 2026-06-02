import { NavLink, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  LayoutDashboard, Network, Route, Terminal, Key, Sliders,
  ScrollText, FileText, Globe, Shield, Users, LogOut,
} from 'lucide-react'

interface SidebarProps {
  onLogout?: () => void
}

export default function Sidebar({ onLogout }: SidebarProps) {
  const location = useLocation()
  const { t, i18n } = useTranslation()
  const username = localStorage.getItem('ai-gateway-user') || 'User'

  const navItems = [
    { to: '/', icon: LayoutDashboard, label: t('nav.dashboard') },
    { to: '/upstreams', icon: Network, label: t('nav.upstreams') },
    { to: '/routes', icon: Route, label: t('nav.routes') },
    { to: '/keys', icon: Key, label: t('nav.keys') },
    { to: '/playground', icon: Terminal, label: t('nav.playground') },
    { to: '/users', icon: Users, label: 'Users' },
    { to: '/security', icon: Shield, label: 'Security' },
    { to: '/audit-logs', icon: ScrollText, label: t('nav.audit') },
    { to: '/prompts', icon: FileText, label: t('nav.prompts') },
    { to: '/settings', icon: Sliders, label: t('nav.settings') },
  ]

  const toggleLang = () => {
    const next = i18n.language === 'zh-CN' ? 'en-US' : 'zh-CN'
    i18n.changeLanguage(next)
    localStorage.setItem('ai-gateway-lang', next)
  }

  return (
    <aside className="w-64 bg-surface-900 border-r border-surface-800 flex flex-col h-screen shrink-0">
      <div className="h-16 px-6 flex items-center gap-3 border-b border-surface-800">
        <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-brand-500 to-brand-700 flex items-center justify-center">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
            <path d="M2 17l10 5 10-5"/>
            <path d="M2 12l10 5 10-5"/>
            <circle cx="12" cy="12" r="2" fill="#818cf8" stroke="none"/>
          </svg>
        </div>
        <div>
          <div className="text-sm font-semibold text-white">{t('app.title')}</div>
          <div className="text-[10px] text-surface-500">{t('app.subtitle')}</div>
        </div>
      </div>

      <nav className="flex-1 px-3 py-4 space-y-1 overflow-y-auto">
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

      <div className="px-4 py-4 border-t border-surface-800 space-y-3">
        <div className="flex items-center gap-2 px-2 text-xs text-surface-400">
          <div className="w-6 h-6 rounded-full bg-brand-600/20 flex items-center justify-center text-brand-400 font-medium text-xs">
            {username[0]?.toUpperCase()}
          </div>
          <span className="truncate">{username}</span>
        </div>
        <button onClick={toggleLang} className="flex items-center gap-2 w-full px-3 py-2 rounded-lg text-xs text-surface-400 hover:text-white hover:bg-surface-800 transition-colors">
          <Globe size={14} />
          <span>{t('lang.switch')}</span>
        </button>
        {onLogout && (
          <button onClick={onLogout} className="flex items-center gap-2 w-full px-3 py-2 rounded-lg text-xs text-surface-400 hover:text-red-400 hover:bg-red-900/10 transition-colors">
            <LogOut size={14} />
            <span>Sign Out</span>
          </button>
        )}
        <div className="flex items-center gap-2 text-xs text-surface-500">
          <div className="w-2 h-2 rounded-full bg-accent-500" />
          <span>v2.2</span>
        </div>
      </div>
    </aside>
  )
}
