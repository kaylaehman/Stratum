import {
  LayoutDashboard,
  HardDrive,
  GitBranch,
  Container,
  FileText,
  Shield,
  ShieldAlert,
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
  Bell,
  ArrowUpCircle,
  LayoutTemplate,
  KeyRound,
  Terminal,
  Archive,
  ShieldCheck,
  Wrench,
  Bot,
  ChevronRight,
  SquareTerminal,
  Network as NetworkIcon,
  Layers,
  AlertCircle,
  Radio,
} from 'lucide-react'
import { NavLink, useNavigate, useLocation } from 'react-router-dom'
import { useState, useEffect } from 'react'
import { useBookmarks, useRemoveBookmark } from '../../lib/api/bookmarks'
import { useCan } from '../../lib/roles'
import type { BookmarkResourceType } from '../../types/api'

interface NavLeaf {
  icon: React.ReactNode
  label: string
  to: string
  adminOnly?: boolean
  operatorOnly?: boolean
}

interface NavGroup {
  icon: React.ReactNode
  label: string
  children: NavLeaf[]
}

type NavEntry = NavLeaf | NavGroup

function isGroup(entry: NavEntry): entry is NavGroup {
  return 'children' in entry
}

const navItems: NavEntry[] = [
  { icon: <LayoutDashboard size={14} />, label: 'Dashboard', to: '/' },
  { icon: <GitBranch size={14} />, label: 'Resources', to: '/resources' },
  { icon: <HardDrive size={14} />, label: 'Nodes', to: '/nodes' },
  { icon: <Database size={14} />, label: 'Volumes', to: '/volumes' },
  { icon: <TrendingUp size={14} />, label: 'Metrics', to: '/metrics' },
  {
    icon: <Share2 size={14} />,
    label: 'Topology',
    children: [
      { icon: <NetworkIcon size={14} />, label: 'Infrastructure', to: '/infrastructure' },
      { icon: <Share2 size={14} />, label: 'Network', to: '/network' },
      { icon: <Workflow size={14} />, label: 'Dependencies', to: '/dependencies' },
      { icon: <Layers size={14} />, label: 'Stacks', to: '/stacks' },
    ],
  },
  { icon: <ScrollText size={14} />, label: 'Logs', to: '/logs' },
  { icon: <Bot size={14} />, label: 'Assistant', to: '/chat', operatorOnly: true },
  { icon: <Shield size={14} />, label: 'Security', to: '/security' },
  { icon: <ShieldAlert size={14} />, label: 'CVE Scan', to: '/cve' },
  { icon: <ListChecks size={14} />, label: 'Bulk Ops', to: '/bulk' },
  { icon: <ArrowUpCircle size={14} />, label: 'Updates', to: '/updates' },
  { icon: <LayoutTemplate size={14} />, label: 'Templates', to: '/templates' },
  { icon: <Wrench size={14} />, label: 'Skills', to: '/skills' },
  { icon: <ShieldCheck size={14} />, label: 'Certificates', to: '/certs', adminOnly: true },
  { icon: <KeyRound size={14} />, label: 'Secrets', to: '/secrets', adminOnly: true },
  { icon: <SquareTerminal size={14} />, label: 'Terminal', to: '/terminal', adminOnly: true },
  { icon: <Terminal size={14} />, label: 'Scripts', to: '/scripts', adminOnly: true },
  { icon: <Archive size={14} />, label: 'Backups', to: '/backups' },
  { icon: <Activity size={14} />, label: 'Activity', to: '/activity' },
  { icon: <AlertCircle size={14} />, label: 'Incidents', to: '/incidents' },
  { icon: <Radio size={14} />, label: 'Uptime', to: '/uptime' },
  { icon: <Bell size={14} />, label: 'Notifications', to: '/notifications', adminOnly: true },
  { icon: <Settings size={14} />, label: 'Settings', to: '/settings' },
]

// Shared NavLink styling for leaf entries (top-level and nested).
function navLinkStyle({ isActive }: { isActive: boolean }): React.CSSProperties {
  return {
    backgroundColor: isActive ? 'var(--accent-glow)' : 'transparent',
    color: isActive ? 'var(--accent)' : 'var(--text-secondary)',
    borderRadius: '3px',
    textDecoration: 'none',
  }
}

/** A collapsible sidebar group (e.g. Topology → Network, Dependencies). Auto-
 *  expands when one of its child routes is active; otherwise starts collapsed. */
function NavGroupItem({ group }: { group: NavGroup }) {
  const location = useLocation()
  const childActive = group.children.some(
    (c) => location.pathname === c.to || location.pathname.startsWith(`${c.to}/`),
  )
  const [open, setOpen] = useState(childActive)

  // Expand automatically when navigating into one of the group's routes.
  useEffect(() => {
    if (childActive) setOpen(true)
  }, [childActive])

  return (
    <li>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-2.5 px-2.5 py-1.5 rounded text-sm transition-colors w-full"
        style={{
          background: 'transparent',
          border: 'none',
          cursor: 'pointer',
          color: childActive ? 'var(--accent)' : 'var(--text-secondary)',
          borderRadius: '3px',
          textAlign: 'left',
        }}
      >
        {group.icon}
        <span className="flex-1">{group.label}</span>
        <ChevronRight
          size={13}
          style={{
            color: 'var(--text-muted)',
            transform: open ? 'rotate(90deg)' : 'none',
            transition: 'transform 0.12s',
          }}
        />
      </button>
      {open && (
        <ul className="flex flex-col gap-0.5 mt-0.5" style={{ marginLeft: '10px', paddingLeft: '8px', borderLeft: '1px solid var(--border-subtle)' }}>
          {group.children.map((child) => (
            <li key={child.to}>
              <NavLink
                to={child.to}
                className="flex items-center gap-2.5 px-2.5 py-1.5 rounded text-sm transition-colors"
                style={navLinkStyle}
              >
                {child.icon}
                {child.label}
              </NavLink>
            </li>
          ))}
        </ul>
      )}
    </li>
  )
}

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
          className="text-sm font-medium uppercase tracking-wider"
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
              className="flex items-center gap-2 px-2.5 py-1.5 flex-1 text-sm truncate text-left"
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
  const { isAdmin, isOperator } = useCan()

  return (
    <nav
      className="w-52 shrink-0 flex flex-col pt-2 pb-4 overflow-y-auto"
      style={{
        backgroundColor: 'var(--bg-surface)',
        borderRight: '1px solid var(--border-subtle)',
      }}
    >
      <div className="px-3 mb-2">
        <span className="text-sm font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
          Navigation
        </span>
      </div>
      <ul className="flex flex-col gap-0.5 px-2">
        {navItems.map((item) => {
          if (isGroup(item)) {
            return <NavGroupItem key={item.label} group={item} />
          }
          if (item.adminOnly && !isAdmin) return null
          if (item.operatorOnly && !isOperator) return null
          return (
            <li key={item.to}>
              <NavLink
                to={item.to}
                end={item.to === '/'}
                className="flex items-center gap-2.5 px-2.5 py-1.5 rounded text-sm transition-colors"
                style={navLinkStyle}
              >
                {item.icon}
                {item.label}
              </NavLink>
            </li>
          )
        })}
      </ul>
      <BookmarksSection />
    </nav>
  )
}
