import { Component, type ReactNode, type ErrorInfo } from 'react'
import { AlertTriangle, RefreshCw } from 'lucide-react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

// Use a simple store-based approach instead of hooks since this is a class component
let _errorLabels = {
  title: 'Something went wrong',
  defaultMsg: 'An unexpected error occurred',
  retry: 'Try Again',
}

export function setErrorLabels(labels: { title: string; defaultMsg: string; retry: string }) {
  _errorLabels = labels
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('[ErrorBoundary]', error, errorInfo)
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null })
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback

      return (
        <div className="flex items-center justify-center min-h-[400px]">
          <div className="card max-w-md w-full text-center">
            <div className="flex justify-center mb-4">
              <div className="p-3 rounded-full bg-red-900/20 border border-red-800/40">
                <AlertTriangle size={24} className="text-red-400" />
              </div>
            </div>
            <h2 className="text-lg font-semibold text-white mb-2">{_errorLabels.title}</h2>
            <p className="text-sm text-surface-400 mb-4">
              {this.state.error?.message || _errorLabels.defaultMsg}
            </p>
            <button onClick={this.handleRetry} className="btn-primary flex items-center gap-2 mx-auto">
              <RefreshCw size={14} /> {_errorLabels.retry}
            </button>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}
