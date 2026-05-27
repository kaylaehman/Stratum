import { Filter } from 'lucide-react'
import { useActivityActions } from '../../lib/api/activity'
import type { ActivityFilters as Filters } from '../../types/api'

interface ActivityFiltersProps {
  filters: Filters
  onChange: (filters: Filters) => void
}

const inputStyle: React.CSSProperties = {
  backgroundColor: 'var(--bg-elevated)',
  border: '1px solid var(--border-default)',
  color: 'var(--text-primary)',
  borderRadius: '3px',
  fontSize: '11px',
  padding: '4px 8px',
  fontFamily: 'inherit',
  outline: 'none',
}

export function ActivityFilters({ filters, onChange }: ActivityFiltersProps) {
  const { data: actionsData } = useActivityActions()
  const actions = actionsData?.actions ?? []

  function set(partial: Partial<Filters>) {
    onChange({ ...filters, ...partial })
  }

  return (
    <div
      className="flex flex-wrap items-center gap-2 px-3 py-2.5"
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
      }}
    >
      <Filter size={11} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />

      {/* Action dropdown */}
      <select
        value={filters.action ?? ''}
        onChange={(e) => set({ action: e.target.value || undefined })}
        style={{ ...inputStyle, minWidth: '160px' }}
      >
        <option value="">All actions</option>
        {actions.map((a) => (
          <option key={a.action} value={a.action}>
            {a.label}
          </option>
        ))}
      </select>

      {/* Result select */}
      <select
        value={filters.result ?? ''}
        onChange={(e) => set({ result: e.target.value || undefined })}
        style={{ ...inputStyle, minWidth: '100px' }}
      >
        <option value="">Any result</option>
        <option value="success">Success</option>
        <option value="error">Error</option>
      </select>

      {/* Free-text search */}
      <input
        type="text"
        placeholder="Search..."
        value={filters.q ?? ''}
        onChange={(e) => set({ q: e.target.value || undefined })}
        style={{ ...inputStyle, minWidth: '160px' }}
      />

      {/* Date from */}
      <input
        type="date"
        value={filters.from ?? ''}
        onChange={(e) => set({ from: e.target.value || undefined })}
        style={{ ...inputStyle, colorScheme: 'dark' }}
      />

      {/* Date to */}
      <input
        type="date"
        value={filters.to ?? ''}
        onChange={(e) => set({ to: e.target.value || undefined })}
        style={{ ...inputStyle, colorScheme: 'dark' }}
      />

      {/* Clear */}
      {(filters.action || filters.result || filters.q || filters.from || filters.to) && (
        <button
          type="button"
          onClick={() => onChange({})}
          className="text-xs px-2 py-1"
          style={{
            backgroundColor: 'transparent',
            border: '1px solid var(--border-default)',
            color: 'var(--text-muted)',
            borderRadius: '3px',
            cursor: 'pointer',
          }}
        >
          Clear
        </button>
      )}
    </div>
  )
}
