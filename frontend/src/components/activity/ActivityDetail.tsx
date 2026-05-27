import type { ActivityEntry } from '../../types/api'

interface ActivityDetailProps {
  entry: ActivityEntry
}

function isDiffDetail(val: unknown): val is { before: unknown; after: unknown } {
  return (
    typeof val === 'object' &&
    val !== null &&
    'before' in val &&
    'after' in val
  )
}

function PrettyValue({ val }: { val: unknown }) {
  if (val === null || val === undefined) {
    return <span style={{ color: 'var(--text-muted)' }}>null</span>
  }
  if (typeof val === 'object') {
    return (
      <pre
        className="font-mono text-xs"
        style={{ color: 'var(--text-secondary)', margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}
      >
        {JSON.stringify(val, null, 2)}
      </pre>
    )
  }
  return (
    <span className="font-mono text-xs" style={{ color: 'var(--text-secondary)' }}>
      {String(val)}
    </span>
  )
}

export function ActivityDetail({ entry }: ActivityDetailProps) {
  if (entry.detail === null || entry.detail === undefined) return null

  const cellStyle: React.CSSProperties = {
    padding: '4px 8px',
    verticalAlign: 'top',
    borderBottom: '1px solid var(--border-subtle)',
    fontSize: '11px',
  }

  if (isDiffDetail(entry.detail)) {
    return (
      <div style={{ padding: '8px 12px' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              {['Before', 'After'].map((h) => (
                <th
                  key={h}
                  style={{
                    ...cellStyle,
                    color: 'var(--text-muted)',
                    fontWeight: 500,
                    textTransform: 'uppercase',
                    letterSpacing: '0.05em',
                    borderBottom: '1px solid var(--border-default)',
                  }}
                  className="text-left"
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            <tr>
              <td style={{ ...cellStyle, width: '50%' }}>
                <PrettyValue val={entry.detail.before} />
              </td>
              <td style={{ ...cellStyle, width: '50%' }}>
                <PrettyValue val={entry.detail.after} />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    )
  }

  if (typeof entry.detail === 'object' && entry.detail !== null && !Array.isArray(entry.detail)) {
    const obj = entry.detail as Record<string, unknown>
    const keys = Object.keys(obj)
    return (
      <div style={{ padding: '8px 12px' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <tbody>
            {keys.map((k) => (
              <tr key={k}>
                <td
                  style={{
                    ...cellStyle,
                    color: 'var(--text-muted)',
                    width: '160px',
                    fontFamily: 'monospace',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {k}
                </td>
                <td style={cellStyle}>
                  <PrettyValue val={obj[k]} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }

  return (
    <div style={{ padding: '8px 12px' }}>
      <pre
        className="font-mono text-xs"
        style={{
          color: 'var(--text-secondary)',
          margin: 0,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
        }}
      >
        {JSON.stringify(entry.detail, null, 2)}
      </pre>
    </div>
  )
}
