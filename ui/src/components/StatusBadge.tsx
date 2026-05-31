interface StatusBadgeProps {
  status: 'healthy' | 'unhealthy' | 'unknown'
  pulse?: boolean
}

const config = {
  healthy: { dot: 'bg-accent-500', bg: 'bg-accent-900/30', text: 'text-accent-400', label: 'Healthy' },
  unhealthy: { dot: 'bg-red-500', bg: 'bg-red-900/30', text: 'text-red-400', label: 'Unhealthy' },
  unknown: { dot: 'bg-surface-500', bg: 'bg-surface-800', text: 'text-surface-400', label: 'Unknown' },
}

export default function StatusBadge({ status, pulse }: StatusBadgeProps) {
  const c = config[status]
  return (
    <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium ${c.bg} ${c.text}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${c.dot} ${pulse ? 'animate-pulse' : ''}`} />
      {c.label}
    </span>
  )
}
