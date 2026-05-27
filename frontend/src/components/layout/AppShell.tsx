import { type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { Topbar } from './Topbar'
import { Sidebar } from './Sidebar'
import { useAuthStore } from '../../store/auth'
import { apiPost } from '../../lib/api'

interface AppShellProps {
  children: ReactNode
}

export function AppShell({ children }: AppShellProps) {
  const navigate = useNavigate()
  const { user, clearAuth } = useAuthStore()

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
      <Topbar user={user} onLogout={handleLogout} />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar />
        <main className="flex-1 overflow-auto p-6" style={{ backgroundColor: 'var(--bg-base)' }}>
          {children}
        </main>
      </div>
    </div>
  )
}
