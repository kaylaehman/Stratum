import { useState, useEffect } from 'react'
import { Bot, Loader, ExternalLink } from 'lucide-react'
import {
  useAIConfig,
  useSetAIConfig,
  useAIOAuthStart,
  useAIOAuthExchange,
  useAIOAuthDisconnect,
  useAIOAuthSetToken,
} from '../../lib/api/ai'
import { ApiError } from '../../lib/api'
import type { AIProvider } from '../../types/api'

// ClaudeOAuthPanel drives the "sign in with Claude" flow: start (PKCE) → open
// the authorize URL → paste the returned code → exchange for tokens. The
// verifier/state live only in this component's state between start and exchange.
function ClaudeOAuthPanel({ connected }: { connected: boolean }) {
  const start = useAIOAuthStart()
  const exchange = useAIOAuthExchange()
  const disconnect = useAIOAuthDisconnect()
  const setToken = useAIOAuthSetToken()
  const [pkce, setPkce] = useState<{ verifier: string; state: string } | null>(null)
  const [code, setCode] = useState('')
  const [err, setErr] = useState('')
  const [showPaste, setShowPaste] = useState(false)
  const [pasteToken, setPasteToken] = useState('')

  const btn: React.CSSProperties = {
    display: 'flex', alignItems: 'center', gap: 6, alignSelf: 'flex-start',
    background: 'var(--accent)', color: '#fff', border: 'none',
    borderRadius: '3px', fontSize: 12, padding: '6px 12px', cursor: 'pointer',
  }

  async function beginSignIn() {
    setErr('')
    try {
      const res = await start.mutateAsync()
      setPkce({ verifier: res.verifier, state: res.state })
      window.open(res.authorize_url, '_blank', 'noopener,noreferrer')
    } catch {
      setErr('Could not start sign-in.')
    }
  }

  async function finishSignIn() {
    if (!pkce || !code.trim()) return
    setErr('')
    try {
      await exchange.mutateAsync({ code: code.trim(), verifier: pkce.verifier, state: pkce.state })
      setPkce(null)
      setCode('')
    } catch (e) {
      const rejected = e instanceof ApiError && (e.body as { error?: string })?.error === 'invalid_code'
      setErr(rejected ? 'That code was rejected — start again and re-copy it.' : 'Sign-in failed. Try again.')
    }
  }

  async function savePastedToken() {
    if (!pasteToken.trim()) return
    setErr('')
    try {
      await setToken.mutateAsync({ access_token: pasteToken.trim() })
      setPasteToken('')
      setShowPaste(false)
    } catch (e) {
      const bad = e instanceof ApiError && (e.body as { error?: string })?.error === 'invalid_token'
      setErr(bad ? 'That token was rejected.' : 'Could not save token.')
    }
  }

  if (connected) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <span className="font-mono text-xs" style={{ color: 'var(--status-ok)' }}>
          Connected to Claude
        </span>
        <button
          type="button"
          disabled={disconnect.isPending}
          onClick={() => void disconnect.mutateAsync()}
          style={{
            background: 'transparent', border: '1px solid var(--status-error)', borderRadius: '3px',
            color: 'var(--status-error)', fontSize: '0.7rem', padding: '2px 10px', cursor: 'pointer',
            fontFamily: 'monospace',
          }}
        >
          Disconnect
        </button>
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      {!pkce ? (
        <button type="button" onClick={() => void beginSignIn()} disabled={start.isPending} style={btn}>
          <ExternalLink size={12} />
          {start.isPending ? 'Starting…' : 'Sign in with Claude'}
        </button>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <p className="text-xs" style={{ color: 'var(--text-secondary)', margin: 0 }}>
            A Claude sign-in tab opened. Approve access, then paste the code it shows here.
          </p>
          <textarea
            rows={2}
            value={code}
            onChange={(e) => setCode(e.target.value)}
            placeholder="paste the authorization code"
            style={{ ...inputStyle(false), resize: 'vertical' }}
          />
          <div style={{ display: 'flex', gap: 8 }}>
            <button type="button" onClick={() => void finishSignIn()} disabled={exchange.isPending || !code.trim()} style={btn}>
              {exchange.isPending ? 'Connecting…' : 'Connect'}
            </button>
            <button
              type="button"
              onClick={() => { setPkce(null); setCode(''); setErr('') }}
              style={{
                background: 'transparent', border: '1px solid var(--border-default)', borderRadius: '3px',
                color: 'var(--text-secondary)', fontSize: 12, padding: '6px 12px', cursor: 'pointer',
              }}
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Fallback: paste a token (e.g. from `claude setup-token`) — skips the
          browser handshake entirely. */}
      {!pkce && (
        showPaste ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <textarea
              rows={2}
              value={pasteToken}
              onChange={(e) => setPasteToken(e.target.value)}
              placeholder="paste a Claude OAuth token (e.g. from `claude setup-token`)"
              style={{ ...inputStyle(false), resize: 'vertical' }}
            />
            <div style={{ display: 'flex', gap: 8 }}>
              <button type="button" onClick={() => void savePastedToken()} disabled={setToken.isPending || !pasteToken.trim()} style={btn}>
                {setToken.isPending ? 'Saving…' : 'Save token'}
              </button>
              <button
                type="button"
                onClick={() => { setShowPaste(false); setPasteToken(''); setErr('') }}
                style={{
                  background: 'transparent', border: '1px solid var(--border-default)', borderRadius: '3px',
                  color: 'var(--text-secondary)', fontSize: 12, padding: '6px 12px', cursor: 'pointer',
                }}
              >
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <button
            type="button"
            onClick={() => setShowPaste(true)}
            style={{
              background: 'none', border: 'none', color: 'var(--text-muted)', fontSize: 11,
              textDecoration: 'underline', cursor: 'pointer', alignSelf: 'flex-start', padding: 0,
            }}
          >
            or paste a token instead
          </button>
        )
      )}

      {err && <p className="text-xs" style={{ color: 'var(--status-error)', margin: 0 }}>{err}</p>}
    </div>
  )
}

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
  const [openaiModel, setOpenaiModel] = useState('')
  const [geminiModel, setGeminiModel] = useState('')
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
    setOpenaiModel(config.openai_model ?? '')
    setGeminiModel(config.gemini_model ?? '')
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
        claude_model: provider === 'claude' || provider === 'claude-oauth' ? claudeModel : undefined,
        openai_model: provider === 'openai' ? openaiModel : undefined,
        gemini_model: provider === 'gemini' ? geminiModel : undefined,
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
              <option value="claude-oauth">Claude (sign in — no API key)</option>
              <option value="openai">OpenAI API</option>
              <option value="gemini">Gemini API</option>
            </select>
            <p className="text-xs" style={{ color: 'var(--text-muted)', margin: '2px 0 0' }}>
              Choosing a provider <strong style={{ color: 'var(--text-secondary)' }}>enables</strong> the
              assistant; <code>None</code> turns it off. Once enabled, open it with the robot button at the
              bottom-right of any page or <code>Ctrl/Cmd+Shift+A</code>.
            </p>
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

          {/* Model field — per API-key provider */}
          {provider === 'claude' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>Model</label>
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
          )}
          {provider === 'openai' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>Model</label>
              <input
                type="text"
                value={openaiModel}
                onChange={e => setOpenaiModel(e.target.value)}
                onFocus={() => setFocusField('openaiModel')}
                onBlur={() => setFocusField(null)}
                placeholder="gpt-4o-mini"
                style={inputStyle(focusField === 'openaiModel')}
              />
            </div>
          )}
          {provider === 'gemini' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>Model</label>
              <input
                type="text"
                value={geminiModel}
                onChange={e => setGeminiModel(e.target.value)}
                onFocus={() => setFocusField('geminiModel')}
                onBlur={() => setFocusField(null)}
                placeholder="gemini-2.0-flash"
                style={inputStyle(focusField === 'geminiModel')}
              />
            </div>
          )}

          {/* Shared API key — Claude / OpenAI / Gemini (one active provider at a time) */}
          {(provider === 'claude' || provider === 'openai' || provider === 'gemini') && (
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
                      placeholder={provider === 'openai' ? 'sk-…' : provider === 'gemini' ? 'AIza…' : 'sk-ant-…'}
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
          )}

          {/* Claude OAuth (sign in) */}
          {provider === 'claude-oauth' && (
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
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                  Claude account
                </label>
                <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
                  Sign in with your Claude subscription instead of an API key. Save the provider first, then connect.
                </p>
                <ClaudeOAuthPanel connected={config?.oauth_connected ?? false} />
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
