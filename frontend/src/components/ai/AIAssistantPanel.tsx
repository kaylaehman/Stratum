import { useState, useEffect, useRef, useCallback } from 'react'
import { Bot, X, Send, Loader, Settings } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useAsk } from '../../lib/api/ai'
import { ApiError } from '../../lib/api'

// ── Types ─────────────────────────────────────────────────────────────────────

interface Turn {
  id: number
  role: 'user' | 'assistant'
  text: string
  inputTokens?: number
  outputTokens?: number
  error?: boolean
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function isNotConfiguredError(err: unknown): boolean {
  return err instanceof ApiError && (err.body as { error?: string })?.error === 'ai_not_configured'
}

/** Render assistant text: preserve whitespace, style code blocks. */
function AnswerText({ text }: { text: string }) {
  // Split on triple-backtick fences for minimal code block support
  const parts = text.split(/(```[\s\S]*?```)/g)
  return (
    <span style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
      {parts.map((part, i) => {
        if (part.startsWith('```') && part.endsWith('```')) {
          const inner = part.slice(3, -3).replace(/^[^\n]*\n/, '') // strip language tag
          return (
            <code
              key={i}
              style={{
                display: 'block',
                backgroundColor: 'var(--bg-base)',
                border: '1px solid var(--border-subtle)',
                borderRadius: '3px',
                padding: '8px',
                marginTop: '4px',
                marginBottom: '4px',
                fontFamily: 'monospace',
                fontSize: '0.7rem',
                color: 'var(--text-secondary)',
                whiteSpace: 'pre',
                overflowX: 'auto',
              }}
            >
              {inner}
            </code>
          )
        }
        return <span key={i}>{part}</span>
      })}
    </span>
  )
}

// ── Panel ─────────────────────────────────────────────────────────────────────

let turnCounter = 0

export function AIAssistantPanel() {
  const [open, setOpen] = useState(false)
  const [turns, setTurns] = useState<Turn[]>([])
  const [prompt, setPrompt] = useState('')
  const [focusedInput, setFocusedInput] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const ask = useAsk()

  // Keyboard shortcut: Ctrl+Shift+A / Cmd+Shift+A
  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key.toLowerCase() === 'a') {
      e.preventDefault()
      setOpen(prev => !prev)
    }
  }, [])

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [handleKeyDown])

  // Scroll to bottom on new turns
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [turns])

  async function handleSend() {
    const text = prompt.trim()
    if (!text || ask.isPending) return

    const userTurn: Turn = { id: ++turnCounter, role: 'user', text }
    setTurns(prev => [...prev, userTurn])
    setPrompt('')

    try {
      const res = await ask.mutateAsync({ prompt: text, task: '' })
      setTurns(prev => [
        ...prev,
        {
          id: ++turnCounter,
          role: 'assistant',
          text: res.answer,
          inputTokens: res.input_tokens,
          outputTokens: res.output_tokens,
        },
      ])
    } catch (err) {
      const notConfigured = isNotConfiguredError(err)
      setTurns(prev => [
        ...prev,
        {
          id: ++turnCounter,
          role: 'assistant',
          text: notConfigured
            ? '__NOT_CONFIGURED__'
            : 'An error occurred. Please try again.',
          error: true,
        },
      ])
    }
  }

  function handleInputKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      void handleSend()
    }
  }

  return (
    <>
      {/* Floating toggle button */}
      <button
        type="button"
        onClick={() => setOpen(prev => !prev)}
        title="AI Assistant (Ctrl+Shift+A)"
        style={{
          position: 'fixed',
          bottom: '24px',
          right: '24px',
          zIndex: 9000,
          width: '44px',
          height: '44px',
          borderRadius: '50%',
          border: '1px solid var(--border-default)',
          backgroundColor: open ? 'var(--accent)' : 'var(--bg-elevated)',
          color: open ? '#fff' : 'var(--text-muted)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          cursor: 'pointer',
          boxShadow: open ? '0 0 0 2px var(--accent-glow)' : '0 2px 8px rgba(0,0,0,0.4)',
          transition: 'background-color 0.15s, color 0.15s',
        }}
      >
        <Bot size={20} />
      </button>

      {/* Panel */}
      {open && (
        <div
          style={{
            position: 'fixed',
            bottom: '80px',
            right: '24px',
            zIndex: 8999,
            width: '380px',
            maxHeight: '560px',
            display: 'flex',
            flexDirection: 'column',
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--border-default)',
            borderRadius: '3px',
            boxShadow: '0 8px 24px rgba(0,0,0,0.5)',
          }}
        >
          {/* Header */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              padding: '10px 14px',
              borderBottom: '1px solid var(--border-subtle)',
              flexShrink: 0,
            }}
          >
            <Bot size={14} style={{ color: 'var(--accent)' }} />
            <span
              className="text-xs font-semibold"
              style={{ color: 'var(--text-primary)', flex: 1 }}
            >
              AI Assistant
            </span>
            <Link
              to="/settings"
              title="AI settings"
              onClick={() => setOpen(false)}
              style={{ color: 'var(--text-muted)', display: 'flex', alignItems: 'center' }}
            >
              <Settings size={13} />
            </Link>
            <button
              type="button"
              onClick={() => setOpen(false)}
              style={{
                background: 'transparent',
                border: 'none',
                cursor: 'pointer',
                color: 'var(--text-muted)',
                display: 'flex',
                alignItems: 'center',
                padding: '0',
              }}
            >
              <X size={14} />
            </button>
          </div>

          {/* Conversation */}
          <div
            style={{
              flex: 1,
              overflowY: 'auto',
              padding: '12px',
              display: 'flex',
              flexDirection: 'column',
              gap: '10px',
            }}
          >
            {turns.length === 0 && (
              <p
                className="text-xs"
                style={{ color: 'var(--text-muted)', textAlign: 'center', marginTop: '24px' }}
              >
                Ask anything about your infrastructure.
              </p>
            )}

            {turns.map(turn => (
              <div
                key={turn.id}
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: turn.role === 'user' ? 'flex-end' : 'flex-start',
                  gap: '3px',
                }}
              >
                <div
                  style={{
                    maxWidth: '90%',
                    padding: '8px 10px',
                    borderRadius: '3px',
                    fontSize: '0.75rem',
                    lineHeight: '1.5',
                    backgroundColor:
                      turn.role === 'user'
                        ? 'color-mix(in srgb, var(--accent) 20%, transparent)'
                        : 'var(--bg-elevated)',
                    border: `1px solid ${
                      turn.error
                        ? 'var(--status-error)'
                        : turn.role === 'user'
                          ? 'color-mix(in srgb, var(--accent) 40%, transparent)'
                          : 'var(--border-subtle)'
                    }`,
                    color: turn.error ? 'var(--status-error)' : 'var(--text-primary)',
                  }}
                >
                  {turn.text === '__NOT_CONFIGURED__' ? (
                    <span>
                      No AI provider configured.{' '}
                      <Link
                        to="/settings"
                        onClick={() => setOpen(false)}
                        style={{ color: 'var(--accent)', textDecoration: 'underline' }}
                      >
                        Set one up in Settings.
                      </Link>
                    </span>
                  ) : (
                    <AnswerText text={turn.text} />
                  )}
                </div>
                {turn.role === 'assistant' && !turn.error && turn.inputTokens !== undefined && (
                  <span
                    className="font-mono text-xs"
                    style={{ color: 'var(--text-muted)', fontSize: '0.65rem' }}
                  >
                    {turn.inputTokens} in / {turn.outputTokens} out tokens
                  </span>
                )}
              </div>
            ))}

            {ask.isPending && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                <Loader size={12} className="animate-spin" style={{ color: 'var(--text-muted)' }} />
                <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                  Thinking…
                </span>
              </div>
            )}

            <div ref={bottomRef} />
          </div>

          {/* Input */}
          <div
            style={{
              padding: '10px 12px',
              borderTop: '1px solid var(--border-subtle)',
              flexShrink: 0,
              display: 'flex',
              gap: '8px',
              alignItems: 'flex-end',
            }}
          >
            <textarea
              rows={2}
              value={prompt}
              onChange={e => setPrompt(e.target.value)}
              onKeyDown={handleInputKeyDown}
              onFocus={() => setFocusedInput(true)}
              onBlur={() => setFocusedInput(false)}
              placeholder="Ask something… (Enter to send)"
              disabled={ask.isPending}
              style={{
                flex: 1,
                resize: 'none',
                backgroundColor: 'var(--bg-elevated)',
                border: `1px solid ${focusedInput ? 'var(--accent)' : 'var(--border-default)'}`,
                borderRadius: '3px',
                color: 'var(--text-primary)',
                fontFamily: 'monospace',
                fontSize: '0.75rem',
                padding: '6px 8px',
                outline: 'none',
                lineHeight: '1.4',
              }}
            />
            <button
              type="button"
              onClick={() => void handleSend()}
              disabled={!prompt.trim() || ask.isPending}
              title="Send"
              style={{
                backgroundColor: 'var(--accent)',
                border: 'none',
                borderRadius: '3px',
                color: '#fff',
                padding: '8px 10px',
                cursor: !prompt.trim() || ask.isPending ? 'not-allowed' : 'pointer',
                opacity: !prompt.trim() || ask.isPending ? 0.5 : 1,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0,
              }}
            >
              {ask.isPending ? <Loader size={14} className="animate-spin" /> : <Send size={14} />}
            </button>
          </div>
        </div>
      )}
    </>
  )
}
