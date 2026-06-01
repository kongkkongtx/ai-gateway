import { Routes, Route } from 'react-router-dom'
import { ToastProvider } from './components/Toast'
import ErrorBoundary from './components/ErrorBoundary'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import UpstreamsPage from './pages/Upstreams'
import RoutesPage from './pages/Routes'
import KeysPage from './pages/Keys'
import SettingsPage from './pages/Settings'
import AuditLogsPage from './pages/AuditLogs'
import PromptsPage from './pages/Prompts'
import Playground from './pages/Playground'

export default function App() {
  return (
    <ErrorBoundary>
      <ToastProvider>
        <Layout>
          <Routes>
            <Route path="/" element={<ErrorBoundary><Dashboard /></ErrorBoundary>} />
            <Route path="/upstreams" element={<ErrorBoundary><UpstreamsPage /></ErrorBoundary>} />
            <Route path="/routes" element={<ErrorBoundary><RoutesPage /></ErrorBoundary>} />
            <Route path="/keys" element={<ErrorBoundary><KeysPage /></ErrorBoundary>} />
            <Route path="/settings" element={<ErrorBoundary><SettingsPage /></ErrorBoundary>} />
            <Route path="/audit-logs" element={<ErrorBoundary><AuditLogsPage /></ErrorBoundary>} />
            <Route path="/prompts" element={<ErrorBoundary><PromptsPage /></ErrorBoundary>} />
            <Route path="/playground" element={<ErrorBoundary><Playground /></ErrorBoundary>} />
          </Routes>
        </Layout>
      </ToastProvider>
    </ErrorBoundary>
  )
}
