import { diffLines, diffStats } from '../../lib/linediff'

interface DiffViewProps {
  oldText: string
  newText: string
}

const bgFor: Record<string, string> = {
  add: 'rgba(64,200,120,0.12)',
  remove: 'rgba(232,64,64,0.12)',
  equal: 'transparent',
}
const colorFor: Record<string, string> = {
  add: 'var(--status-ok, #40c878)',
  remove: 'var(--status-error)',
  equal: 'var(--text-secondary)',
}
const signFor: Record<string, string> = { add: '+', remove: '-', equal: ' ' }

// DiffView renders a unified line diff between the on-disk baseline and the
// edited content for the config editor's review-before-save step.
export function DiffView({ oldText, newText }: DiffViewProps) {
  const lines = diffLines(oldText, newText)
  const { added, removed } = diffStats(lines)

  if (added === 0 && removed === 0) {
    return (
      <div className="px-3 py-4 text-xs" style={{ color: 'var(--text-muted)' }}>
        No changes to save.
      </div>
    )
  }

  return (
    <div className="flex flex-col min-h-0">
      <div className="px-3 py-1.5 text-xs shrink-0" style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-subtle)' }}>
        <span style={{ color: 'var(--status-ok, #40c878)' }}>+{added}</span>{' '}
        <span style={{ color: 'var(--status-error)' }}>-{removed}</span> lines
      </div>
      <div className="flex-1 overflow-auto font-mono" style={{ fontSize: '12px' }}>
        {lines.map((l, idx) => (
          <div
            key={idx}
            className="flex"
            style={{ background: bgFor[l.op], color: colorFor[l.op], whiteSpace: 'pre' }}
          >
            <span style={{ width: '40px', textAlign: 'right', paddingRight: '8px', color: 'var(--text-muted)', userSelect: 'none' }}>
              {l.oldLine ?? ''}
            </span>
            <span style={{ width: '40px', textAlign: 'right', paddingRight: '8px', color: 'var(--text-muted)', userSelect: 'none' }}>
              {l.newLine ?? ''}
            </span>
            <span style={{ paddingRight: '6px', userSelect: 'none' }}>{signFor[l.op]}</span>
            <span>{l.text || ' '}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
