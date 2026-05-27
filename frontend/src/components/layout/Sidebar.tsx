import {
  LayoutDashboard,
  HardDrive,
  GitBranch,
  Container,
  FileText,
  Shield,
  Activity,
  Settings,
  ScrollText,
  Database,
  TrendingUp,
  Share2,
  Workflow,
  ListChecks,
  Server,
  Folder,
  X,
} from 'lucide-react'
import { NavLink, useNavigate } from 'react-router-dom'
import { useState } from 'react'
import { useBookmarks, useRemoveBookmark } from '../../lib/api/bookmarks'
import type { BookmarkResourceType } from '../../types/api'

interface NavItem {
  icon: React.ReactNode
  label: string
  to: string
}

const navItems: NavItem[] = [
  { icon: <LayoutDashboard size={14} />, label: 'Dashboard', to: '/' },
  { icon: <GitBranch size={14} />, label: 'Resources', to: '/resources' },
  { icon: <HardDrive size={14} />, label: 'Nodes', to: '/nodes' },
  { icon: <Container size={14} />, label: 'Containers', to: '/containers' },
  { icon: <Database size={14} />, label: 'Volumes', to: '/volumes' },
  { icon: <TrendingUp size={14} />, label: 'Metrics', to: '/metrics' },
  { icon: <Share2 size={14} />, label: 'Network', to: '/network' },
  { icon: <Workflow size={14} />, label: 'Dependencies', to: '/dependencies' },
  { icon: <FileText size={14} />, label: 'Filesystem', to: '/filesystem' },
  { icon: <ScrollText size={14} />, label: 'Logs', to: '/logs' },
  { icon: <Shield size={14} />, label: 'Security', to: '/security' },
  { icon: <ListChecks size={14} />, label: 'Bulk Ops', to: '/bulk' },
  { icon: <Activity size={14} />, label: 'Activity', to: '/activity' },
  { icon: <Settings size={14} />, label: 'Settings', to: '/settings' },
]

function bookmarkIcon(type: BookmarkResourceType) {
  switch (type) {
    case 'container':
      return <Container size={12} />
    case 'node':
      return <HardDrive size={12} />
    case 'vm':
      return <Server size={12} />
    case 'file':
      return <FileText size={12} />
    case 'path':
    default:
      return <Folder size={12} />
  }
}

function BookmarksSection() {
  const { data } = useBookmarks()
  const { mutate: remove } = useRemoveBookmark()
  const navigate = useNavigate()
  const [hoveredId, setHoveredId] = useState<string | null>(null)

  const bookmarks = data?.bookmarks ?? []
  if (bookmarks.length === 0) return null

  return (
    <div className="mt-3">
      <div className="px-3 mb-2">
        <span
          className="text-xs font-medium uppercase tracking-wider"
          style={{ color: 'var(--text-muted)' }}
        >
          Bookmarks
        </span>
      </div>
      <ul className="flex flex-col gap-0.5 px-2">
        {bookmarks.map((bm) => (
          <li
            key={bm.id}
            onMouseEnter={() => setHoveredId(bm.id)}
            onMouseLeave={() => setHoveredId(null)}
            className="flex items-center gap-1 rounded"
            style={{ borderRadius: '3px' }}
          >
            <button
              type="button"
              onClick={() => navigate('/resources')}
              className="flex items-center gap-2 px-2.5 py-1.5 flex-1 text-xs truncate text-left"
              style={{
                background: 'transparent',
                border: 'none',
                color: 'var(--text-secondary)',
                cursor: 'pointer',
                borderRadius: '3px',
                minWidth: 0,
              }}
            >
              <span style={{ color: 'var(--text-muted)', flexShrink: 0 }}>
                {bookmarkIcon(bm.resource_type)}
              </span>
              <span className="truncate">{bm.label}</span>
            </button>
            {hoveredId === bm.id && (
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation()
                  remove(bm.id)
                }}
                title="Remove bookmark"
                className="flex items-center justify-center shrink-0 mr-1"
                style={{
                  background: 'transparent',
                  border: 'none',
                  color: 'var(--text-muted)',
                  cursor: 'pointer',
                  padding: '2px',
                  borderRadius: '3px',
                }}
              >
                <X size={11} />
              </button>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}

export function Sidebar() {
  return (
    <nav
      className="w-52 shrink-0 flex flex-col pt-2 pb-4"
      style={{
        backgroundColor: 'var(--bg-surface)',
        borderRight: '1px solid var(--border-subtle)',
      }}
    >
      <div className="px-3 mb-2">
        <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
          Navigation
        </span>
      </div>
      <ul className="flex flex-col gap-0.5 px-2">
        {navItems.map((item) => (
          <li key={item.to}>
            <NavLink
              to={item.to}
              end={item.to === '/'}
              className="flex items-center gap-2.5 px-2.5 py-1.5 rounded text-xs transition-colors"
              style={({ isActive }) => ({
                backgroundColor: isActive ? 'var(--accent-glow)' : 'transparent',
                color: isActive ? 'var(--accent)' : 'var(--text-secondary)',
                borderRadius: '3px',
                textDecoration: 'none',
              })}
            >
              {item.icon}
              {item.label}
            </NavLink>
          </li>
        ))}
      </ul>
      <BookmarksSection />
    </nav>
  )
}
