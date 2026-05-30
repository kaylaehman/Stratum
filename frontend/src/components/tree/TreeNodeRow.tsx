import { ChevronRight, ChevronDown } from 'lucide-react'
import type { ReactNode } from 'react'
import { StatusBadge } from './StatusBadge'

interface TreeNodeRowProps {
  depth: number
  icon: ReactNode
  label: string
  sublabel?: string
  status?: string
  stale?: boolean
  expandable?: boolean
  expanded?: boolean
  selected?: boolean
  /** Optional badge rendered before the status dot (e.g. security flag indicator). */
  badge?: ReactNode
  onToggle?: () => void
  onClick?: () => void
}

const INDENT = 16

export function TreeNodeRow({
  depth,
  icon,
  label,
  sublabel,
  status,
  stale,
  expandable,
  expanded,
  selected,
  badge,
  onToggle,
  onClick,
}: TreeNodeRowProps) {
  return (
    <div
      role="row"
      aria-selected={selected}
      onClick={onClick}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '5px',
        paddingLeft: `${8 + depth * INDENT}px`,
        paddingRight: '8px',
        paddingTop: '3px',
        paddingBottom: '3px',
        cursor: 'pointer',
        backgroundColor: selected ? 'var(--accent-glow)' : 'transparent',
        borderLeft: selected ? '2px solid var(--accent)' : '2px solid transparent',
        userSelect: 'none',
      }}
      onMouseEnter={(e) => {
        if (!selected) {
          ;(e.currentTarget as HTMLDivElement).style.backgroundColor = 'var(--bg-elevated)'
        }
      }}
      onMouseLeave={(e) => {
        if (!selected) {
          ;(e.currentTarget as HTMLDivElement).style.backgroundColor = 'transparent'
        }
      }}
    >
      {/* Expand chevron */}
      <span
        style={{ width: '12px', height: '12px', flexShrink: 0, color: 'var(--text-muted)' }}
        onClick={(e) => {
          if (expandable && onToggle) {
            e.stopPropagation()
            onToggle()
          }
        }}
      >
        {expandable ? (
          expanded ? (
            <ChevronDown size={12} strokeWidth={1.5} />
          ) : (
            <ChevronRight size={12} strokeWidth={1.5} />
          )
        ) : null}
      </span>

      {/* Type icon */}
      <span style={{ flexShrink: 0 }}>{icon}</span>

      {/* Name */}
      <span
        className="font-mono text-xs truncate flex-1"
        style={{ color: 'var(--text-primary)', minWidth: 0 }}
        title={label}
      >
        {label}
      </span>

      {/* Sublabel */}
      {sublabel && (
        <span
          className="font-mono text-xs truncate"
          style={{ color: 'var(--text-muted)', fontSize: '12px', flexShrink: 1 }}
          title={sublabel}
        >
          {sublabel}
        </span>
      )}

      {/* Security badge (e.g. shield icon for flagged containers) */}
      {badge && <span style={{ flexShrink: 0, display: 'flex', alignItems: 'center' }}>{badge}</span>}

      {/* Status dot */}
      {status && (
        <StatusBadge status={status} stale={stale} dot />
      )}
    </div>
  )
}
