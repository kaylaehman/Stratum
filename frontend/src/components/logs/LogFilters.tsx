import { useLogsStore } from '../../store/logs'

export function LogFilters() {
  const { filter, seenLevels, setFilterQuery, setFilterRegex, toggleFilterLevel } = useLogsStore()
  const [regexError, setRegexError] = useState<string | null>(null)

  function handleQueryChange(value: string) {
    setFilterQuery(value)
    if (filter.isRegex && value) {
      try {
        new RegExp(value)
        setRegexError(null)
      } catch {
        setRegexError('Invalid regex')
      }
    } else {
      setRegexError(null)
    }
  }

  function handleRegexToggle() {
    const next = !filter.isRegex
    setFilterRegex(next)
    if (next && filter.query) {
      try {
        new RegExp(filter.query)
        setRegexError(null)
      } catch {
        setRegexError('Invalid regex')
      }
    } else {
      setRegexError(null)
    }
  }

  const sortedLevels = [...seenLevels].sort()

  return (
    <div className="flex items-center gap-2 flex-wrap">
      {/* Text filter */}
      <div className="flex items-center gap-0">
        <input
          type="text"
          placeholder={filter.isRegex ? 'Filter regex…' : 'Filter text…'}
          value={filter.query}
          onChange={(e) => handleQueryChange(e.target.value)}
          className="text-xs px-2 py-1 font-mono"
          style={{
            background: 'var(--bg-elevated)',
            border: `1px solid ${regexError ? 'var(--status-error)' : 'var(--border-default)'}`,
            borderRight: 'none',
            color: 'var(--text-primary)',
            borderRadius: '3px 0 0 3px',
            outline: 'none',
            width: '180px',
          }}
        />
        <button
          type="button"
          onClick={handleRegexToggle}
          className="px-2 py-1 text-xs font-mono shrink-0"
          style={{
            background: filter.isRegex ? 'var(--accent-glow)' : 'var(--bg-elevated)',
            border: `1px solid ${filter.isRegex ? 'var(--accent-dim)' : 'var(--border-default)'}`,
            color: filter.isRegex ? 'var(--accent)' : 'var(--text-muted)',
            borderRadius: '0 3px 3px 0',
            cursor: 'pointer',
          }}
          title={filter.isRegex ? 'Regex mode (click to switch to substring)' : 'Substring mode (click to switch to regex)'}
        >
          .*
        </button>
      </div>

      {/* Regex error */}
      {regexError && (
        <span className="text-xs" style={{ color: 'var(--status-error)' }}>
          {regexError}
        </span>
      )}

      {/* Level filter chips (only shown when levels have been detected) */}
      {sortedLevels.length > 0 && (
        <div className="flex items-center gap-1">
          <span className="text-xs shrink-0" style={{ color: 'var(--text-muted)' }}>
            Level:
          </span>
          {sortedLevels.map((lvl) => {
            const active = filter.levels.has(lvl)
            return (
              <button
                key={lvl}
                type="button"
                onClick={() => toggleFilterLevel(lvl)}
                className="px-2 py-0.5 text-xs rounded-sm"
                style={{
                  background: active ? 'var(--accent-glow)' : 'var(--bg-elevated)',
                  border: `1px solid ${active ? 'var(--accent-dim)' : 'var(--border-subtle)'}`,
                  color: active ? 'var(--accent)' : 'var(--text-secondary)',
                  cursor: 'pointer',
                }}
              >
                {lvl}
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}

// Need useState for the regex error local state
import { useState } from 'react'
