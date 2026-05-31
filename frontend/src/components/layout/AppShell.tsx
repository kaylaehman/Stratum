import { type ReactNode, useState, useEffect, useRef, useCallback } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { Topbar } from './Topbar'
import { Sidebar } from './Sidebar'
import { CommandPalette } from '../search/CommandPalette'
import { AIAssistantPanel } from '../ai/AIAssistantPanel'
import { StepUpModal } from '../auth/StepUpModal'
import { useAuthStore } from '../../store/auth'
import { useCan } from '../../lib/roles'
import { apiPost } from '../../lib/api'

interface AppShellProps {
  children: ReactNode
  /** Optional panel rendered between the sidebar and main content (e.g. resource tree). */
  treeSlot?: ReactNode
}

const TREE_MIN = 180
const TREE_MAX = 560
const TREE_DEFAULT = 240

export function AppShell({ children, treeSlot }: AppShellProps) {
  const navigate = useNavigate()
  const location = useLocation()
  const { user, clearAuth } = useAuthStore()
  const { isOperator } = useCan()
  const [searchOpen, setSearchOpen] = useState(false)
  const [sidebarOpen, setSidebarOpen] = useState(false)

  // Close drawer on route change.
  useEffect(() => {
    setSidebarOpen(false)
  }, [location.pathname])

  // Resizable resource-tree panel width, persisted across sessions.
  const [treeWidth, setTreeWidth] = useState<number>(() => {
    const saved = Number(localStorage.getItem('stratum.treeWidth'))
    return Number.isFinite(saved) && saved >= TREE_MIN && saved <= TREE_MAX ? saved : TREE_DEFAULT
  })
  const treePanelRef = useRef<HTMLDivElement>(null)

  const startTreeResize = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    let latest = treeWidth
    const onMove = (ev: MouseEvent) => {
      const left = treePanelRef.current?.getBoundingClientRect().left ?? 0
      latest = Math.min(TREE_MAX, Math.max(TREE_MIN, ev.clientX - left))
      setTreeWidth(latest)
    }
    const onUp = () => {
      document.removeEventListener('mousemove', onMove)
      document.removeEventListener('mouseup', onUp)
      document.body.style.userSelect = ''
      localStorage.setItem('stratum.treeWidth', String(Math.round(latest)))
    }
    document.body.style.userSelect = 'none'
    document.addEventListener('mousemove', onMove)
    document.addEventListener('mouseup', onUp)
  }, [treeWidth])

  // Global Ctrl+K / Cmd+K shortcut
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault()
        setSearchOpen((prev) => !prev)
      }
      if (e.key === 'Escape' && searchOpen) {
        setSearchOpen(false)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [searchOpen])

  async function handleLogout() {
    try {
      await apiPost('/api/auth/logout', null)
    } catch {
      // ignore logout errors — clear auth regardless
    }
    clearAuth()
    navigate('/login')
  }

  return (
    <div className="flex flex-col h-screen" style={{ backgroundColor: 'var(--bg-base)' }}>
      <Topbar
        user={user}
        onLogout={handleLogout}
        onSearchOpen={() => setSearchOpen(true)}
        onMenuToggle={() => setSidebarOpen((prev) => !prev)}
      />

      {/* Mobile off-canvas backdrop — only rendered below md when drawer is open */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 md:hidden"
          style={{ backgroundColor: 'rgba(0,0,0,0.5)', zIndex: 30 }}
          aria-hidden="true"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Mobile off-canvas drawer — slides in from the left below md */}
      <div
        className="fixed inset-y-0 left-0 md:hidden flex flex-col"
        style={{
          zIndex: 40,
          transform: sidebarOpen ? 'translateX(0)' : 'translateX(-100%)',
          transition: 'transform 0.2s ease-in-out',
          width: '13rem', // matches w-52
        }}
      >
        <Sidebar />
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* Persistent sidebar — visible only at md and above */}
        <div className="hidden md:flex">
          <Sidebar />
        </div>

        {treeSlot && (
          <>
            <div
              ref={treePanelRef}
              className="flex flex-col shrink-0 overflow-hidden"
              style={{
                // On mobile, cap the tree panel so it doesn't consume the full viewport
                width: `${treeWidth}px`,
                maxWidth: 'calc(100vw - 2rem)',
                backgroundColor: 'var(--bg-surface)',
                borderRight: '1px solid var(--border-subtle)',
              }}
            >
              {treeSlot}
            </div>
            {/* Drag handle to resize the tree panel — only meaningful on desktop */}
            <div
              role="separator"
              aria-orientation="vertical"
              title="Drag to resize"
              onMouseDown={startTreeResize}
              className="tree-resize-handle hidden md:block"
              style={{ width: '5px', cursor: 'col-resize', flexShrink: 0 }}
            />
          </>
        )}
        <main
          className={`flex-1 overflow-auto flex ${treeSlot ? '' : 'p-6'}`}
          style={{ backgroundColor: 'var(--bg-base)' }}
        >
          {treeSlot ? children : <div className="w-full">{children}</div>}
        </main>
      </div>
      <CommandPalette open={searchOpen} onClose={() => setSearchOpen(false)} />
      {isOperator && <AIAssistantPanel />}
      <StepUpModal />
    </div>
  )
}
