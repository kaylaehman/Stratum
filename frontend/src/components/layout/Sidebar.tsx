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
  Zap,
} from 'lucide-react'
import { NavLink, useNavigate, useLocation } from 'react-router-dom'
import { useState, useEffect, useRef } from 'react'
import { useBookmarks, useRemoveBookmark } from '../../lib/api/bookmarks'
import { useCan } from '../../lib/roles'
import { resourceLink } from '../../lib/resourceLink'
import type { BookmarkResourceType, Bookmark } from '../../types/api'

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


// ---------------------------------------------------------------------------
// Nav structure
// ---------------------------------------------------------------------------

/** Pinned items rendered above the groups (always visible). */
const pinnedTop: NavLeaf[] = [
  { icon: <LayoutDashboard size={14} />, label: 'Dashboard', to: '/' },
  { icon: <GitBranch size={14} />, label: 'Resources', to: '/resources' },
]

/** Pinned items rendered below the groups (always visible). */
const pinnedBottom: NavLeaf[] = [
  { icon: <Bell size={14} />, label: 'Notifications', to: '/notifications', adminOnly: true },
  { icon: <Settings size={14} />, label: 'Settings', to: '/settings' },
]

/**
 * Collapsible groups. Topology's four children are flattened directly under
 * Infrastructure — nesting a NavGroup inside another NavGroup is not supported
 * by the existing pattern and would add unnecessary complexity.
 */
const navGroups: NavGroup[] = [
  {
    icon: <TrendingUp size={14} />,
    label: 'Monitor',
    children: [
      { icon: <TrendingUp size={14} />, label: 'Metrics', to: '/metrics' },
      { icon: <ScrollText size={14} />, label: 'Logs', to: '/logs' },
      { icon: <Radio size={14} />, label: 'Uptime', to: '/uptime' },
      { icon: <AlertCircle size={14} />, label: 'Incidents', to: '/incidents' },
      { icon: <Activity size={14} />, label: 'Activity', to: '/activity' },
    ],
  },
  {
    icon: <HardDrive size={14} />,
    label: 'Infrastructure',
    children: [
      { icon: <HardDrive size={14} />, label: 'Nodes', to: '/nodes' },
      { icon: <Database size={14} />, label: 'Volumes', to: '/volumes' },
      // Topology children flattened here
      { icon: <NetworkIcon size={14} />, label: 'Infrastructure', to: '/infrastructure' },
      { icon: <Share2 size={14} />, label: 'Network', to: '/network' },
      { icon: <Workflow size={14} />, label: 'Dependencies', to: '/dependencies' },
      { icon: <Layers size={14} />, label: 'Stacks', to: '/stacks' },
    ],
  },
  {
    icon: <ArrowUpCircle size={14} />,
    label: 'Operations',
    children: [
      { icon: <ArrowUpCircle size={14} />, label: 'Updates', to: '/updates' },
      { icon: <ListChecks size={14} />, label: 'Bulk Ops', to: '/bulk' },
      { icon: <LayoutTemplate size={14} />, label: 'Templates', to: '/templates' },
      { icon: <Archive size={14} />, label: 'Backups', to: '/backups' },
      { icon: <Terminal size={14} />, label: 'Scripts', to: '/scripts', adminOnly: true },
      { icon: <SquareTerminal size={14} />, label: 'Terminal', to: '/terminal', adminOnly: true },
    ],
  },
  {
    icon: <Shield size={14} />,
    label: 'Security',
    children: [
      { icon: <Shield size={14} />, label: 'Security', to: '/security' },
      { icon: <ShieldAlert size={14} />, label: 'CVE Scan', to: '/cve' },
      { icon: <ShieldCheck size={14} />, label: 'Certificates', to: '/certs', adminOnly: true },
      { icon: <KeyRound size={14} />, label: 'Secrets', to: '/secrets', adminOnly: true },
    ],
  },
  {
    icon: <Bot size={14} />,
    label: 'Assist',
    children: [
      { icon: <Bot size={14} />, label: 'Assistant', to: '/chat', operatorOnly: true },
      { icon: <Zap size={14} />, label: 'Automations', to: '/automations', operatorOnly: true },
      { icon: <Wrench size={14} />, label: 'Skills', to: '/skills' },
    ],
  },
]

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function sessionKey(groupLabel: string): string {
  return `stratum.navGroup.${groupLabel}`
}

function readStoredOpen(groupLabel: string): boolean | null {
  const raw = sessionStorage.getItem(sessionKey(groupLabel))
  if (raw === null) return null
  return raw === 'true'
}

function writeStoredOpen(groupLabel: string, open: boolean): void {
  sessionStorage.setItem(sessionKey(groupLabel), String(open))
}

// Shared NavLink styling for leaf entries (top-level and nested).
function navLinkStyle({ isActive }: { isActive: boolean }): React.CSSProperties {
  return {
    backgroundColor: isActive ? 'var(--accent-glow)' : 'transparent',
    color: isActive ? 'var(--accent)' : 'var(--text-secondary)',
    borderRadius: '3px',
    textDecoration: 'none',
  }
}

// ---------------------------------------------------------------------------
// NavGroupItem — now persists open/closed state in sessionStorage
// ---------------------------------------------------------------------------

interface NavGroupItemProps {
  group: NavGroup
  /** Leaves that have been filtered out for the current user's role. */
  visibleChildren: NavLeaf[]
}

/**
 * A collapsible sidebar group. Persists open/closed state in sessionStorage
 * (key: `stratum.navGroup.<label>`). The active-route group is always expanded
 * on load, overriding any previously stored closed state.
 */
function NavGroupItem({ group, visibleChildren }: NavGroupItemProps) {
  const location = useLocation()
  const childActive = visibleChildren.some(
    (c) => location.pathname === c.to || location.pathname.startsWith(`${c.to}/`),
  )

  const [open, setOpen] = useState<boolean>(() => {
    // Active group: always open, regardless of stored state.
    if (childActive) return true
    const stored = readStoredOpen(group.label)
    // No stored state yet → default closed for non-active groups.
    return stored ?? false
  })

  // When navigating into a child route, ensure the group expands.
  useEffect(() => {
    if (childActive) setOpen(true)
  }, [childActive])

  const handleToggle = () => {
    setOpen((prev) => {
      const next = !prev
      writeStoredOpen(group.label, next)
      return next
    })
  }

  return (
    <li>
      <button
        type="button"
        onClick={handleToggle}
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
          {visibleChildren.map((child) => (
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

/**
 * Builds the navigation destination for a bookmark.
 *
 * - container bookmark: deep-links via resourceLink(undefined, resource_ref)
 *   so that useResourceDeepLink in Resources.tsx can find the owning node.
 * - node bookmark: deep-links via resourceLink(resource_ref).
 * - vm / path / file: fall back to /resources (tree will be visible).
 */
function bookmarkNavTarget(bm: Bookmark): string {
  if (bm.resource_type === 'container') {
    return resourceLink(undefined, bm.resource_ref)
  }
  if (bm.resource_type === 'node') {
    return resourceLink(bm.resource_ref)
  }
  // vm, path, file — land on Resources; user can navigate further from there.
  return '/resources'
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
              onClick={() => navigate(bookmarkNavTarget(bm))}
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
  const navRef = useRef<HTMLElement>(null)

  // Restore scroll position on mount (covers the rare full-remount case).
  useEffect(() => {
    const el = navRef.current
    if (!el) return
    const saved = sessionStorage.getItem('stratum.sidebarScroll')
    if (saved) el.scrollTop = Number(saved)
  }, [])

  // Persist scroll position whenever the user scrolls the sidebar.
  // Because AppShell keeps the Sidebar mounted across all routes, this
  // listener stays active and scrollTop is never reset by navigation.
  useEffect(() => {
    const el = navRef.current
    if (!el) return
    const onScroll = () => sessionStorage.setItem('stratum.sidebarScroll', String(el.scrollTop))
    el.addEventListener('scroll', onScroll, { passive: true })
    return () => el.removeEventListener('scroll', onScroll)
  }, [])

  /** Filter a leaf list by the current user's role. */
  function visibleLeaves(leaves: NavLeaf[]): NavLeaf[] {
    return leaves.filter((item) => {
      if (item.adminOnly && !isAdmin) return false
      if (item.operatorOnly && !isOperator) return false
      return true
    })
  }

  /** Render a single pinned leaf item. */
  function renderPinnedLeaf(item: NavLeaf) {
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
  }

  return (
    <nav
      ref={navRef}
      className="w-52 shrink-0 flex flex-col pt-2 pb-4 overflow-y-auto h-full"
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
        {/* Pinned top items — always visible, ungrouped */}
        {pinnedTop.map(renderPinnedLeaf)}

        {/* Collapsible groups — hidden entirely if no visible children */}
        {navGroups.map((group) => {
          const visible = visibleLeaves(group.children)
          if (visible.length === 0) return null
          return <NavGroupItem key={group.label} group={group} visibleChildren={visible} />
        })}

        {/* Pinned bottom items — always visible, ungrouped */}
        {pinnedBottom.map(renderPinnedLeaf)}
      </ul>
      <BookmarksSection />
    </nav>
  )
}
