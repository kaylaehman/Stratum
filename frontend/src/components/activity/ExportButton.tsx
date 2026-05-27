import { useState } from 'react'
import { Download, Loader } from 'lucide-react'
import { exportActivityCsv } from '../../lib/api/activity'
import type { ActivityFilters } from '../../types/api'

interface ExportButtonProps {
  filters: ActivityFilters
}

export function ExportButton({ filters }: ExportButtonProps) {
  const [loading, setLoading] = useState(false)

  async function handleExport() {
    if (loading) return
    setLoading(true)
    try {
      await exportActivityCsv(filters)
    } catch (err) {
      console.error('CSV export failed', err)
    } finally {
      setLoading(false)
    }
  }

  return (
    <button
      type="button"
      onClick={() => void handleExport()}
      disabled={loading}
      className="flex items-center gap-1.5 text-xs px-3 py-1.5"
      style={{
        backgroundColor: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        color: 'var(--text-secondary)',
        borderRadius: '3px',
        cursor: loading ? 'default' : 'pointer',
        opacity: loading ? 0.6 : 1,
      }}
    >
      {loading ? (
        <Loader size={11} className="animate-spin" />
      ) : (
        <Download size={11} />
      )}
      {loading ? 'Exporting...' : 'Export CSV'}
    </button>
  )
}
