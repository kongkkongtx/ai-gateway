import { useState, useCallback, createContext, useContext, type ReactNode } from 'react'
import { X, CheckCircle, AlertTriangle, Info, AlertOctagon } from 'lucide-react'

type ToastType = 'success' | 'error' | 'warning' | 'info'

interface Toast {
  id: number
  type: ToastType
  title: string
  message?: string
}

interface ToastContextValue {
  toast: (type: ToastType, title: string, message?: string) => void
}

const ToastContext = createContext<ToastContextValue>({ toast: () => {} })

export const useToast = () => useContext(ToastContext)

const ICONS: Record<ToastType, { icon: typeof Info; bg: string; border: string; iconColor: string }> = {
  success: { icon: CheckCircle, bg: 'bg-accent-900/20', border: 'border-accent-800/40', iconColor: 'text-accent-400' },
  error:   { icon: AlertOctagon, bg: 'bg-red-900/20', border: 'border-red-800/40', iconColor: 'text-red-400' },
  warning: { icon: AlertTriangle, bg: 'bg-yellow-900/20', border: 'border-yellow-800/40', iconColor: 'text-yellow-400' },
  info:    { icon: Info, bg: 'bg-brand-900/20', border: 'border-brand-800/40', iconColor: 'text-brand-400' },
}

let nextId = 0

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])

  const addToast = useCallback((type: ToastType, title: string, message?: string) => {
    const id = nextId++
    setToasts((prev) => [...prev, { id, type, title, message }])
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, 5000)
  }, [])

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  return (
    <ToastContext.Provider value={{ toast: addToast }}>
      {children}
      {/* Toast container */}
      <div className="fixed top-4 right-4 z-50 flex flex-col gap-2 max-w-sm w-full pointer-events-none">
        {toasts.map((t) => {
          const style = ICONS[t.type]
          const Icon = style.icon
          return (
            <div
              key={t.id}
              className={`${style.bg} ${style.border} border rounded-xl p-4 pointer-events-auto animate-slide-in shadow-xl backdrop-blur-sm`}
            >
              <div className="flex items-start gap-3">
                <Icon size={18} className={`${style.iconColor} shrink-0 mt-0.5`} />
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-white">{t.title}</p>
                  {t.message && <p className="text-xs text-surface-400 mt-0.5">{t.message}</p>}
                </div>
                <button onClick={() => removeToast(t.id)} className="text-surface-500 hover:text-surface-300 shrink-0">
                  <X size={14} />
                </button>
              </div>
            </div>
          )
        })}
      </div>
    </ToastContext.Provider>
  )
}
