import { LogOut, Search, Menu } from 'lucide-react'
import type { User } from '../../types/api'

interface TopbarProps {
  user: User | null
  onLogout: () => void
  onSearchOpen?: () => void
  /** Called when the hamburger button is tapped (mobile only). */
  onMenuToggle?: () => void
}

export function Topbar({ user, onLogout, onSearchOpen, onMenuToggle }: TopbarProps) {
  return (
    <header
      className="h-11 flex items-center px-4 gap-4 shrink-0"
      style={{
        backgroundColor: 'var(--bg-surface)',
        borderBottom: '1px solid var(--border-subtle)',
      }}
    >
      {/* Hamburger — visible only below md */}
      <button
        type="button"
        onClick={onMenuToggle}
        className="md:hidden flex items-center justify-center shrink-0 -ml-1"
        aria-label="Open navigation"
        style={{
          background: 'transparent',
          border: 'none',
          color: 'var(--text-secondary)',
          cursor: 'pointer',
          padding: '4px',
          borderRadius: '3px',
        }}
      >
        <Menu size={18} />
      </button>

      {/* Brand — logo block is w-52 on desktop; auto-width on mobile to save space. */}
      <div className="flex items-center w-auto md:w-52 shrink-0">
        <span className="flex items-center gap-2 select-none" aria-label="Stratum">
          <svg width={28} height={28} viewBox="0 0 64 64" fill="none" aria-hidden="true">
            <defs>
              <filter id="strmHeaderGlow" x="-40%" y="-40%" width="180%" height="180%">
                <feGaussianBlur stdDeviation={1.3} result="b" />
                <feMerge>
                  <feMergeNode in="b" />
                  <feMergeNode in="SourceGraphic" />
                </feMerge>
              </filter>
            </defs>
            <g filter="url(#strmHeaderGlow)" stroke="#2E4BD8" strokeWidth={2.4} strokeLinejoin="round">
              <path d="M32 7 50 16 32 25 14 16Z" fill="#2E4BD8" fillOpacity={0.16} />
              <path d="M32 24 50 33 32 42 14 33Z" fill="#2E4BD8" fillOpacity={0.16} />
              <path d="M32 41 50 50 32 59 14 50Z" fill="#2E4BD8" fillOpacity={0.16} />
            </g>
          </svg>
          <span
            style={{
              fontFamily: "'Space Grotesk', 'IBM Plex Sans', sans-serif",
              fontWeight: 600,
              fontSize: '17px',
              letterSpacing: '-0.02em',
              color: 'var(--text-primary)',
            }}
          >
            Stratum
          </span>
        </span>
      </div>

      {/* Search trigger button */}
      <button
        onClick={onSearchOpen}
        className="flex items-center gap-2 flex-1 max-w-sm px-3 py-1.5 text-left"
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
          cursor: 'pointer',
        }}
      >
        <Search size={12} style={{ color: 'var(--text-muted)' }} />
        <span className="text-xs flex-1" style={{ color: 'var(--text-muted)', fontFamily: 'monospace' }}>
          Search…
        </span>
        <kbd
          className="text-xs font-mono"
          style={{
            color: 'var(--text-muted)',
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            padding: '1px 5px',
          }}
        >
          Ctrl+K
        </kbd>
      </button>

      <div className="flex-1" />

      {/* User + logout */}
      {user && (
        <div className="flex items-center gap-3">
          <div className="flex flex-col items-end">
            <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
              {user.username}
            </span>
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              {user.role}
            </span>
          </div>
          <button
            onClick={onLogout}
            className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs transition-colors"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
            title="Sign out"
          >
            <LogOut size={12} />
            Sign out
          </button>
        </div>
      )}
    </header>
  )
}
