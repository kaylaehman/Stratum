import { useState, useEffect } from 'react'
import { Bot, Loader } from 'lucide-react'
import { useAIConfig, useSetAIConfig } from '../../lib/api/ai'
import { ApiError } from '../../lib/api'
import type { AIProvider } from '../../types/api'

// ── Helpers ───────────────────────────────────────────────────────────────────

function inputStyle(focused: boolean): React.CSSProperties {
  return {
    backgroundColor: 'var(--bg-elevated)',
    border: `1px solid ${focused ? 'var(--accent)' : 'var(--border-default)'}`,
    color: 'var(--text-primary)',
    borderRadius: '3px',
    outline: 'none',
    width: '100%',
    padding: '6px 10px',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
    boxSizing: 'border-box',
  }
}

// ── Section ───────────────────────────────────────────────────────────────────

export function AISettingsSection() {
  const { data: config, isLoading } = useAIConfig()
  const save = useSetAIConfig()

  const [provider, setProvider] = useState<AIProvider>('')
  const [ollamaUrl, setOllamaUrl] = useState('')
  const [ollamaModel, setOllamaModel] = useState('')
  const [claudeModel, setClaudeModel] = useState('')
  // newApiKey: undefined = not touched (don't send), '' = clear, string = set
  const [newApiKey, setNewApiKey] = useState<string | undefined>(undefined)
  const [showKeyInput, setShowKeyInput] = useState(false)

  const [focusField, setFocusField] = useState<string | null>(null)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  // Sync form from fetched config
  useEffect(() => {
    if (!config) return
    setProvider(config.provider)
    setOllamaUrl(config.ollama_base_url ?? '')
    setOllamaModel(config.ollama_model ?? '')
    setClaudeModel(config.claude_model ?? '')
    setNewApiKey(undefined)
    setShowKeyInput(false)
  }, [config])

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSuccessMsg(null)
    setErrorMsg(null)

    try {
      await save.mutateAsync({
        provider,
        ollama_base_url: provider === 'ollama' ? ollamaUrl : undefined,
        ollama_model: provider === 'ollama' ? ollamaModel : undefined,
        claude_model: provider === 'claude' ? claudeModel : undefined,
        // Only include api_key in payload if user actually typed something or explicitly cleared
        ...(newApiKey !== undefined ? { api_key: newApiKey } : {}),
      })
      setSuccessMsg('Configuration saved.')
      setNewApiKey(undefined)
      setShowKeyInput(false)
    } catch (err) {
      if (err instanceof ApiError && (err.body as { error?: string })?.error === 'invalid_config') {
        setErrorMsg('Invalid configuration. Check the provider settings and try again.')
      } else {
        setErrorMsg('Failed to save configuration.')
      }
    }
  }

  const configured = config?.configured ?? false

  return (
    <section
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        padding: '20px',
      }}
    >
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
        <Bot size={16} style={{ color: configured ? 'var(--accent)' : 'var(--text-muted)' }} />
        <div style={{ flex: 1 }}>
          <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)', margin: 0 }}>
            AI Assistant
          </h2>
          <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
            Provider configuration for AI-powered log analysis and diagnostics (admin only)
          </p>
        </div>
        {!isLoading && (
          <span
            style={{
              fontSize: '0.65rem',
              fontWeight: 600,
              padding: '2px 8px',
              borderRadius: '3px',
              backgroundColor: configured
                ? 'color-mix(in srgb, var(--status-ok) 15%, transparent)'
                : 'var(--bg-elevated)',
              color: configured ? 'var(--status-ok)' : 'var(--text-muted)',
              border: `1px solid ${configured ? 'var(--status-ok)' : 'var(--border-subtle)'}`,
              letterSpacing: '0.05em',
              textTransform: 'uppercase',
            }}
          >
            {configured ? 'Configured' : 'Not configured'}
          </span>
        )}
      </div>

      {isLoading ? (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-muted)' }}>
          <Loader size={14} className="animate-spin" />
          <span className="text-xs">Loading…</span>
        </div>
      ) : (
        <form onSubmit={handleSave} style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          {/* Provider select */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              Provider
            </label>
            <select
              value={provider}
              onChange={e => {
                setProvider(e.target.value as AIProvider)
                setSuccessMsg(null)
                setErrorMsg(null)
                setNewApiKey(undefined)
                setShowKeyInput(false)
              }}
              style={{
                ...inputStyle(false),
                cursor: 'pointer',
              }}
            >
              <option value="">None</option>
              <option value="ollama">Local (Ollama)</option>
              <option value="claude">Claude API</option>
            </select>
          </div>

          {/* Ollama fields */}
          {provider === 'ollama' && (
            <>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                  Base URL
                </label>
                <input
                  type="url"
                  value={ollamaUrl}
                  onChange={e => setOllamaUrl(e.target.value)}
                  onFocus={() => setFocusField('ollamaUrl')}
                  onBlur={() => setFocusField(null)}
                  placeholder="http://localhost:11434"
                  style={inputStyle(focusField === 'ollamaUrl')}
                />
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                  Model
                </label>
                <input
                  type="text"
                  value={ollamaModel}
                  onChange={e => setOllamaModel(e.target.value)}
                  onFocus={() => setFocusField('ollamaModel')}
                  onBlur={() => setFocusField(null)}
                  placeholder="llama3"
                  style={inputStyle(focusField === 'ollamaModel')}
                />
              </div>
            </>
          )}

          {/* Claude fields */}
          {provider === 'claude' && (
            <>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                  Model
                </label>
                <input
                  type="text"
                  value={claudeModel}
                  onChange={e => setClaudeModel(e.target.value)}
                  onFocus={() => setFocusField('claudeModel')}
                  onBlur={() => setFocusField(null)}
                  placeholder="claude-sonnet-4-6"
                  style={inputStyle(focusField === 'claudeModel')}
                />
              </div>

              {/* API Key management */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                  API Key
                </label>

                {!showKeyInput ? (
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <span
                      className="font-mono text-xs"
                      style={{
                        color: config?.has_api_key ? 'var(--status-ok)' : 'var(--text-muted)',
                      }}
                    >
                      {config?.has_api_key ? 'Key is set' : 'No key stored'}
                    </span>
                    <button
                      type="button"
                      onClick={() => { setShowKeyInput(true); setNewApiKey('') }}
                      style={{
                        background: 'transparent',
                        border: '1px solid var(--border-default)',
                        borderRadius: '3px',
                        color: 'var(--text-secondary)',
                        fontSize: '0.7rem',
                        padding: '2px 10px',
                        cursor: 'pointer',
                        fontFamily: 'monospace',
                      }}
                    >
                      {config?.has_api_key ? 'Replace' : 'Set key'}
                    </button>
                    {config?.has_api_key && (
                      <button
                        type="button"
                        onClick={() => { setNewApiKey(''); setShowKeyInput(false) }}
                        style={{
                          background: 'transparent',
                          border: '1px solid var(--status-error)',
                          borderRadius: '3px',
                          color: 'var(--status-error)',
                          fontSize: '0.7rem',
                          padding: '2px 10px',
                          cursor: 'pointer',
                          fontFamily: 'monospace',
                        }}
                      >
                        Clear
                      </button>
                    )}
                  </div>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                    <input
                      type="password"
                      value={newApiKey ?? ''}
                      onChange={e => setNewApiKey(e.target.value)}
                      onFocus={() => setFocusField('apiKey')}
                      onBlur={() => setFocusField(null)}
                      placeholder="sk-ant-…"
                      autoComplete="off"
                      style={inputStyle(focusField === 'apiKey')}
                    />
                    <button
                      type="button"
                      onClick={() => { setShowKeyInput(false); setNewApiKey(undefined) }}
                      style={{
                        background: 'transparent',
                        border: 'none',
                        color: 'var(--text-muted)',
                        fontSize: '0.7rem',
                        cursor: 'pointer',
                        fontFamily: 'monospace',
                        alignSelf: 'flex-start',
                        padding: '0',
                      }}
                    >
                      Cancel key change
                    </button>
                  </div>
                )}

                {/* Hint: clear action is pending on save */}
                {!showKeyInput && newApiKey === '' && config?.has_api_key && (
                  <p className="text-xs" style={{ color: 'var(--status-warn)', margin: 0 }}>
                    Key will be cleared on save.
                  </p>
                )}
              </div>
            </>
          )}

          {/* Feedback */}
          {successMsg && (
            <p className="text-xs" style={{ color: 'var(--status-ok)', margin: 0 }}>
              {successMsg}
            </p>
          )}
          {errorMsg && (
            <p className="text-xs" style={{ color: 'var(--status-error)', margin: 0 }}>
              {errorMsg}
            </p>
          )}

          {/* Save */}
          <div>
            <button
              type="submit"
              disabled={save.isPending}
              style={{
                backgroundColor: 'var(--accent)',
                color: '#fff',
                border: 'none',
                borderRadius: '3px',
                padding: '6px 18px',
                fontSize: '0.75rem',
                fontWeight: 500,
                cursor: save.isPending ? 'not-allowed' : 'pointer',
                opacity: save.isPending ? 0.6 : 1,
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
              }}
            >
              {save.isPending && <Loader size={12} className="animate-spin" />}
              Save
            </button>
          </div>
        </form>
      )}
    </section>
  )
}
