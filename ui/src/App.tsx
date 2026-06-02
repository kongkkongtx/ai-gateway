import { useState } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { ToastProvider } from './components/Toast'
import ErrorBoundary from './components/ErrorBoundary'
import Layout from './components/Layout'
import LoginPage from './pages/Login'
import Dashboard from './pages/Dashboard'
import UpstreamsPage from './pages/Upstreams'
import RoutesPage from './pages/Routes'
import KeysPage from './pages/Keys'
import UsersPage from './pages/Users'
import SettingsPage from './pages/Settings'
import AuditLogsPage from './pages/AuditLogs'
import PromptsPage from './pages/Prompts'
import Playground from './pages/Playground'
import SecurityPoliciesPage from './pages/SecurityPolicies'

export default function App() {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('ai-gateway-token'))

  const handleLogin = (t: string, _username: string, _role: string) => {
    setToken(t)
  }

  const handleLogout = () => {
    localStorage.removeItem('ai-gateway-token')
    localStorage.removeItem('ai-gateway-user')
    localStorage.removeItem('ai-gateway-role')
    setToken(null)
  }

  if (!token) {
    return (
      <ErrorBoundary>
        <LoginPage onLogin={handleLogin} />
      </ErrorBoundary>
    )
  }

  return (
    <ErrorBoundary>
      <ToastProvider>
        <Layout onLogout={handleLogout}>
          <Routes>
            <Route path="/" element={<ErrorBoundary><Dashboard /></ErrorBoundary>} />
            <Route path="/upstreams" element={<ErrorBoundary><UpstreamsPage /></ErrorBoundary>} />
            <Route path="/routes" element={<ErrorBoundary><RoutesPage /></ErrorBoundary>} />
            <Route path="/keys" element={<ErrorBoundary><KeysPage /></ErrorBoundary>} />
            <Route path="/users" element={<ErrorBoundary><UsersPage /></ErrorBoundary>} />
            <Route path="/security" element={<ErrorBoundary><SecurityPoliciesPage /></ErrorBoundary>} />
            <Route path="/settings" element={<ErrorBoundary><SettingsPage /></ErrorBoundary>} />
            <Route path="/audit-logs" element={<ErrorBoundary><AuditLogsPage /></ErrorBoundary>} />
            <Route path="/prompts" element={<ErrorBoundary><PromptsPage /></ErrorBoundary>} />
            <Route path="/playground" element={<ErrorBoundary><Playground /></ErrorBoundary>} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Layout>
      </ToastProvider>
    </ErrorBoundary>
  )
}

