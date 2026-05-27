import { Loader, AlertTriangle } from 'lucide-react'
import { useFileVerdict } from '../../lib/api/permissions'

interface PermChipProps {
  label: string
  granted: boolean
}

function PermChip({ label, granted }: PermChipProps) {
  return (
    <span
      className="font-mono text-xs px-2 py-0.5"
      style={{
        background: granted ? 'rgba(34,201,122,0.12)' : 'rgba(232,64,64,0.12)',
        border: `1px solid ${granted ? 'var(--status-ok)' : 'var(--status-error)'}`,
        color: granted ? 'var(--status-ok)' : 'var(--status-error)',
        borderRadius: '3px',
      }}
    >
      {label}
    </span>
  )
}

interface VerdictRowProps {
  label: string
  value: string
  mono?: boolean
  valueColor?: string
}

function VerdictRow({ label, value, mono, valueColor }: VerdictRowProps) {
  return (
    <div className="flex items-baseline gap-3">
      <span
        className="text-xs shrink-0"
        style={{ color: 'var(--text-muted)', width: '160px' }}
      >
        {label}
      </span>
      <span
        className={`text-xs truncate ${mono ? 'font-mono' : ''}`}
        style={{ color: valueColor ?? 'var(--text-primary)' }}
      >
        {value}
      </span>
    </div>
  )
}

interface FileUidPanelProps {
  containerId: string
  hostPath: string
}

export function FileUidPanel({ containerId, hostPath }: FileUidPanelProps) {
  const { data, isLoading, isError } = useFileVerdict(containerId, hostPath)

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 px-4 py-4">
        <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          Analyzing permissions...
        </span>
      </div>
    )
  }

  if (isError) {
    return (
      <div className="flex items-center gap-2 px-4 py-3">
        <AlertTriangle size={13} style={{ color: 'var(--status-warn)' }} />
        <span className="text-xs" style={{ color: 'var(--status-warn)' }}>
          Unable to analyze this path. Check that the container is running and
          the path exists.
        </span>
      </div>
    )
  }

  if (!data) return null

  const fileOwnerDisplay = data.host_owner_name
    ? `${data.file_uid} (${data.host_owner_name})`
    : String(data.file_uid)

  const containerNameDisplay = data.container_owner_name
    ? `${data.file_uid} = ${data.container_owner_name}`
    : `${data.file_uid} (unresolved in container)`

  const effUidDisplay = data.process_is_root || data.root_override
    ? `${data.eff_uid} (root — all access granted)`
    : String(data.eff_uid)

  return (
    <div
      className="flex flex-col gap-3 px-4 py-3"
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
      }}
    >
      <p
        className="text-xs font-medium uppercase tracking-wider"
        style={{ color: 'var(--text-muted)' }}
      >
        File permission verdict
      </p>

      <p className="font-mono text-xs truncate" style={{ color: 'var(--accent)' }}>
        {hostPath}
      </p>

      <div className="flex flex-col gap-2">
        <VerdictRow
          label="File owner (host)"
          value={fileOwnerDisplay}
          mono
        />
        <VerdictRow
          label="Container identity"
          value={containerNameDisplay}
          mono
          valueColor={data.container_owner_name ? 'var(--text-primary)' : 'var(--text-muted)'}
        />
        <VerdictRow
          label="File mode"
          value={data.file_mode_octal}
          mono
        />
        <VerdictRow
          label="Container runs as"
          value={effUidDisplay}
          mono
          valueColor={data.process_is_root || data.root_override ? 'var(--status-warn)' : 'var(--text-primary)'}
        />
        <VerdictRow
          label="Permission category"
          value={data.category}
          mono
        />
      </div>

      {/* Read / write / exec chips */}
      <div className="flex items-center gap-2 pt-1">
        <PermChip label="read" granted={data.read_granted} />
        <PermChip label="write" granted={data.write_granted} />
        <PermChip label="exec" granted={data.exec_granted} />
      </div>

      {/* Reason */}
      <p
        className="text-xs"
        style={{
          color: 'var(--text-secondary)',
          borderLeft: '2px solid var(--border-strong)',
          paddingLeft: '8px',
        }}
      >
        {data.reason}
      </p>
    </div>
  )
}
