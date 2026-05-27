import type { UidRow, UidRowClass } from '../../types/api'

interface UserTableProps {
  rows: UidRow[]
  side: 'host' | 'container'
  /** When true, tint rows by their class (used in aligned diff view) */
  tinted?: boolean
}

function rowBackground(cls: UidRowClass): string {
  switch (cls) {
    case 'match':
      return 'rgba(34,201,122,0.07)'
    case 'mismatch':
      return 'rgba(240,160,32,0.10)'
    case 'unresolvable':
      return 'rgba(232,64,64,0.08)'
  }
}

function nameColor(cls: UidRowClass, missing: boolean): string {
  if (missing) return 'var(--text-muted)'
  switch (cls) {
    case 'match':
      return 'var(--status-ok)'
    case 'mismatch':
      return 'var(--status-warn)'
    case 'unresolvable':
      return 'var(--status-error)'
  }
}

interface TableRowProps {
  row: UidRow
  side: 'host' | 'container'
  tinted: boolean
}

function TableRow({ row, side, tinted }: TableRowProps) {
  const name = side === 'host' ? row.host_name : row.container_name
  const present = side === 'host' ? row.on_host : row.on_container
  const displayName = name ?? (present ? '(no name)' : '--')

  return (
    <div
      className="flex items-center gap-3 px-3 py-1"
      style={{
        borderBottom: '1px solid var(--border-subtle)',
        backgroundColor: tinted ? rowBackground(row.class) : undefined,
        minHeight: '28px',
      }}
    >
      <span
        className="font-mono text-xs shrink-0"
        style={{ color: 'var(--text-secondary)', width: '48px' }}
      >
        {row.id}
      </span>
      <span
        className="font-mono text-xs truncate flex-1"
        style={{
          color: tinted ? nameColor(row.class, !present || !name) : 'var(--text-primary)',
          fontStyle: !present ? 'italic' : undefined,
        }}
      >
        {displayName}
      </span>
    </div>
  )
}

export function UserTable({ rows, side, tinted = false }: UserTableProps) {
  return (
    <div className="flex flex-col flex-1 min-w-0">
      {/* Column header */}
      <div
        className="flex items-center gap-3 px-3 py-1.5 shrink-0"
        style={{
          borderBottom: '1px solid var(--border-default)',
          backgroundColor: 'var(--bg-elevated)',
        }}
      >
        <span
          className="font-mono text-xs shrink-0"
          style={{ color: 'var(--text-muted)', width: '48px' }}
        >
          UID
        </span>
        <span className="font-mono text-xs flex-1" style={{ color: 'var(--text-muted)' }}>
          {side === 'host' ? 'Host username' : 'Container username'}
        </span>
      </div>

      <div style={{ overflowY: 'auto', flex: 1 }}>
        {rows.map((row) => (
          <TableRow key={row.id} row={row} side={side} tinted={tinted} />
        ))}
        {rows.length === 0 && (
          <div className="px-3 py-3 text-xs" style={{ color: 'var(--text-muted)' }}>
            No entries.
          </div>
        )}
      </div>
    </div>
  )
}
