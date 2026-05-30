import { useEffect, useRef, useState, useCallback, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { Search, Server, Container, HardDrive, Bookmark, Clock, CornerDownLeft } from 'lucide-react'
import { useSearch } from '../../lib/api/search'
import type {
  SearchNodeHit,
  SearchContainerHit,
  SearchVMHit,
  SearchBookmarkHit,
} from '../../types/api'

const STORAGE_KEY = 'stratum.recentSearches'
const MAX_RECENT = 5
const DEBOUNCE_MS = 200

// ── types ──────────────────────────────────────────────────────────────────

type HitKind = 'node' | 'container' | 'vm' | 'bookmark'

interface FlatHit {
  kind: HitKind
  id: string
  primary: string
  secondary: string
}

interface CommandPaletteProps {
  open: boolean
  onClose: () => void
}

// ── helpers ────────────────────────────────────────────────────────────────

function loadRecent(): string[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? (JSON.parse(raw) as string[]) : []
  } catch {
    return []
  }
}

function saveRecent(query: string): void {
  const prev = loadRecent().filter((q) => q !== query)
  const next = [query, ...prev].slice(0, MAX_RECENT)
  localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
}

function flattenResults(
  nodes: SearchNodeHit[],
  containers: SearchContainerHit[],
  vms: SearchVMHit[],
  bookmarks: SearchBookmarkHit[],
): FlatHit[] {
  const hits: FlatHit[] = [
    ...nodes.map((n) => ({
      kind: 'node' as const,
      id: n.id,
      primary: n.name,
      secondary: n.type,
    })),
    ...containers.map((c) => ({
      kind: 'container' as const,
      id: c.id,
      primary: c.name,
      secondary: `${c.image} · ${c.status}`,
    })),
    ...vms.map((v) => ({
      kind: 'vm' as const,
      id: v.id,
      primary: v.name,
      secondary: v.node_id,
    })),
    ...bookmarks.map((b) => ({
      kind: 'bookmark' as const,
      id: b.id,
      primary: b.label,
      secondary: b.resource_ref,
    })),
  ]
  return hits
}

// ── sub-components ─────────────────────────────────────────────────────────

const iconFor = (kind: HitKind) => {
  const sz = 13
  if (kind === 'node') return <Server size={sz} style={{ color: 'var(--accent)' }} />
  if (kind === 'container') return <Container size={sz} style={{ color: 'var(--accent)' }} />
  if (kind === 'vm') return <HardDrive size={sz} style={{ color: 'var(--accent)' }} />
  return <Bookmark size={sz} style={{ color: 'var(--accent)' }} />
}

const labelFor = (kind: HitKind): string => {
  if (kind === 'node') return 'NODES'
  if (kind === 'container') return 'CONTAINERS'
  if (kind === 'vm') return 'VMS'
  return 'BOOKMARKS'
}

// ── main component ─────────────────────────────────────────────────────────

export function CommandPalette({ open, onClose }: CommandPaletteProps) {
  const navigate = useNavigate()
  const inputRef = useRef<HTMLInputElement>(null)

  const [rawQuery, setRawQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [activeIdx, setActiveIdx] = useState(0)
  const [recent, setRecent] = useState<string[]>([])

  // Debounce raw input → debouncedQuery
  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(rawQuery.trim()), DEBOUNCE_MS)
    return () => clearTimeout(t)
  }, [rawQuery])

  // Reset state when palette opens/closes
  useEffect(() => {
    if (open) {
      setRawQuery('')
      setDebouncedQuery('')
      setActiveIdx(0)
      setRecent(loadRecent())
      setTimeout(() => inputRef.current?.focus(), 0)
    }
  }, [open])

  const { data, isFetching } = useSearch(debouncedQuery)

  const hasQuery = debouncedQuery.length > 0
  const flatHits = useMemo(
    () =>
      hasQuery
        ? flattenResults(
            data?.nodes ?? [],
            data?.containers ?? [],
            data?.vms ?? [],
            data?.bookmarks ?? [],
          )
        : [],
    [hasQuery, data],
  )

  // Reset active index whenever results change
  useEffect(() => {
    setActiveIdx(0)
  }, [flatHits.length, debouncedQuery])

  const navigate_ = useCallback(
    (hit: FlatHit) => {
      navigate('/resources')
      saveRecent(debouncedQuery || hit.primary)
      setRecent(loadRecent())
      onClose()
    },
    [navigate, onClose, debouncedQuery],
  )

  const runRecent = useCallback(
    (q: string) => {
      setRawQuery(q)
      setDebouncedQuery(q)
    },
    [],
  )

  // Keyboard navigation
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
        return
      }
      if (!hasQuery || flatHits.length === 0) return

      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setActiveIdx((i) => Math.min(i + 1, flatHits.length - 1))
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setActiveIdx((i) => Math.max(i - 1, 0))
      } else if (e.key === 'Enter') {
        e.preventDefault()
        const hit = flatHits[activeIdx]
        if (hit) navigate_(hit)
      }
    },
    [hasQuery, flatHits, activeIdx, navigate_, onClose],
  )

  if (!open) return null

  // Build grouped sections for rendering, tracking each group's start index in
  // the flattened hit list (for keyboard-nav highlight math).
  const kinds: HitKind[] = ['node', 'container', 'vm', 'bookmark']
  let runningIdx = 0
  const groupMeta: { kind: HitKind; startIdx: number; hits: FlatHit[] }[] = []
  for (const kind of kinds) {
    const group = flatHits.filter((h) => h.kind === kind)
    if (group.length > 0) {
      groupMeta.push({ kind, startIdx: runningIdx, hits: group })
      runningIdx += group.length
    }
  }

  const isEmpty = hasQuery && !isFetching && flatHits.length === 0

  return (
    <>
      {/* Backdrop */}
      <div
        onClick={onClose}
        style={{
          position: 'fixed',
          inset: 0,
          backgroundColor: 'rgba(0,0,0,0.55)',
          zIndex: 999,
        }}
      />

      {/* Palette container */}
      <div
        onKeyDown={handleKeyDown}
        style={{
          position: 'fixed',
          top: '15%',
          left: '50%',
          transform: 'translateX(-50%)',
          width: '560px',
          maxWidth: 'calc(100vw - 32px)',
          maxHeight: '480px',
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          zIndex: 1000,
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {/* Input row */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            padding: '10px 14px',
            borderBottom: '1px solid var(--border-subtle)',
          }}
        >
          <Search size={15} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
          <input
            ref={inputRef}
            type="text"
            value={rawQuery}
            onChange={(e) => setRawQuery(e.target.value)}
            placeholder="Search nodes, containers, VMs, bookmarks…"
            style={{
              flex: 1,
              background: 'transparent',
              border: 'none',
              outline: 'none',
              fontSize: '13px',
              fontFamily: 'var(--font-mono, monospace)',
              color: 'var(--text-primary)',
            }}
          />
          {isFetching && (
            <span
              style={{
                fontSize: '12px',
                color: 'var(--text-muted)',
                fontFamily: 'monospace',
                letterSpacing: '0.05em',
              }}
            >
              …
            </span>
          )}
          <span
            style={{
              fontSize: '12px',
              color: 'var(--text-muted)',
              fontFamily: 'monospace',
              display: 'flex',
              alignItems: 'center',
              gap: '3px',
            }}
          >
            <CornerDownLeft size={11} /> to select
          </span>
        </div>

        {/* Results area */}
        <div style={{ overflowY: 'auto', flex: 1 }}>
          {/* Recent searches (shown when input is empty) */}
          {!hasQuery && recent.length > 0 && (
            <section>
              <GroupLabel label="RECENT" />
              {recent.map((q) => (
                <RecentRow key={q} query={q} onClick={() => runRecent(q)} />
              ))}
            </section>
          )}

          {/* No results */}
          {isEmpty && (
            <div
              style={{
                padding: '24px',
                textAlign: 'center',
                fontSize: '12px',
                fontFamily: 'monospace',
                color: 'var(--text-muted)',
              }}
            >
              No matches for "{debouncedQuery}"
            </div>
          )}

          {/* Grouped results */}
          {groupMeta.map(({ kind, startIdx, hits }) => (
            <section key={kind}>
              <GroupLabel label={labelFor(kind)} />
              {hits.map((hit, localIdx) => {
                const flatIdx = startIdx + localIdx
                return (
                  <HitRow
                    key={hit.id}
                    hit={hit}
                    active={activeIdx === flatIdx}
                    onMouseEnter={() => setActiveIdx(flatIdx)}
                    onClick={() => navigate_(hit)}
                  />
                )
              })}
            </section>
          ))}
        </div>
      </div>
    </>
  )
}

// ── small presentational components ───────────────────────────────────────

function GroupLabel({ label }: { label: string }) {
  return (
    <div
      style={{
        padding: '6px 14px 3px',
        fontSize: '12px',
        fontFamily: 'monospace',
        letterSpacing: '0.08em',
        color: 'var(--text-muted)',
        textTransform: 'uppercase',
      }}
    >
      {label}
    </div>
  )
}

interface HitRowProps {
  hit: FlatHit
  active: boolean
  onMouseEnter: () => void
  onClick: () => void
}

function HitRow({ hit, active, onMouseEnter, onClick }: HitRowProps) {
  return (
    <div
      onClick={onClick}
      onMouseEnter={onMouseEnter}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '10px',
        padding: '7px 14px',
        cursor: 'pointer',
        backgroundColor: active ? 'var(--bg-surface)' : 'transparent',
        borderLeft: active ? '2px solid var(--accent)' : '2px solid transparent',
      }}
    >
      {iconFor(hit.kind)}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{
            fontSize: '13px',
            color: 'var(--text-primary)',
            fontFamily: 'monospace',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {hit.primary}
        </div>
        <div
          style={{
            fontSize: '12px',
            color: 'var(--text-muted)',
            fontFamily: 'monospace',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {hit.secondary}
        </div>
      </div>
    </div>
  )
}

function RecentRow({ query, onClick }: { query: string; onClick: () => void }) {
  return (
    <div
      onClick={onClick}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '10px',
        padding: '7px 14px',
        cursor: 'pointer',
      }}
      onMouseEnter={(e) => {
        ;(e.currentTarget as HTMLDivElement).style.backgroundColor = 'var(--bg-surface)'
      }}
      onMouseLeave={(e) => {
        ;(e.currentTarget as HTMLDivElement).style.backgroundColor = 'transparent'
      }}
    >
      <Clock size={13} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
      <span
        style={{
          fontSize: '13px',
          color: 'var(--text-secondary)',
          fontFamily: 'monospace',
        }}
      >
        {query}
      </span>
    </div>
  )
}
