import { AlertTriangle, X } from 'lucide-react'

interface ConfirmDialogProps {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  variant?: 'danger' | 'warning' | 'info'
  loading?: boolean
}

const VARIANT_STYLES = {
  danger: {
    icon: 'bg-red-900/20 border-red-800/40 text-red-400',
    button: 'bg-red-600 hover:bg-red-500 text-white',
    iconComponent: AlertTriangle,
  },
  warning: {
    icon: 'bg-yellow-900/20 border-yellow-800/40 text-yellow-400',
    button: 'bg-yellow-600 hover:bg-yellow-500 text-white',
    iconComponent: AlertTriangle,
  },
  info: {
    icon: 'bg-brand-600/10 border-brand-600/20 text-brand-400',
    button: 'bg-brand-600 hover:bg-brand-500 text-white',
    iconComponent: AlertTriangle,
  },
}

export default function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  variant = 'danger',
  loading = false,
}: ConfirmDialogProps) {
  if (!open) return null

  const style = VARIANT_STYLES[variant]
  const Icon = style.iconComponent

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center">
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div className="relative bg-surface-900 border border-surface-700 rounded-xl shadow-2xl w-full max-w-md mx-4">
        <div className="p-6">
          <div className="flex items-start gap-4">
            <div className={`${style.icon} p-2.5 rounded-lg border shrink-0`}>
              <Icon size={20} />
            </div>
            <div className="flex-1 min-w-0">
              <h3 className="text-base font-semibold text-white">{title}</h3>
              <p className="text-sm text-surface-400 mt-1.5 leading-relaxed">{message}</p>
            </div>
            <button onClick={onClose} className="text-surface-500 hover:text-surface-300 shrink-0">
              <X size={16} />
            </button>
          </div>
        </div>
        <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-surface-800">
          <button onClick={onClose} disabled={loading} className="btn-ghost text-sm">
            {cancelLabel}
          </button>
          <button onClick={onConfirm} disabled={loading} className={`${style.button} px-4 py-2 rounded-lg text-sm font-medium transition-all`}>
            {loading ? 'Processing...' : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}


