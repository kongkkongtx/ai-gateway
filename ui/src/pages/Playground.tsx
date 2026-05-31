import { useState, useRef, useEffect, useCallback } from 'react'
import {
  Terminal,
  Send,
  Trash2,
  Settings2,
  Copy,
  Check,
  ChevronDown,
  Square,
  Sliders,
} from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { streamChatCompletion } from '../api/gateway'
import { useToast } from '../components/Toast'

interface Message {
  role: 'system' | 'user' | 'assistant'
  content: string
}

interface PlaygroundSettings {
  apiKey: string
  model: string
  customModel: string
  temperature: number
  maxTokens: number
  topP: number
}

const MODELS = ['deepseek-chat', 'deepseek-reasoner', 'gpt-4o', 'gpt-4o-mini', 'claude-3-opus', 'custom']

const DEFAULT_SETTINGS: PlaygroundSettings = {
  apiKey: '',
  model: 'deepseek-chat',
  customModel: '',
  temperature: 0.7,
  maxTokens: 4096,
  topP: 1,
}

function loadSettings(): PlaygroundSettings {
  try {
    const raw = localStorage.getItem('gw_playground_settings')
    if (raw) return { ...DEFAULT_SETTINGS, ...JSON.parse(raw) }
  } catch { /* ignore */ }
  return { ...DEFAULT_SETTINGS }
}

function saveSettings(s: PlaygroundSettings) {
  localStorage.setItem('gw_playground_settings', JSON.stringify(s))
}

export default function Playground() {
  const [settings, setSettings] = useState<PlaygroundSettings>(loadSettings)
  const [messages, setMessages] = useState<Message[]>([
    { role: 'system', content: 'You are a helpful AI assistant.' },
  ])
  const [input, setInput] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [showApiKey, setShowApiKey] = useState(false)
  const [showAuthHint, setShowAuthHint] = useState(true)
  const [copied, setCopied] = useState(false)
  const [showSettings, setShowSettings] = useState(false)
  const [showModelDropdown, setShowModelDropdown] = useState(false)
  const abortRef = useRef<AbortController | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const { toast } = useToast()

  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [messages])

  // Persist settings on change
  useEffect(() => { saveSettings(settings) }, [settings])

  const updateSetting = useCallback(<K extends keyof PlaygroundSettings>(key: K, value: PlaygroundSettings[K]) => {
    setSettings((prev) => ({ ...prev, [key]: value }))
  }, [])

  const handleSend = async () => {
    if (!input.trim() || streaming) return
    const userMsg: Message = { role: 'user', content: input.trim() }
    const newMessages = [...messages, userMsg]
    setMessages(newMessages)
    setInput('')
    setStreaming(true)

    const actualModel = settings.model === 'custom' ? settings.customModel : settings.model
    const controller = new AbortController()
    abortRef.current = controller

    try {
      const assistantMsg: Message = { role: 'assistant', content: '' }
      setMessages((prev) => [...prev, assistantMsg])

      let fullContent = ''
      await streamChatCompletion(
        {
          model: actualModel,
          messages: newMessages.map((m) => ({ role: m.role, content: m.content })),
          temperature: settings.temperature,
          max_tokens: settings.maxTokens,
          top_p: settings.topP,
        },
        settings.apiKey || undefined,
        (chunk) => {
          fullContent += chunk
          setMessages((prev) => {
            const copy = [...prev]
            if (copy.length > 0) {
              copy[copy.length - 1] = { role: 'assistant', content: fullContent }
            }
            return copy
          })
        },
        () => {
          setStreaming(false)
          abortRef.current = null
        },
        controller.signal,
      )
    } catch (err) {
      if ((err as Error)?.name === 'AbortError') {
        toast('info', 'Stream cancelled')
      } else {
        setMessages((prev) => [
          ...prev,
          { role: 'assistant', content: `**Error**: ${err instanceof Error ? err.message : 'Request failed'}` },
        ])
        toast('error', 'Request failed', err instanceof Error ? err.message : undefined)
      }
      setStreaming(false)
      abortRef.current = null
    }
  }

  const handleStop = () => {
    abortRef.current?.abort()
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const clearChat = () => {
    if (streaming) handleStop()
    setMessages([{ role: 'system', content: 'You are a helpful AI assistant.' }])
    setInput('')
    toast('info', 'Chat cleared')
  }

  const copyMessages = async () => {
    const text = messages.map((m) => `### ${m.role}\n${m.content}`).join('\n\n')
    await navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
    toast('success', 'Copied to clipboard')
  }

  const fillDemoKey = () => {
    updateSetting('apiKey', 'sk-gateway-demo-key')
    setShowAuthHint(false)
    toast('success', 'Demo API key filled')
  }

  return (
    <div className="space-y-6 h-[calc(100vh-7rem)] flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between shrink-0">
        <div>
          <h1 className="text-2xl font-bold text-white">Playground</h1>
          <p className="text-sm text-surface-400 mt-1">Online AI Gateway API testing</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={clearChat} className="btn-ghost flex items-center gap-1.5 text-xs">
            <Trash2 size={14} /> Clear
          </button>
          <button onClick={copyMessages} className="btn-ghost flex items-center gap-1.5 text-xs">
            {copied ? <Check size={14} className="text-accent-400" /> : <Copy size={14} />}
            Copy
          </button>
          <button onClick={() => setShowSettings(!showSettings)} className={`btn-ghost flex items-center gap-1.5 text-xs ${showSettings ? 'text-brand-400 bg-surface-800' : ''}`}>
            <Settings2 size={14} /> Settings
          </button>
        </div>
      </div>

      {/* Settings Panel */}
      {showSettings && (
        <div className="card shrink-0">
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {/* Model */}
            <div className="space-y-2">
              <label className="text-xs font-medium text-surface-400">Model</label>
              <div className="relative">
                <button
                  onClick={() => setShowModelDropdown(!showModelDropdown)}
                  className="input w-full text-left flex items-center justify-between"
                >
                  <span>{settings.model === 'custom' ? settings.customModel || 'Enter custom model...' : settings.model}</span>
                  <ChevronDown size={14} className="text-surface-400" />
                </button>
                {showModelDropdown && (
                  <div className="absolute top-full left-0 right-0 mt-1 bg-surface-800 border border-surface-700 rounded-lg shadow-xl z-10 py-1 max-h-48 overflow-y-auto">
                    {MODELS.map((m) => (
                      <button
                        key={m}
                        className={`w-full text-left px-3 py-2 text-sm hover:bg-surface-700 transition-colors ${settings.model === m ? 'text-brand-400 bg-brand-600/10' : 'text-surface-300'}`}
                        onClick={() => { updateSetting('model', m); setShowModelDropdown(false) }}
                      >{m}</button>
                    ))}
                  </div>
                )}
              </div>
              {settings.model === 'custom' && (
                <input
                  className="input w-full mt-1"
                  placeholder="e.g. gpt-4-turbo"
                  value={settings.customModel}
                  onChange={(e) => updateSetting('customModel', e.target.value)}
                />
              )}
            </div>

            {/* API Key */}
            <div className="space-y-2">
              <label className="text-xs font-medium text-surface-400">API Key</label>
              <div className="relative">
                <input
                  className="input w-full pr-10"
                  type={showApiKey ? 'text' : 'password'}
                  placeholder="sk-..."
                  value={settings.apiKey}
                  onChange={(e) => updateSetting('apiKey', e.target.value)}
                />
                <button
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-surface-500 hover:text-surface-300"
                  onClick={() => setShowApiKey(!showApiKey)}
                >
                  {showApiKey ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </div>

            {/* Parameters */}
            <div className="space-y-3">
              <div className="flex items-center gap-1.5 text-xs font-medium text-surface-400">
                <Sliders size={12} /> Parameters
              </div>
              <div>
                <div className="flex justify-between text-xs">
                  <span className="text-surface-500">Temperature</span>
                  <span className="text-surface-300 font-mono">{settings.temperature.toFixed(1)}</span>
                </div>
                <input
                  type="range" min="0" max="2" step="0.1"
                  value={settings.temperature}
                  onChange={(e) => updateSetting('temperature', parseFloat(e.target.value))}
                  className="w-full accent-brand-500"
                />
              </div>
              <div>
                <div className="flex justify-between text-xs">
                  <span className="text-surface-500">Max Tokens</span>
                  <span className="text-surface-300 font-mono">{settings.maxTokens}</span>
                </div>
                <input
                  type="range" min="64" max="16384" step="64"
                  value={settings.maxTokens}
                  onChange={(e) => updateSetting('maxTokens', parseInt(e.target.value))}
                  className="w-full accent-brand-500"
                />
              </div>
              <div>
                <div className="flex justify-between text-xs">
                  <span className="text-surface-500">Top P</span>
                  <span className="text-surface-300 font-mono">{settings.topP.toFixed(1)}</span>
                </div>
                <input
                  type="range" min="0" max="1" step="0.05"
                  value={settings.topP}
                  onChange={(e) => updateSetting('topP', parseFloat(e.target.value))}
                  className="w-full accent-brand-500"
                />
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Auth Hint */}
      {showAuthHint && !settings.apiKey && (
        <div className="bg-yellow-900/20 border border-yellow-800/40 rounded-xl p-4 flex items-start gap-3 shrink-0">
          <AlertTriangle size={20} className="text-yellow-400 shrink-0 mt-0.5" />
          <div className="flex-1">
            <p className="text-sm font-medium text-yellow-300">API Key Required</p>
            <p className="text-xs text-yellow-400/70 mt-1">Gateway has authentication enabled. Enter an API key in Settings, or use the demo key:</p>
            <div className="mt-2 flex items-center gap-2">
              <code className="px-2 py-1 rounded bg-yellow-900/30 text-yellow-300 text-xs font-mono border border-yellow-700/30">sk-gateway-demo-key</code>
              <button
                onClick={fillDemoKey}
                className="px-2.5 py-1 rounded-lg bg-yellow-700/30 text-yellow-300 text-xs font-medium hover:bg-yellow-700/50 transition-colors"
              >Auto-fill</button>
              <button onClick={() => setShowAuthHint(false)} className="text-yellow-500/60 hover:text-yellow-400 text-xs ml-1">Dismiss</button>
            </div>
          </div>
        </div>
      )}

      {/* Chat Area */}
      <div className="flex-1 overflow-y-auto space-y-4 min-h-0 pr-2">
        {messages.map((msg, idx) => (
          <div key={idx} className={`flex gap-3 ${msg.role === 'user' ? 'justify-end' : ''}`}>
            {msg.role !== 'user' && (
              <div className="w-8 h-8 rounded-lg bg-brand-600/15 border border-brand-600/20 flex items-center justify-center shrink-0 mt-1">
                <Terminal size={14} className="text-brand-400" />
              </div>
            )}
            <div className={`max-w-[75%] ${msg.role === 'user' ? 'order-first' : ''}`}>
              {msg.role !== 'user' && (
                <div className="text-[10px] font-semibold text-surface-500 uppercase mb-1.5">
                  {msg.role === 'system' ? 'System' : 'Assistant'}
                </div>
              )}
              <div className={`rounded-xl px-4 py-3 text-sm leading-relaxed ${
                msg.role === 'user'
                  ? 'bg-brand-600 text-white'
                  : msg.role === 'system'
                  ? 'bg-surface-900 border border-surface-700 text-surface-400 italic'
                  : 'bg-surface-900 border border-surface-800 text-surface-200'
              }`}>
                {msg.role === 'assistant' && !msg.content.startsWith('**Error') ? (
                  <div className="prose prose-invert prose-sm max-w-none">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{msg.content}</ReactMarkdown>
                  </div>
                ) : (
                  <pre className="whitespace-pre-wrap font-sans">{msg.content || (streaming && idx === messages.length - 1 ? 'Thinking...' : '')}</pre>
                )}
              </div>
            </div>
            {msg.role === 'user' && (
              <div className="w-8 h-8 rounded-lg bg-accent-600/15 border border-accent-600/20 flex items-center justify-center shrink-0 mt-1">
                <span className="text-xs font-bold text-accent-400">U</span>
              </div>
            )}
          </div>
        ))}
        <div ref={bottomRef} />
      </div>

      {/* Input Area */}
      <div className="shrink-0 card p-3">
        <div className="flex gap-3 items-end">
          <div className="flex-1 relative">
            <textarea
              ref={inputRef}
              className="input w-full resize-none pr-12 py-3 text-sm min-h-[44px] max-h-[120px]"
              placeholder="Type a message... (Shift+Enter for new line)"
              rows={1}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              disabled={streaming}
            />
            {streaming ? (
              <button
                onClick={handleStop}
                className="absolute right-1.5 bottom-1.5 p-2 rounded-lg bg-red-600 text-white hover:bg-red-500 transition-all"
              >
                <Square size={16} />
              </button>
            ) : (
              <button
                onClick={handleSend}
                disabled={!input.trim()}
                className={`absolute right-1.5 bottom-1.5 p-2 rounded-lg transition-all ${
                  input.trim()
                    ? 'bg-brand-600 text-white hover:bg-brand-500'
                    : 'bg-surface-800 text-surface-500 cursor-not-allowed'
                }`}
              >
                <Send size={16} />
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
