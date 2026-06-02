import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { ShieldCheck, ShieldAlert, Loader, X } from 'lucide-react'
import { useStepUpStore } from '../../store/stepup'
import { submitStepUpCode, ApiError } from '../../lib/api'

export function StepUpModal() {
  const { open, mode, resolve, reject } = useStepUpStore()
  const navigate = useNavigate()

  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  // Reset state each time modal opens and focus the input
  useEffect(() => {
    if (open) {
      setCode('')
      setError(null)
      setLoading(false)
      setTimeout(() => inputRef.current?.focus(), 0)
    }
  }, [open])

  if (!open) return null

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const trimmed = code.replace(/\s/g, '')
    if (trimmed.length !== 6) {
      setError('Enter a 6-digit code.')
      return
    }
    setLoading(true)
    setError(null)
    try {
      await submitStepUpCode(trimmed)
      resolve()
    } catch (err) {
      setLoading(false)
      if (err instanceof ApiError && err.status === 400) {
        const body = err.body as { error?: string }
        if (body?.error === 'invalid_code') {
          setError('Invalid code — try again.')
        } else if (body?.error === '2fa_not_enabled') {
          setError('2FA is not enabled on your account.')
        } else {
          setError('Verification failed.')
        }
      } else {
        setError('Unexpected error.')
      }
      setCode('')
      setTimeout(() => inputRef.current?.focus(), 0)
    }
  }

  function handleCancel() {
    reject()
  }

  function handleGoEnroll() {
    reject() // the destructive action does not proceed; user enrols first
    navigate('/settings#twofa')
  }

  return (
    <>
      {/* Backdrop */}
      <div
        onClick={handleCancel}
        style={{
          position: 'fixed',
          inset: 0,
          backgroundColor: 'rgba(0,0,0,0.55)',
          zIndex: 1100,
        }}
      />

      {/* Modal */}
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="stepup-title"
        style={{
          position: 'fixed',
          top: '50%',
          left: '50%',
          transform: 'translate(-50%,-50%)',
          width: '340px',
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          zIndex: 1101,
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '12px 14px',
            borderBottom: '1px solid var(--border-subtle)',
          }}
        >
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
            }}
          >
            {mode === 'enroll' ? (
              <ShieldAlert size={14} style={{ color: 'var(--status-warn)' }} />
            ) : (
              <ShieldCheck size={14} style={{ color: 'var(--accent)' }} />
            )}
            <span
              id="stepup-title"
              style={{
                fontSize: '12px',
                fontFamily: 'var(--font-mono, monospace)',
                letterSpacing: '0.05em',
                color: 'var(--text-primary)',
                textTransform: 'uppercase',
              }}
            >
              {mode === 'enroll' ? 'Two-factor required' : 'Confirm identity'}
            </span>
          </div>
          <button
            type="button"
            onClick={handleCancel}
            aria-label="Cancel"
            style={{
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              padding: '2px',
              display: 'flex',
              alignItems: 'center',
              color: 'var(--text-muted)',
            }}
          >
            <X size={14} />
          </button>
        </div>

        {/* Enrollment-required variant: the destructive action is blocked until
            the user sets up TOTP. No code entry here — direct them to Settings. */}
        {mode === 'enroll' ? (
          <div style={{ padding: '16px 14px' }}>
            <p
              style={{
                fontSize: '12px',
                fontFamily: 'var(--font-mono, monospace)',
                color: 'var(--text-secondary)',
                marginBottom: '14px',
                lineHeight: '1.5',
              }}
            >
              This action requires two-factor authentication. Enable 2FA on your
              account before performing destructive actions.
            </p>
            <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
              <button
                type="button"
                onClick={handleCancel}
                style={{
                  background: 'none',
                  border: '1px solid var(--border-default)',
                  borderRadius: '3px',
                  padding: '6px 14px',
                  fontSize: '12px',
                  fontFamily: 'var(--font-mono, monospace)',
                  color: 'var(--text-secondary)',
                  cursor: 'pointer',
                }}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleGoEnroll}
                style={{
                  background: 'var(--accent)',
                  border: 'none',
                  borderRadius: '3px',
                  padding: '6px 14px',
                  fontSize: '12px',
                  fontFamily: 'var(--font-mono, monospace)',
                  color: '#fff',
                  cursor: 'pointer',
                }}
              >
                Enable 2FA
              </button>
            </div>
          </div>
        ) : (
        /* Body */
        <form onSubmit={(e) => { void handleSubmit(e) }} style={{ padding: '16px 14px' }}>
          <p
            style={{
              fontSize: '12px',
              fontFamily: 'var(--font-mono, monospace)',
              color: 'var(--text-secondary)',
              marginBottom: '14px',
              lineHeight: '1.5',
            }}
          >
            This action requires 2FA confirmation. Enter your authenticator code.
          </p>

          <input
            ref={inputRef}
            type="text"
            inputMode="numeric"
            pattern="[0-9 ]*"
            maxLength={7}
            value={code}
            onChange={(e) => {
              setCode(e.target.value)
              setError(null)
            }}
            placeholder="000000"
            autoComplete="one-time-code"
            disabled={loading}
            style={{
              width: '100%',
              boxSizing: 'border-box',
              backgroundColor: 'var(--bg-surface)',
              border: `1px solid ${error ? 'var(--status-error)' : 'var(--border-default)'}`,
              borderRadius: '3px',
              padding: '8px 10px',
              fontSize: '18px',
              fontFamily: 'var(--font-mono, monospace)',
              letterSpacing: '0.3em',
              color: 'var(--text-primary)',
              outline: 'none',
              textAlign: 'center',
            }}
          />

          {error && (
            <p
              style={{
                marginTop: '8px',
                fontSize: '12px',
                fontFamily: 'var(--font-mono, monospace)',
                color: 'var(--status-error)',
              }}
            >
              {error}
            </p>
          )}

          {/* Actions */}
          <div
            style={{
              display: 'flex',
              gap: '8px',
              marginTop: '16px',
              justifyContent: 'flex-end',
            }}
          >
            <button
              type="button"
              onClick={handleCancel}
              disabled={loading}
              style={{
                background: 'none',
                border: '1px solid var(--border-default)',
                borderRadius: '3px',
                padding: '6px 14px',
                fontSize: '12px',
                fontFamily: 'var(--font-mono, monospace)',
                color: 'var(--text-secondary)',
                cursor: loading ? 'not-allowed' : 'pointer',
                opacity: loading ? 0.5 : 1,
              }}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading || code.replace(/\s/g, '').length !== 6}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                background: 'var(--accent)',
                border: 'none',
                borderRadius: '3px',
                padding: '6px 14px',
                fontSize: '12px',
                fontFamily: 'var(--font-mono, monospace)',
                color: '#fff',
                cursor:
                  loading || code.replace(/\s/g, '').length !== 6
                    ? 'not-allowed'
                    : 'pointer',
                opacity: loading || code.replace(/\s/g, '').length !== 6 ? 0.6 : 1,
              }}
            >
              {loading && <Loader size={11} style={{ animation: 'spin 1s linear infinite' }} />}
              Confirm
            </button>
          </div>
        </form>
        )}
      </div>
    </>
  )
}
