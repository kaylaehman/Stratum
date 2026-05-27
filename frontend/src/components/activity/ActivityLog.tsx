import { useState } from 'react'
import { ChevronDown, ChevronRight, Loader, Check, X } from 'lucide-react'
import { useActivity } from '../../lib/api/activity'
import { ActivityFilters } from './ActivityFilters'
import { ActivityDetail } from './ActivityDetail'
import type { ActivityEntry, ActivityFilters as Filters } from '../../types/api'

interface ActivityLogProps {
  filters: Filters
  onFiltersChange: (filters: Filters) => void
}

function ResultChip({ result }: { result: string }) {
  const isSuccess = result === 'success'
  const isError = result === 'error'

  const bg = isSuccess
    ? 'rgba(40,180,90,0.12)'
    : isError
      ? 'rgba(232,64,64,0.12)'
      : 'rgba(74,82,104,0.15)'

  const border = isSuccess
    ? 'rgba(40,180,90,0.35)'
    : isError
      ? 'rgba(232,64,64,0.35)'
      : 'var(--border-default)'

  const color = isSuccess
    ? '#28b45a'
    : isError
      ? 'var(--status-error)'
      : 'var(--text-muted)'

  return (
    <span
      className="flex items-center gap-1 font-mono text-xs px-1.5 py-0.5"
      style={{
        background: bg,
        border: `1px solid ${border}`,
        color,
        borderRadius: '3px',
        fontSize: '10px',
        whiteSpace: 'nowrap',
      }}
    >
      {isSuccess && <Check size={9} />}
      {isError && <X size={9} />}
      {result}
    </span>
  )
}

const tdBase: React.CSSProperties = {
  padding: '6px 12px',
  fontSize: '11px',
  borderBottom: '1px solid var(--border-subtle)',
  verticalAlign: 'middle',
}

interface EntryRowProps {
  entry: ActivityEntry
  expanded: boolean
  onToggle: () => void
}

function EntryRow({ entry, expanded, onToggle }: EntryRowProps) {
  const hasDetail = entry.detail !== null && entry.detail !== undefined
  const target =
    entry.target_type && entry.target_id
      ? `${entry.target_type}:${entry.target_id}`
      : entry.target_type ?? entry.target_id ?? '—'

  return (
    <>
      <tr
        style={{ cursor: hasDetail ? 'pointer' : 'default' }}
        onClick={hasDetail ? onToggle : undefined}
      >
        <td style={{ ...tdBase, width: '28px', paddingRight: 0 }}>
          {hasDetail ? (
            expanded ? (
              <ChevronDown size={11} style={{ color: 'var(--text-muted)' }} />
            ) : (
              <ChevronRight size={11} style={{ color: 'var(--text-muted)' }} />
            )
          ) : null}
        </td>
        <td style={{ ...tdBase, color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
          {new Date(entry.created_at).toLocaleString()}
        </td>
        <td
          style={{ ...tdBase, color: 'var(--text-secondary)', fontFamily: 'monospace' }}
        >
          {entry.username ?? entry.user_id ?? '—'}
        </td>
        <td style={{ ...tdBase, color: 'var(--text-primary)', fontFamily: 'monospace' }}>
          {entry.action}
        </td>
        <td
          style={{
            ...tdBase,
            color: 'var(--text-muted)',
            fontFamily: 'monospace',
            maxWidth: '200px',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {target}
        </td>
        <td style={{ ...tdBase }}>
          <ResultChip result={entry.result} />
        </td>
      </tr>
      {expanded && hasDetail && (
        <tr>
          <td
            colSpan={6}
            style={{
              padding: 0,
              borderBottom: '1px solid var(--border-subtle)',
              backgroundColor: 'var(--bg-elevated)',
            }}
          >
            <ActivityDetail entry={entry} />
          </td>
        </tr>
      )}
    </>
  )
}

export function ActivityLog({ filters, onFiltersChange }: ActivityLogProps) {
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set())
  const { data, isLoading, isFetchingNextPage, fetchNextPage, hasNextPage } =
    useActivity(filters)

  const entries = data?.pages.flatMap((p) => p.entries) ?? []

  function toggleExpanded(id: string) {
    setExpandedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  return (
    <div className="flex flex-col gap-3">
      <ActivityFilters filters={filters} onChange={onFiltersChange} />

      {isLoading && (
        <div className="flex items-center gap-2 py-6">
          <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
            Loading activity log...
          </span>
        </div>
      )}

      {!isLoading && entries.length === 0 && (
        <div
          className="px-3 py-4 text-xs"
          style={{
            color: 'var(--text-muted)',
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
          }}
        >
          No activity entries match the current filters.
        </div>
      )}

      {entries.length > 0 && (
        <div
          style={{
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            overflowX: 'auto',
          }}
        >
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                <th style={{ width: '28px', padding: '6px 0 6px 12px' }} />
                {['Time', 'User', 'Action', 'Target', 'Result'].map((col) => (
                  <th
                    key={col}
                    className="text-left text-xs uppercase tracking-wider font-medium"
                    style={{
                      padding: '6px 12px',
                      color: 'var(--text-muted)',
                      borderBottom: '1px solid var(--border-subtle)',
                    }}
                  >
                    {col}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {entries.map((entry) => (
                <EntryRow
                  key={entry.id}
                  entry={entry}
                  expanded={expandedIds.has(entry.id)}
                  onToggle={() => toggleExpanded(entry.id)}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Footer: count + load more */}
      {entries.length > 0 && (
        <div className="flex items-center justify-between px-1">
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
            {entries.length} {entries.length === 1 ? 'entry' : 'entries'}
          </span>
          {hasNextPage && (
            <button
              type="button"
              onClick={() => void fetchNextPage()}
              disabled={isFetchingNextPage}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: isFetchingNextPage ? 'default' : 'pointer',
                opacity: isFetchingNextPage ? 0.6 : 1,
              }}
            >
              {isFetchingNextPage && <Loader size={11} className="animate-spin" />}
              {isFetchingNextPage ? 'Loading...' : 'Load more'}
            </button>
          )}
        </div>
      )}
    </div>
  )
}
