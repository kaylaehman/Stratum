import { Folder, FileText, Link, AlertTriangle, Loader, CornerDownRight } from 'lucide-react'
import type { FsSearchHit, FsSearchResponse } from '../../types/api'

function humanSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
}

function HitIcon({ hit }: { hit: FsSearchHit }) {
  if (hit.is_symlink) return <Link size={13} style={{ color: 'var(--text-secondary)', flexShrink: 0 }} />
  if (hit.is_dir) return <Folder size={13} style={{ color: 'var(--accent)', flexShrink: 0 }} />
  if ((hit.classes ?? []).includes('world_writable'))
    return <AlertTriangle size={13} style={{ color: 'var(--status-error)', flexShrink: 0 }} />
  return <FileText size={13} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
}

// Highlight the matched substring within the entry name.
function HighlightedName({ name, query }: { name: string; query: string }) {
  const idx = name.toLowerCase().indexOf(query.toLowerCase())
  if (idx < 0 || !query) return <>{name}</>
  return (
    <>
      {name.slice(0, idx)}
      <span style={{ background: 'var(--accent-glow)', color: 'var(--accent)', borderRadius: '2px' }}>
        {name.slice(idx, idx + query.length)}
      </span>
      {name.slice(idx + query.length)}
    </>
  )
}

interface SearchResultsProps {
  query: string
  result: FsSearchResponse | undefined
  isLoading: boolean
  isError: boolean
  onOpenHit: (hit: FsSearchHit) => void
}

export function SearchResults({ query, result, isLoading, isError, onOpenHit }: SearchResultsProps) {
  if (isLoading) {
    return (
      <div className="flex items-center gap-2 px-4 py-6">
        <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          Searching subfolders…
        </span>
      </div>
    )
  }
  if (isError) {
    return (
      <div className="px-4 py-4 text-xs" style={{ color: 'var(--status-error)' }}>
        Search failed. The node may be unreachable, or the path isn't readable.
      </div>
    )
  }
  const hits = result?.hits ?? []
  if (hits.length === 0) {
    return (
      <div className="px-4 py-4 text-xs" style={{ color: 'var(--text-muted)' }}>
        No files or folders under this path match &ldquo;{query}&rdquo;.
      </div>
    )
  }

  return (
    <div className="flex flex-col flex-1 min-h-0">
      {/* Count / truncation banner */}
      <div
        className="flex items-center gap-2 px-3 py-1.5 shrink-0"
        style={{ borderBottom: '1px solid var(--border-default)', backgroundColor: 'var(--bg-elevated)' }}
      >
        <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          {hits.length} match{hits.length === 1 ? '' : 'es'}
          {result?.truncated && ' (truncated — narrow your search)'}
        </span>
      </div>

      {/* Results */}
      <div style={{ overflowY: 'auto', flex: 1 }}>
        {hits.map((hit) => (
          <div
            key={hit.path}
            className="flex items-center gap-3 px-3 py-1.5"
            style={{ borderBottom: '1px solid var(--border-subtle)', minHeight: '30px', cursor: 'pointer' }}
            onClick={() => onOpenHit(hit)}
          >
            <HitIcon hit={hit} />
            <div className="flex-1 min-w-0 flex items-center gap-2">
              <span
                className="font-mono text-xs truncate"
                style={{
                  color: hit.is_dir ? 'var(--accent)' : 'var(--text-primary)',
                  fontStyle: hit.is_symlink ? 'italic' : undefined,
                }}
              >
                <HighlightedName name={hit.name} query={query} />
              </span>
              {/* Where the match lives, relative to the search root */}
              <span
                className="font-mono text-xs flex items-center gap-0.5 truncate"
                style={{ color: 'var(--text-muted)' }}
                title={hit.path}
              >
                <CornerDownRight size={10} style={{ flexShrink: 0 }} />
                {hit.rel_dir === '.' ? './' : `./${hit.rel_dir}/`}
              </span>
            </div>
            <span
              className="font-mono text-xs shrink-0 text-right hidden sm:block"
              style={{ color: 'var(--text-secondary)', width: '70px' }}
            >
              {hit.is_dir ? '--' : humanSize(hit.size)}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}
