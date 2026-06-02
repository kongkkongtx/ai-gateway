import { useState, useEffect, useRef } from 'react'
import { Lock, User, ArrowRight, Zap, Shield, Activity, Sparkles } from 'lucide-react'
import { login } from '../api/gateway'

interface LoginPageProps {
  onLogin: (token: string, username: string, role: string) => void
}

// Gradient mesh background with subtle movement
function AmbientCanvas() {
  return (
    <div className="fixed inset-0 -z-10 overflow-hidden bg-surface-950">
      {/* Primary gradient orb */}
      <div className="absolute -top-[20%] -left-[10%] w-[60%] h-[80%] rounded-full bg-brand-600/[0.06] blur-[120px] animate-pulse-slow" />
      {/* Secondary accent orb */}
      <div className="absolute -bottom-[20%] -right-[10%] w-[50%] h-[70%] rounded-full bg-accent-500/[0.04] blur-[100px] animate-pulse-slow" style={{ animationDelay: '2s' }} />
      {/* Subtle center glow */}
      <div className="absolute top-1/3 left-1/2 -translate-x-1/2 w-[40%] h-[50%] rounded-full bg-brand-400/[0.03] blur-[150px]" />
      {/* Mesh grid */}
      <div
        className="absolute inset-0 opacity-[0.015]"
        style={{
          backgroundImage: 'radial-gradient(circle at 1px 1px, rgb(148 163 184) 1px, transparent 0)',
          backgroundSize: '48px 48px',
        }}
      />
      {/* Subtle vignette */}
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_center,transparent_30%,rgba(2,6,23,0.7)_100%)]" />
    </div>
  )
}

const stats = [
  { icon: Zap, value: '< 10ms', label: '路由延迟' },
  { icon: Shield, value: '99.9%', label: '服务可用性' },
  { icon: Activity, value: '10K+', label: '日处理请求' },
]

export default function LoginPage({ onLogin }: LoginPageProps) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [focus, setFocus] = useState<'username' | 'password' | null>(null)
  const [mounted, setMounted] = useState(false)
  const formRef = useRef<HTMLFormElement>(null)

  useEffect(() => { setMounted(true) }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const res = await login(username, password)
      localStorage.setItem('ai-gateway-token', res.token)
      localStorage.setItem('ai-gateway-user', res.username)
      localStorage.setItem('ai-gateway-role', res.role)
      onLogin(res.token, res.username, res.role)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-surface-950 overflow-hidden">
      <AmbientCanvas />

      <div className="w-full max-w-[1040px] mx-auto grid lg:grid-cols-2 gap-0 min-h-[640px]">
        {/* Left Panel — Visual Identity */}
        <div
          className={`hidden lg:flex flex-col justify-between p-12 xl:p-16 relative overflow-hidden rounded-l-3xl
            bg-surface-900/40 border border-surface-800/30 border-r-0 backdrop-blur-sm
            transition-all duration-1000 delay-200 ${mounted ? 'opacity-100' : 'opacity-0'}`}
        >
          {/* Accent line decoration */}
          <div className="absolute top-0 left-0 w-1 h-full bg-gradient-to-b from-brand-500 via-accent-500 to-brand-500/20" />

          {/* Logo + tagline */}
          <div>
            <div className="flex items-center gap-3 mb-10">
              <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-brand-500 to-brand-700 flex items-center justify-center shadow-lg shadow-brand-600/20">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M12 2L2 7l10 5 10-5-10-5z" />
                  <path d="M2 17l10 5 10-5" />
                  <path d="M2 12l10 5 10-5" />
                </svg>
              </div>
              <div className="leading-tight">
                <div className="text-base font-bold text-white tracking-tight font-mono">AI Gateway</div>
                <div className="text-[10px] text-surface-500 tracking-widest uppercase">Enterprise AI Proxy</div>
              </div>
            </div>

            <h2 className="text-3xl xl:text-4xl font-bold text-white leading-[1.15] tracking-tight max-w-sm">
              统一管理
              <br />
              <span className="bg-gradient-to-r from-brand-300 via-brand-400 to-accent-400 bg-clip-text text-transparent">
                AI 服务网关
              </span>
            </h2>
            <p className="text-sm text-surface-400/80 leading-relaxed mt-5 max-w-xs">
              智能路由、负载均衡、安全治理。连接多个 AI 提供商，一站式 API 管理平台。
            </p>
          </div>

          {/* Stats row */}
          <div className="space-y-6">
            <div className="h-px w-full bg-gradient-to-r from-surface-700/50 via-surface-600/30 to-transparent" />
            <div className="grid grid-cols-3 gap-6">
              {stats.map(({ icon: Icon, value, label }) => (
                <div key={label} className="space-y-1">
                  <Icon size={14} className="text-brand-400/60" strokeWidth={1.5} />
                  <div className="text-lg font-bold text-white tracking-tight">{value}</div>
                  <div className="text-[11px] text-surface-500">{label}</div>
                </div>
              ))}
            </div>
          </div>

          {/* Bottom tag */}
          <div className="flex items-center gap-2 text-[11px] text-surface-600">
            <Sparkles size={12} className="text-brand-400/50" />
            <span>Gateway v2.2 — Running with DeepSeek</span>
          </div>
        </div>

        {/* Right Panel — Login Form */}
        <div className="flex items-center justify-center p-8 lg:p-12 xl:p-16 bg-surface-900/30 border border-surface-800/30 lg:border-l-0 lg:rounded-r-3xl backdrop-blur-sm">
          <div
            className={`w-full max-w-[340px] transition-all duration-1000 delay-500 ${mounted ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-4'}`}
          >
            {/* Mobile logo */}
            <div className="lg:hidden text-center mb-10">
              <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-brand-500 to-brand-700 flex items-center justify-center mx-auto mb-4 shadow-lg shadow-brand-600/20">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2">
                  <path d="M12 2L2 7l10 5 10-5-10-5z" />
                  <path d="M2 17l10 5 10-5" />
                  <path d="M2 12l10 5 10-5" />
                </svg>
              </div>
              <h1 className="text-xl font-bold text-white tracking-tight">AI Gateway</h1>
            </div>

            <div className="mb-8">
              <h2 className="text-xl font-bold text-white tracking-tight">欢迎回来</h2>
              <p className="text-sm text-surface-400 mt-1.5">登录以管理你的 AI 网关</p>
            </div>

            <form ref={formRef} onSubmit={handleSubmit} className="space-y-5">
              {/* Error */}
              <div
                className={`overflow-hidden transition-all duration-300 ${error ? 'max-h-40 opacity-100' : 'max-h-0 opacity-0'}`}
              >
                <div className="bg-red-950/40 border border-red-800/30 rounded-xl px-4 py-3 flex items-start gap-3">
                  <div className="w-5 h-5 rounded-full bg-red-600/20 flex items-center justify-center shrink-0 mt-px">
                    <span className="text-red-400 text-[10px] font-bold">!</span>
                  </div>
                  <div>
                    <p className="text-xs font-medium text-red-300">认证失败</p>
                    <p className="text-[11px] text-red-400/60 mt-0.5">{error}</p>
                  </div>
                </div>
              </div>

              {/* Username */}
              <div className="space-y-2">
                <label className="text-[11px] font-semibold text-surface-500 uppercase tracking-[0.15em]">用户名</label>
                <div
                  className={`relative rounded-xl transition-all duration-300`}
                >
                  <User
                    size={15}
                    className={`absolute left-4 top-1/2 -translate-y-1/2 transition-colors duration-300 z-10 ${focus === 'username' ? 'text-brand-400' : 'text-surface-500'}`}
                    strokeWidth={1.5}
                  />
                  <input
                    type="text"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    onFocus={() => setFocus('username')}
                    onBlur={() => setFocus(null)}
                    className="w-full bg-surface-900/80 border border-surface-700/60 rounded-xl px-4 py-3 pl-11
                             text-sm text-white placeholder:text-surface-600
                             focus:outline-none focus:border-brand-500/40 focus:bg-surface-900
                             transition-all duration-200"
                    placeholder="输入用户名"
                    autoFocus
                  />
                </div>
              </div>

              {/* Password */}
              <div className="space-y-2">
                <label className="text-[11px] font-semibold text-surface-500 uppercase tracking-[0.15em]">密码</label>
                <div
                  className={`relative rounded-xl transition-all duration-300`}
                >
                  <Lock
                    size={15}
                    className={`absolute left-4 top-1/2 -translate-y-1/2 transition-colors duration-300 z-10 ${focus === 'password' ? 'text-brand-400' : 'text-surface-500'}`}
                    strokeWidth={1.5}
                  />
                  <input
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    onFocus={() => setFocus('password')}
                    onBlur={() => setFocus(null)}
                    onKeyDown={(e: React.KeyboardEvent) => { if (e.key === 'Enter') formRef.current?.requestSubmit() }}
                    className="w-full bg-surface-900/80 border border-surface-700/60 rounded-xl px-4 py-3 pl-11
                             text-sm text-white placeholder:text-surface-600
                             focus:outline-none focus:border-brand-500/40 focus:bg-surface-900
                             transition-all duration-200"
                    placeholder="••••••••"
                  />
                </div>
              </div>

              {/* Submit */}
              <button
                type="submit"
                disabled={loading || !username || !password}
                className="relative w-full h-12 rounded-xl bg-white text-surface-950 font-semibold text-sm
                         shadow-lg shadow-white/5 hover:shadow-white/10 hover:-translate-y-0.5
                         active:translate-y-0 transition-all duration-200
                         disabled:opacity-20 disabled:cursor-not-allowed disabled:hover:translate-y-0
                         flex items-center justify-center gap-2 group overflow-hidden"
              >
                <div className="absolute inset-0 bg-gradient-to-r from-brand-500 via-brand-400 to-accent-400 opacity-0 group-hover:opacity-100 transition-opacity duration-300 rounded-xl" />
                {loading ? (
                  <span className="relative flex items-center gap-2">
                    <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                    </svg>
                    <span className="relative z-10 text-surface-950">登录中...</span>
                  </span>
                ) : (
                  <span className="relative z-10 flex items-center gap-2">
                    <span>登 录</span>
                    <ArrowRight size={16} className="transition-transform group-hover:translate-x-1" />
                  </span>
                )}
              </button>

              {/* Hint */}
              <div className="flex items-center gap-3 pt-2">
                <span className="h-px flex-1 bg-surface-800/50" />
                <span className="text-[11px] text-surface-600">admin / admin123</span>
                <span className="h-px flex-1 bg-surface-800/50" />
              </div>
            </form>
          </div>
        </div>
      </div>
    </div>
  )
}