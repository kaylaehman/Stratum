import { useTree } from '../../lib/api/tree'

// A small stable palette — cycles when more containers selected than colors
const PALETTE = [
  '#00c2cc', // accent teal
  '#f0a020', // warn amber
  '#7c6af7', // indigo
  '#4ade80', // green
  '#f472b6', // pink
  '#fb923c', // orange
  '#38bdf8', // sky
  '#a78bfa', // violet
]

export function containerColor(index: number): string {
  return PALETTE[index % PALETTE.length]
}

interface ContainerPickerProps {
  selectedIds: string[]
  onChange: (ids: string[]) => void
}

export function ContainerPicker({ selectedIds, onChange }: ContainerPickerProps) {
  const { data: tree } = useTree()
  const nodes = tree?.nodes ?? []

  function toggle(id: string) {
    if (selectedIds.includes(id)) {
      onChange(selectedIds.filter((x) => x !== id))
    } else {
      onChange([...selectedIds, id])
    }
  }

  if (nodes.length === 0) {
    return (
      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
        No nodes connected.
      </span>
    )
  }

  // Build flat list preserving order for stable color assignment
  const allContainerIds = nodes.flatMap((n) => n.containers.map((c) => c.id))

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
      {nodes.map((node) => {
        if (node.containers.length === 0) return null
        return (
          <div key={node.id}>
            <div
              className="text-xs uppercase tracking-wider mb-1.5"
              style={{ color: 'var(--text-muted)', fontWeight: 500 }}
            >
              {node.name}
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              {node.containers.map((c) => {
                const checked = selectedIds.includes(c.id)
                const colorIdx = allContainerIds.indexOf(c.id)
                const color = containerColor(colorIdx)
                return (
                  <label
                    key={c.id}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '8px',
                      cursor: 'pointer',
                      padding: '3px 6px',
                      borderRadius: '3px',
                      backgroundColor: checked ? 'var(--accent-glow)' : 'transparent',
                      border: `1px solid ${checked ? 'rgba(0,194,204,0.25)' : 'transparent'}`,
                    }}
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => toggle(c.id)}
                      style={{ accentColor: color, width: '12px', height: '12px', cursor: 'pointer' }}
                    />
                    {checked && (
                      <span
                        style={{
                          width: '8px',
                          height: '8px',
                          borderRadius: '50%',
                          backgroundColor: color,
                          flexShrink: 0,
                        }}
                      />
                    )}
                    <span
                      className="font-mono text-xs truncate"
                      style={{ color: checked ? 'var(--text-primary)' : 'var(--text-secondary)', maxWidth: '160px' }}
                      title={c.name}
                    >
                      {c.name}
                    </span>
                    <span
                      className="font-mono text-xs ml-auto"
                      style={{
                        color: c.status === 'running' ? 'var(--accent)' : 'var(--text-muted)',
                        fontSize: '12px',
                        textTransform: 'uppercase',
                        letterSpacing: '0.05em',
                        flexShrink: 0,
                      }}
                    >
                      {c.status}
                    </span>
                  </label>
                )
              })}
            </div>
          </div>
        )
      })}
    </div>
  )
}
