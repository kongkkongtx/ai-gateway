import { ReactNode } from 'react'

interface StatCardProps {
  label: string
  value: string | number
  icon: ReactNode
  trend?: { value: string; positive: boolean }
  loading?: boolean
}

export default function StatCard({ label, value, icon, trend, loading }: StatCardProps) {
  return (
    <div className="card flex items-start gap-4">
      <div className="p-2.5 rounded-lg bg-brand-600/10 text-brand-400 border border-brand-600/20">
        {icon}
      </div>
      <div className="flex-1 min-w-0">
        {loading ? (
          <>
            <div className="h-7 w-20 bg-surface-800 rounded animate-pulse mb-2" />
            <div className="h-3 w-16 bg-surface-800 rounded animate-pulse" />
          </>
        ) : (
          <>
            <div className="stat-value truncate">{value}</div>
            <div className="stat-label">{label}</div>
            {trend && (
              <div className={`text-xs mt-1.5 flex items-center gap-1 ${trend.positive ? 'text-accent-400' : 'text-red-400'}`}>
                <span>{trend.positive ? '↑' : '↓'}</span>
                <span>{trend.value}</span>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
