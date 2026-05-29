import { LogOut, Search } from 'lucide-react'
import type { User } from '../../types/api'

interface TopbarProps {
  user: User | null
  onLogout: () => void
  onSearchOpen?: () => void
}

export function Topbar({ user, onLogout, onSearchOpen }: TopbarProps) {
  return (
    <header
      className="h-11 flex items-center px-4 gap-4 shrink-0"
      style={{
        backgroundColor: 'var(--bg-surface)',
        borderBottom: '1px solid var(--border-subtle)',
      }}
    >
      {/* Brand — logo includes the wordmark, no adjacent text needed. */}
      <div className="flex items-center w-52 shrink-0">
        <img src="/logo.png?v=2" alt="Stratum" height={28} style={{ height: 28, width: 'auto', display: 'block' }} />
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
