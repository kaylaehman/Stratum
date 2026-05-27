import { type ReactNode, useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
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

export function AppShell({ children, treeSlot }: AppShellProps) {
  const navigate = useNavigate()
  const { user, clearAuth } = useAuthStore()
  const { isOperator } = useCan()
  const [searchOpen, setSearchOpen] = useState(false)

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
      <Topbar user={user} onLogout={handleLogout} onSearchOpen={() => setSearchOpen(true)} />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar />
        {treeSlot && (
          <div
            className="flex flex-col shrink-0 overflow-hidden"
            style={{
              width: '220px',
              backgroundColor: 'var(--bg-surface)',
              borderRight: '1px solid var(--border-subtle)',
            }}
          >
            {treeSlot}
          </div>
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
