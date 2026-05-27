import { ChevronRight } from 'lucide-react'

interface BreadcrumbsProps {
  path: string
  onNavigate: (path: string) => void
}

function buildSegments(path: string): Array<{ label: string; path: string }> {
  const normalized = path.startsWith('/') ? path : `/${path}`
  const parts = normalized.split('/').filter(Boolean)
  const segments: Array<{ label: string; path: string }> = [
    { label: '/', path: '/' },
  ]
  let accumulated = ''
  for (const part of parts) {
    accumulated += `/${part}`
    segments.push({ label: part, path: accumulated })
  }
  return segments
}

export function Breadcrumbs({ path, onNavigate }: BreadcrumbsProps) {
  const segments = buildSegments(path)

  return (
    <nav
      aria-label="Path breadcrumbs"
      className="flex items-center gap-0.5 flex-wrap min-w-0"
    >
      {segments.map((seg, i) => {
        const isLast = i === segments.length - 1
        return (
          <span key={seg.path} className="flex items-center gap-0.5 min-w-0">
            {i > 0 && (
              <ChevronRight
                size={11}
                style={{ color: 'var(--text-muted)', flexShrink: 0 }}
              />
            )}
            <button
              type="button"
              onClick={() => onNavigate(seg.path)}
              className="font-mono text-xs truncate max-w-[160px]"
              style={{
                color: isLast ? 'var(--text-primary)' : 'var(--accent)',
                background: 'none',
                border: 'none',
                padding: '0 2px',
                cursor: isLast ? 'default' : 'pointer',
                textDecoration: 'none',
              }}
              disabled={isLast}
              title={seg.path}
            >
              {seg.label}
            </button>
          </span>
        )
      })}
    </nav>
  )
}
