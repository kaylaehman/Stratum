import { useEffect, useRef, useState, useCallback, useMemo } from 'react'
import { Pause, Play, Download, Trash2 } from 'lucide-react'
import { wsManager } from '../../lib/ws'
import { useTree } from '../../lib/api/tree'
import { useLogsStore } from '../../store/logs'
import type { SelectedContainer } from '../../store/logs'
import type { LogSubscribeRequest, LogSubscribeResponse } from '../../types/api'
import { apiPost } from '../../lib/api'
import { ContainerPicker } from './ContainerPicker'
import { LogFilters } from './LogFilters'
import { LogLineRow } from './LogLineRow'

// --- Filtered view ---

const ROW_HEIGHT = 20 // px per log line
const OVERSCAN = 10  // extra rows above/below viewport

function useFilteredLines() {
  const { lines, filter } = useLogsStore()

  return useMemo(() => {
    const { query, isRegex, levels } = filter
    let result = lines

    if (levels.size > 0) {
      result = result.filter((l) => l.level && levels.has(l.level))
    }

    if (query) {
      if (isRegex) {
        try {
          const re = new RegExp(query)
          result = result.filter((l) => re.test(l.text))
        } catch {
          // invalid regex: show all lines
        }
      } else {
        const lower = query.toLowerCase()
        result = result.filter((l) => l.text.toLowerCase().includes(lower))
      }
    }

    return result
  }, [lines, filter])
}

// --- Virtualized log list ---

interface VirtualizedLogListProps {
  scrollRef: React.RefObject<HTMLDivElement | null>
}

function VirtualizedLogList({ scrollRef }: VirtualizedLogListProps) {
  const filteredLines = useFilteredLines()
  const { colorMap, selectedContainers, paused, filter } = useLogsStore()
  const [scrollTop, setScrollTop] = useState(0)
  const [viewportHeight, setViewportHeight] = useState(400)

  // Observe container resize
  useEffect(() => {
    const el = scrollRef.current
    if (!el) return
    const ro = new ResizeObserver((entries) => {
      setViewportHeight(entries[0]?.contentRect.height ?? 400)
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [scrollRef])

  // Track scroll position
  useEffect(() => {
    const el = scrollRef.current
    if (!el) return
    const handler = () => setScrollTop(el.scrollTop)
    el.addEventListener('scroll', handler, { passive: true })
    return () => el.removeEventListener('scroll', handler)
  }, [scrollRef])

  // Auto-scroll to bottom when not paused
  useEffect(() => {
    if (!paused) {
      const el = scrollRef.current
      if (el) {
        el.scrollTop = el.scrollHeight
      }
    }
  }, [filteredLines.length, paused, scrollRef])

  const totalHeight = filteredLines.length * ROW_HEIGHT
  const startIdx = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN)
  const visibleCount = Math.ceil(viewportHeight / ROW_HEIGHT) + OVERSCAN * 2
  const endIdx = Math.min(filteredLines.length, startIdx + visibleCount)

  const containerNameMap = useMemo(() => {
    const m: Record<string, string> = {}
    for (const c of selectedContainers) {
      m[c.dockerId] = c.name
    }
    return m
  }, [selectedContainers])

  return (
    <div style={{ height: `${totalHeight}px`, position: 'relative' }}>
      <div style={{ transform: `translateY(${startIdx * ROW_HEIGHT}px)` }}>
        {filteredLines.slice(startIdx, endIdx).map((line, i) => {
          const color = colorMap[line.container_id] ?? 'var(--accent)'
          const name = containerNameMap[line.container_id] ?? line.container_id.slice(0, 12)
          return (
            <LogLineRow
              key={startIdx + i}
              line={line}
              color={color}
              containerName={name}
              searchQuery={filter.query}
              searchIsRegex={filter.isRegex}
            />
          )
        })}
      </div>
    </div>
  )
}

// --- Subscription manager ---

function useLogSubscriptions() {
  const { selectedContainers } = useLogsStore()
  const addLine = useLogsStore((s) => s.addLine)
  const subscribedRef = useRef<Set<string>>(new Set())

  // Register log-line handler once
  useEffect(() => {
    const unsub = wsManager.onLogLine((line) => {
      addLine(line)
    })
    return unsub
  }, [addLine])

  // Subscribe / unsubscribe as selectedContainers changes
  useEffect(() => {
    const currentUuids = new Set(selectedContainers.map((c) => c.uuid))

    async function subscribe(c: SelectedContainer) {
      // Wait for client_id if not yet available (retry with brief polling)
      let clientId = wsManager.clientId()
      if (!clientId) {
        await new Promise<void>((resolve) => {
          const unsub = wsManager.onOpen(() => {
            unsub()
            resolve()
          })
          // If already open, client_id may arrive shortly after hello; poll briefly
          let attempts = 0
          const poll = setInterval(() => {
            if (wsManager.clientId() || attempts++ > 20) {
              clearInterval(poll)
              resolve()
            }
          }, 100)
        })
        clientId = wsManager.clientId()
      }
      if (!clientId) return // still no client_id; skip

      try {
        await apiPost<LogSubscribeResponse>('/api/logs/subscribe', {
          container_id: c.uuid,
          ws_client_id: clientId,
        } satisfies LogSubscribeRequest)
        subscribedRef.current.add(c.uuid)
      } catch {
        // subscribe failed (container not running, unauthorized, etc.)
      }
    }

    async function unsubscribe(uuid: string) {
      const clientId = wsManager.clientId()
      if (!clientId) return
      try {
        await apiPost<void>('/api/logs/unsubscribe', {
          container_id: uuid,
          ws_client_id: clientId,
        } satisfies LogSubscribeRequest)
      } catch {
        // best-effort
      }
      subscribedRef.current.delete(uuid)
    }

    // New containers to subscribe
    for (const c of selectedContainers) {
      if (!subscribedRef.current.has(c.uuid)) {
        void subscribe(c)
      }
    }

    // Removed containers to unsubscribe
    for (const uuid of [...subscribedRef.current]) {
      if (!currentUuids.has(uuid)) {
        void unsubscribe(uuid)
      }
    }
  }, [selectedContainers])

  // Unsubscribe all on unmount
  useEffect(() => {
    const ref = subscribedRef
    return () => {
      const clientId = wsManager.clientId()
      if (!clientId) return
      for (const uuid of [...ref.current]) {
        apiPost<void>('/api/logs/unsubscribe', {
          container_id: uuid,
          ws_client_id: clientId,
        } satisfies LogSubscribeRequest).catch(() => {
          // best-effort
        })
      }
      ref.current.clear()
    }
  }, [])
}

// --- Color legend ---

function ColorLegend() {
  const { selectedContainers, colorMap } = useLogsStore()
  if (selectedContainers.length === 0) return null

  return (
    <div className="flex items-center gap-3 flex-wrap">
      {selectedContainers.map((c) => {
        const color = colorMap[c.dockerId] ?? 'var(--accent)'
        return (
          <div key={c.uuid} className="flex items-center gap-1.5">
            <span
              className="inline-block rounded-full shrink-0"
              style={{ width: '8px', height: '8px', backgroundColor: color }}
            />
            <span className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
              {c.name}
            </span>
          </div>
        )
      })}
    </div>
  )
}

// --- Download helper ---

function useDownload(getLines: () => string) {
  return useCallback(() => {
    const content = getLines()
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `stratum-logs-${new Date().toISOString().slice(0, 19)}.log`
    a.click()
    URL.revokeObjectURL(url)
  }, [getLines])
}

// --- Root component ---

interface LogViewerProps {
  /** Optional pre-selected container to add on mount (e.g. from tree "View logs" action). */
  initialContainer?: SelectedContainer
}

export function LogViewer({ initialContainer }: LogViewerProps) {
  const { paused, togglePause, clear, selectedContainers, lines } = useLogsStore()
  const addContainer = useLogsStore((s) => s.addContainer)
  const filteredLines = useFilteredLines()
  const [jumpToLive, setJumpToLive] = useState(false)
  const scrollRef = useRef<HTMLDivElement | null>(null)
  const { data: tree } = useTree()
  const didDefaultSelectRef = useRef(false)

  // Ensure ws is connected
  useEffect(() => {
    wsManager.connect()
  }, [])

  // Add initial container if provided
  useEffect(() => {
    if (initialContainer) {
      addContainer(initialContainer)
    }
  }, [initialContainer, addContainer])

  // Default to streaming ALL running containers on first load.
  // Runs exactly once, after the tree first becomes available, so that
  // containers the user later removes are not re-added on tree refreshes.
  useEffect(() => {
    if (didDefaultSelectRef.current) return
    if (!tree) return
    didDefaultSelectRef.current = true

    const running: SelectedContainer[] = tree.nodes.flatMap((node) =>
      node.containers
        .filter((c) => c.status === 'running')
        .map((c) => ({ uuid: c.id, dockerId: c.docker_id, name: c.name })),
    )
    // addContainer is idempotent (dedupes by uuid), so an initialContainer
    // already added above will not be duplicated.
    for (const c of running) {
      addContainer(c)
    }
  }, [tree, addContainer])

  // Show "jump to live" banner when paused and new lines arrive
  useEffect(() => {
    if (paused) {
      setJumpToLive(true)
    } else {
      setJumpToLive(false)
    }
  }, [paused])

  useLogSubscriptions()

  const containerNameMap = useMemo(() => {
    const m: Record<string, string> = {}
    for (const c of selectedContainers) {
      m[c.dockerId] = c.name
    }
    return m
  }, [selectedContainers])

  const getDownloadContent = useCallback(() => {
    return filteredLines
      .map((l) => {
        const name = containerNameMap[l.container_id] ?? l.container_id.slice(0, 12)
        const ts = l.ts ? new Date(l.ts).toISOString() : ''
        return `${ts}\t[${name}]\t${l.stream}\t${l.text}`
      })
      .join('\n')
  }, [filteredLines, containerNameMap])

  const handleDownload = useDownload(getDownloadContent)

  function handleResumeJumpToLive() {
    togglePause()
    setJumpToLive(false)
    // Scroll to bottom after resuming
    requestAnimationFrame(() => {
      if (scrollRef.current) {
        scrollRef.current.scrollTop = scrollRef.current.scrollHeight
      }
    })
  }

  return (
    <div
      className="flex flex-col h-full"
      style={{ backgroundColor: 'var(--bg-base)' }}
    >
      {/* Toolbar */}
      <div
        className="shrink-0 px-4 py-2.5 flex flex-col gap-2"
        style={{
          backgroundColor: 'var(--bg-surface)',
          borderBottom: '1px solid var(--border-subtle)',
        }}
      >
        {/* Row 1: container picker + actions */}
        <div className="flex items-center gap-2 flex-wrap">
          <ContainerPicker />
          <div className="flex items-center gap-1.5 ml-auto">
            <button
              type="button"
              onClick={togglePause}
              className="flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-sm"
              style={{
                background: paused ? 'rgba(240,160,32,0.15)' : 'var(--bg-elevated)',
                border: `1px solid ${paused ? 'var(--status-warn)' : 'var(--border-default)'}`,
                color: paused ? 'var(--status-warn)' : 'var(--text-secondary)',
                cursor: 'pointer',
              }}
              title={paused ? 'Resume log stream' : 'Pause log stream'}
            >
              {paused ? <Play size={11} /> : <Pause size={11} />}
              {paused ? 'Resume' : 'Pause'}
            </button>
            <button
              type="button"
              onClick={handleDownload}
              className="flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-sm"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                cursor: 'pointer',
              }}
              title="Download current filtered view as .log"
              disabled={filteredLines.length === 0}
            >
              <Download size={11} />
              Download
            </button>
            <button
              type="button"
              onClick={clear}
              className="flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-sm"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                cursor: 'pointer',
              }}
              title="Clear log buffer"
            >
              <Trash2 size={11} />
              Clear
            </button>
          </div>
        </div>

        {/* Row 2: filters */}
        <LogFilters />

        {/* Row 3: color legend */}
        <ColorLegend />
      </div>

      {/* Pause banner */}
      {jumpToLive && paused && (
        <div
          className="shrink-0 flex items-center justify-between px-4 py-1.5"
          style={{
            backgroundColor: 'rgba(240,160,32,0.10)',
            borderBottom: '1px solid var(--status-warn)',
          }}
        >
          <span className="text-xs" style={{ color: 'var(--status-warn)' }}>
            Stream paused. Buffer is filling in the background.
          </span>
          <button
            type="button"
            onClick={handleResumeJumpToLive}
            className="text-xs px-2 py-0.5"
            style={{
              background: 'rgba(240,160,32,0.15)',
              border: '1px solid var(--status-warn)',
              color: 'var(--status-warn)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Resume and jump to live
          </button>
        </div>
      )}

      {/* Log body */}
      <div
        ref={scrollRef}
        className="flex-1 overflow-auto"
        style={{
          backgroundColor: 'var(--bg-base)',
          fontFamily: "'IBM Plex Mono', monospace",
        }}
      >
        {selectedContainers.length === 0 ? (
          <div
            className="flex items-center justify-center h-full"
            style={{ color: 'var(--text-muted)' }}
          >
            <span className="text-xs">Add a container above to start streaming logs.</span>
          </div>
        ) : lines.length === 0 ? (
          <div
            className="flex items-center justify-center h-full"
            style={{ color: 'var(--text-muted)' }}
          >
            <span className="text-xs">Waiting for log lines...</span>
          </div>
        ) : (
          <VirtualizedLogList scrollRef={scrollRef} />
        )}
      </div>

      {/* Status bar */}
      <div
        className="shrink-0 flex items-center gap-3 px-4 py-1"
        style={{
          backgroundColor: 'var(--bg-surface)',
          borderTop: '1px solid var(--border-subtle)',
        }}
      >
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          {filteredLines.length.toLocaleString()} / {lines.length.toLocaleString()} lines
        </span>
        {lines.length >= 10_000 && (
          <span className="text-xs" style={{ color: 'var(--status-warn)' }}>
            Buffer at capacity (10,000 lines) — oldest lines dropped
          </span>
        )}
      </div>
    </div>
  )
}
