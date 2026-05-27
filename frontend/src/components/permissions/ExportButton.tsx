import { Download } from 'lucide-react'
import { downloadAnalysis } from '../../lib/api/permissions'

interface ExportButtonProps {
  containerId: string
}

export function ExportButton({ containerId }: ExportButtonProps) {
  return (
    <div className="flex items-center gap-2">
      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
        Export:
      </span>
      <button
        type="button"
        onClick={() => downloadAnalysis(containerId, 'json')}
        className="flex items-center gap-1 text-xs px-2 py-1"
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          color: 'var(--text-secondary)',
          borderRadius: '3px',
          cursor: 'pointer',
        }}
      >
        <Download size={11} />
        JSON
      </button>
      <button
        type="button"
        onClick={() => downloadAnalysis(containerId, 'csv')}
        className="flex items-center gap-1 text-xs px-2 py-1"
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          color: 'var(--text-secondary)',
          borderRadius: '3px',
          cursor: 'pointer',
        }}
      >
        <Download size={11} />
        CSV
      </button>
    </div>
  )
}
