import { useState, useRef, useEffect } from 'react'
import { Plus, X, ChevronDown } from 'lucide-react'
import { useTree } from '../../lib/api/tree'
import { useLogsStore } from '../../store/logs'
import type { SelectedContainer } from '../../store/logs'

export function ContainerPicker() {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const { data: tree } = useTree()
  const { selectedContainers, addContainer, removeContainer, colorMap } = useLogsStore()

  // Flatten all running containers from tree
  const allContainers: SelectedContainer[] = (tree?.nodes ?? []).flatMap((node) =>
    node.containers
      .filter((c) => c.status === 'running')
      .map((c) => ({ uuid: c.id, dockerId: c.docker_id, name: c.name })),
  )

  // Close dropdown on outside click
  useEffect(() => {
    if (!open) return
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [open])

  const isSelected = (uuid: string) => selectedContainers.some((c) => c.uuid === uuid)

  return (
    <div className="flex items-center gap-1.5 flex-wrap" ref={ref}>
      {/* Selected chips */}
      {selectedContainers.map((c) => {
        const color = colorMap[c.dockerId] ?? 'var(--accent)'
        return (
          <div
            key={c.uuid}
            className="flex items-center gap-1 px-2 py-0.5 rounded-sm text-xs font-mono"
            style={{
              background: `${color}18`,
              border: `1px solid ${color}50`,
              color,
            }}
          >
            <span
              className="inline-block rounded-full shrink-0"
              style={{ width: '6px', height: '6px', backgroundColor: color }}
            />
            <span className="max-w-[120px] truncate">{c.name}</span>
            <button
              type="button"
              onClick={() => removeContainer(c.dockerId)}
              className="opacity-60 hover:opacity-100 ml-0.5"
              style={{ lineHeight: 1 }}
              aria-label={`Remove ${c.name}`}
            >
              <X size={10} />
            </button>
          </div>
        )
      })}

      {/* Add button */}
      <div className="relative">
        <button
          type="button"
          className="flex items-center gap-1 px-2 py-1 rounded-sm text-xs"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            cursor: 'pointer',
          }}
          onClick={() => setOpen((v) => !v)}
          aria-haspopup="listbox"
          aria-expanded={open}
        >
          <Plus size={11} />
          Add container
          <ChevronDown size={10} />
        </button>

        {open && (
          <div
            className="absolute left-0 top-full mt-1 z-50 min-w-[220px] max-h-60 overflow-y-auto"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              borderRadius: '3px',
              boxShadow: '0 4px 16px rgba(0,0,0,0.5)',
            }}
          >
            {allContainers.length === 0 ? (
              <div
                className="px-3 py-2 text-xs"
                style={{ color: 'var(--text-muted)' }}
              >
                No running containers found
              </div>
            ) : (
              allContainers.map((c) => {
                const sel = isSelected(c.uuid)
                return (
                  <button
                    key={c.uuid}
                    type="button"
                    role="option"
                    aria-selected={sel}
                    className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-left"
                    style={{
                      background: sel ? 'var(--accent-glow)' : 'transparent',
                      color: sel ? 'var(--accent)' : 'var(--text-primary)',
                      cursor: 'pointer',
                    }}
                    onClick={() => {
                      if (sel) {
                        removeContainer(c.dockerId)
                      } else {
                        addContainer(c)
                        setOpen(false)
                      }
                    }}
                  >
                    <span className="font-mono truncate flex-1">{c.name}</span>
                    {sel && <X size={10} />}
                  </button>
                )
              })
            )}
          </div>
        )}
      </div>
    </div>
  )
}
