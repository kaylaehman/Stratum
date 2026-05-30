import { useState, useEffect, useRef } from 'react'
import { Send, Loader } from 'lucide-react'
import { Link } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
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

// providerErrorText surfaces the backend's sanitized provider message (e.g. the
// actual Claude/OpenAI/Gemini rejection reason) instead of a generic error.
function providerErrorText(err: unknown): string {
  if (err instanceof ApiError) {
    const detail = (err.body as { detail?: string })?.detail
    if (detail) return detail
  }
  return 'An error occurred. Please try again.'
}

/** Render assistant text as Markdown (GFM), styled via `.assistant-md` in index.css. */
function AnswerText({ text }: { text: string }) {
  return (
    <div className="assistant-md">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{text}</ReactMarkdown>
    </div>
  )
}

let turnCounter = 0

// ── Conversation ───────────────────────────────────────────────────────────────

export interface AIConversationProps {
  /** 'panel' is the compact floating widget; 'page' is the full-screen route. */
  variant?: 'panel' | 'page'
  /** Called when an internal <Link> (e.g. Settings) is clicked — the panel uses
   *  this to close itself; the page leaves it undefined. */
  onLinkNavigate?: () => void
  /** Autofocus the input on mount (used by the full-screen page). */
  autoFocus?: boolean
}

/**
 * The shared AI chat experience — message history + input — used by both the
 * floating AIAssistantPanel and the full-screen Assistant page. It owns the
 * conversation state and the /api/ai/ask call; callers provide only the chrome
 * around it. History is per-mount (not persisted across sessions).
 */
export function AIConversation({ variant = 'panel', onLinkNavigate, autoFocus }: AIConversationProps) {
  const isPage = variant === 'page'
  const [turns, setTurns] = useState<Turn[]>([])
  const [prompt, setPrompt] = useState('')
  const [focusedInput, setFocusedInput] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const ask = useAsk()

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [turns])

  useEffect(() => {
    if (autoFocus) inputRef.current?.focus()
  }, [autoFocus])

  async function handleSend() {
    const text = prompt.trim()
    if (!text || ask.isPending) return

    setTurns((prev) => [...prev, { id: ++turnCounter, role: 'user', text }])
    setPrompt('')

    try {
      const res = await ask.mutateAsync({ prompt: text, task: '' })
      setTurns((prev) => [
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
      setTurns((prev) => [
        ...prev,
        {
          id: ++turnCounter,
          role: 'assistant',
          text: notConfigured ? '__NOT_CONFIGURED__' : providerErrorText(err),
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

  const bubbleFontSize = isPage ? '0.85rem' : '0.75rem'

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
      {/* Conversation */}
      <div
        style={{
          flex: 1,
          overflowY: 'auto',
          padding: isPage ? '20px' : '12px',
          display: 'flex',
          flexDirection: 'column',
          gap: isPage ? '14px' : '10px',
        }}
      >
        {/* Centered reading column on the full-screen page. */}
        <div
          style={
            isPage
              ? { width: '100%', maxWidth: '820px', margin: '0 auto', display: 'flex', flexDirection: 'column', gap: '14px', flex: 1 }
              : { display: 'contents' }
          }
        >
          {turns.length === 0 && (
            <p
              className="text-xs"
              style={{ color: 'var(--text-muted)', textAlign: 'center', marginTop: isPage ? '15vh' : '24px' }}
            >
              Ask anything about your infrastructure.
            </p>
          )}

          {turns.map((turn) => (
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
                  maxWidth: isPage ? '85%' : '90%',
                  padding: isPage ? '10px 13px' : '8px 10px',
                  borderRadius: '3px',
                  fontSize: bubbleFontSize,
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
                      onClick={onLinkNavigate}
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
      </div>

      {/* Input */}
      <div
        style={{
          padding: isPage ? '14px 20px' : '10px 12px',
          borderTop: '1px solid var(--border-subtle)',
          flexShrink: 0,
        }}
      >
        <div
          style={{
            display: 'flex',
            gap: '8px',
            alignItems: 'flex-end',
            width: '100%',
            maxWidth: isPage ? '820px' : undefined,
            margin: isPage ? '0 auto' : undefined,
          }}
        >
          <textarea
            ref={inputRef}
            rows={isPage ? 3 : 2}
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            onKeyDown={handleInputKeyDown}
            onFocus={() => setFocusedInput(true)}
            onBlur={() => setFocusedInput(false)}
            placeholder="Ask something… (Enter to send, Shift+Enter for newline)"
            disabled={ask.isPending}
            style={{
              flex: 1,
              resize: 'none',
              backgroundColor: 'var(--bg-elevated)',
              border: `1px solid ${focusedInput ? 'var(--accent)' : 'var(--border-default)'}`,
              borderRadius: '3px',
              color: 'var(--text-primary)',
              fontFamily: 'monospace',
              fontSize: bubbleFontSize,
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
              padding: isPage ? '10px 12px' : '8px 10px',
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
    </div>
  )
}
