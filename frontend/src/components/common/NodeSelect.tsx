import type { CSSProperties } from 'react'

/** Minimal shape a node needs to appear in the picker. */
export interface NodeOption {
  id: string
  name: string
  host?: string
}

interface NodeSelectProps {
  nodes: NodeOption[]
  value: string
  onChange: (nodeId: string) => void
  /** Append "(host)" to each option label (Terminal-style). */
  showHost?: boolean
  /** Option shown when the list is empty. */
  emptyLabel?: string
  /** Optional leading "Select…" placeholder option (value ""). */
  placeholder?: string
  /** Sort by name (case-insensitive). Default true. */
  sort?: boolean
  style?: CSSProperties
  className?: string
  disabled?: boolean
  'aria-label'?: string
}

const defaultStyle: CSSProperties = {
  backgroundColor: 'var(--bg-elevated)',
  border: '1px solid var(--border-default)',
  color: 'var(--text-primary)',
  borderRadius: '3px',
  cursor: 'pointer',
  outline: 'none',
}

/**
 * NodeSelect is the shared node-picker used across pages that act on a single
 * node (Terminal, Backups, …) — replacing several copy-pasted `<select>` blocks
 * that each re-implemented the same sort + option rendering + empty handling.
 * Behaviour-preserving: callers keep their own styling and onChange side effects.
 */
export function NodeSelect({
  nodes,
  value,
  onChange,
  showHost = false,
  emptyLabel,
  placeholder,
  sort = true,
  style,
  className = 'text-xs font-mono px-2 py-1',
  disabled,
  'aria-label': ariaLabel,
}: NodeSelectProps) {
  const ordered = sort
    ? [...nodes].sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
    : nodes

  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className={className}
      style={style ?? defaultStyle}
      disabled={disabled}
      aria-label={ariaLabel ?? 'Select node'}
    >
      {ordered.length === 0 && emptyLabel && <option value="">{emptyLabel}</option>}
      {placeholder && <option value="">{placeholder}</option>}
      {ordered.map((n) => (
        <option key={n.id} value={n.id}>
          {showHost && n.host ? `${n.name} (${n.host})` : n.name}
        </option>
      ))}
    </select>
  )
}
