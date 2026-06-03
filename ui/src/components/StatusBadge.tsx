interface StatusBadgeProps {
  status: 'healthy' | 'unhealthy' | 'unknown'
  pulse?: boolean
}

import { useTranslation } from 'react-i18next'

const styleConfig = {
  healthy: { dot: 'bg-accent-500', bg: 'bg-accent-900/30', text: 'text-accent-400' },
  unhealthy: { dot: 'bg-red-500', bg: 'bg-red-900/30', text: 'text-red-400' },
  unknown: { dot: 'bg-surface-500', bg: 'bg-surface-800', text: 'text-surface-400' },
}

export default function StatusBadge({ status, pulse }: StatusBadgeProps) {
  const { t } = useTranslation()
  const c = styleConfig[status]
  const label = t(`components.${status}`)
  return (
    <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium ${c.bg} ${c.text}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${c.dot} ${pulse ? 'animate-pulse' : ''}`} />
      {label}
    </span>
  )
}
