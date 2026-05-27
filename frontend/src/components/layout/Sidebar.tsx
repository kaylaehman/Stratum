import { LayoutDashboard, HardDrive, GitBranch, Container, FileText, Shield, Activity, Settings } from 'lucide-react'
import { NavLink } from 'react-router-dom'

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
  { icon: <FileText size={14} />, label: 'Filesystem', to: '/filesystem' },
  { icon: <Shield size={14} />, label: 'Security', to: '/security' },
  { icon: <Activity size={14} />, label: 'Activity', to: '/activity' },
  { icon: <Settings size={14} />, label: 'Settings', to: '/settings' },
]

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
    </nav>
  )
}
