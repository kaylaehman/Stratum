import { useState, useEffect, useCallback } from 'react'
import { Bot, X, Settings, Maximize2 } from 'lucide-react'
import { Link } from 'react-router-dom'
import { AIConversation } from './AIConversation'

// ── Panel ─────────────────────────────────────────────────────────────────────

export function AIAssistantPanel() {
  const [open, setOpen] = useState(false)

  // Keyboard shortcut: Ctrl+Shift+A / Cmd+Shift+A
  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key.toLowerCase() === 'a') {
      e.preventDefault()
      setOpen((prev) => !prev)
    }
  }, [])

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [handleKeyDown])

  return (
    <>
      {/* Floating toggle button */}
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
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
            <span className="text-xs font-semibold" style={{ color: 'var(--text-primary)', flex: 1 }}>
              AI Assistant
            </span>
            <Link
              to="/chat"
              title="Open full-screen"
              onClick={() => setOpen(false)}
              style={{ color: 'var(--text-muted)', display: 'flex', alignItems: 'center' }}
            >
              <Maximize2 size={13} />
            </Link>
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

          <AIConversation variant="panel" onLinkNavigate={() => setOpen(false)} />
        </div>
      )}
    </>
  )
}
