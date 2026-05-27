import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { Activity, HardDrive, Container, Shield } from 'lucide-react'

interface StatCardProps {
  icon: React.ReactNode
  label: string
  value: string
  status?: 'ok' | 'warn' | 'muted'
}

function StatCard({ icon, label, value, status = 'muted' }: StatCardProps) {
  const statusColor = {
    ok: 'var(--status-ok)',
    warn: 'var(--status-warn)',
    muted: 'var(--text-muted)',
  }[status]

  return (
    <div
      className="flex items-center gap-4 p-4 rounded border"
      style={{
        backgroundColor: 'var(--bg-surface)',
        borderColor: 'var(--border-default)',
        borderRadius: '3px',
      }}
    >
      <div style={{ color: statusColor }}>{icon}</div>
      <div>
        <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
          {label}
        </p>
        <p className="text-sm font-semibold font-mono" style={{ color: 'var(--text-primary)' }}>
          {value}
        </p>
      </div>
    </div>
  )
}

export default function Dashboard() {
  const { data: me, isLoading, isError } = useMe()

  return (
    <AppShell>
      <div className="flex flex-col gap-6">
        {/* Header */}
        <div>
          <h1 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
            Dashboard
          </h1>
          {isLoading && (
            <p className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
              Loading...
            </p>
          )}
          {isError && (
            <p className="text-xs mt-1" style={{ color: 'var(--status-error)' }}>
              Failed to load user info.
            </p>
          )}
          {me && (
            <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
              Signed in as{' '}
              <span className="font-mono" style={{ color: 'var(--accent)' }}>
                {me.username}
              </span>{' '}
              &middot; role:{' '}
              <span className="font-mono" style={{ color: 'var(--text-primary)' }}>
                {me.role}
              </span>
            </p>
          )}
        </div>

        {/* Stat cards */}
        <div className="grid grid-cols-2 gap-3" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))' }}>
          <StatCard icon={<HardDrive size={18} />} label="Connected nodes" value="—" status="muted" />
          <StatCard icon={<Container size={18} />} label="Running containers" value="—" status="muted" />
          <StatCard icon={<Shield size={18} />} label="Critical CVEs" value="—" status="muted" />
          <StatCard icon={<Activity size={18} />} label="Alerts" value="—" status="muted" />
        </div>

        {/* Placeholder content */}
        <div
          className="rounded border p-6 text-center"
          style={{
            backgroundColor: 'var(--bg-surface)',
            borderColor: 'var(--border-subtle)',
            borderRadius: '3px',
          }}
        >
          <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
            No nodes connected. Add a node to get started.
          </p>
        </div>
      </div>
    </AppShell>
  )
}
