import { useState } from 'react'
import { Activity as ActivityIcon, ShieldAlert } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { ActivityLog } from '../components/activity/ActivityLog'
import { ExportButton } from '../components/activity/ExportButton'
import type { ActivityFilters } from '../types/api'

function AdminRequired() {
  return (
    <div
      className="flex flex-col items-center justify-center gap-3 py-16"
      style={{ color: 'var(--text-muted)' }}
    >
      <ShieldAlert size={28} style={{ color: 'var(--text-muted)' }} />
      <span className="text-xs uppercase tracking-wider">Admin access required</span>
      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
        The activity log is only accessible to administrators.
      </span>
    </div>
  )
}

export default function Activity() {
  const { data: me, isLoading: meLoading } = useMe()
  const isAdmin = me?.role === 'admin'

  const [filters, setFilters] = useState<ActivityFilters>({})

  return (
    <AppShell>
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full p-6"
        style={{ maxWidth: '1100px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-2">
            <ActivityIcon size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1
              className="text-sm font-medium uppercase tracking-wider"
              style={{ color: 'var(--text-primary)' }}
            >
              Activity Log
            </h1>
          </div>
          {isAdmin && <ExportButton filters={filters} />}
        </div>

        {/* Admin gate */}
        {!meLoading && !isAdmin && <AdminRequired />}

        {/* Content */}
        {isAdmin && (
          <ActivityLog filters={filters} onFiltersChange={setFilters} />
        )}
      </div>
    </AppShell>
  )
}
