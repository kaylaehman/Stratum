import { Loader, AlertTriangle, ShieldCheck, ShieldX } from 'lucide-react'
import { useDiagnostic } from '../../lib/api/diagnostic'
import { DiagnosticStep } from './DiagnosticStep'
import { SuggestedFixList } from './SuggestedFixList'
import { useEffect } from 'react'

interface Props {
  containerId: string
  hostPath: string
}

function VerdictBanner({ granted, summary }: { granted: boolean; summary: string }) {
  const color = granted ? 'var(--status-ok)' : 'var(--status-error)'
  const bg = granted ? 'rgba(34,201,122,0.10)' : 'rgba(232,64,64,0.10)'
  const border = granted ? 'rgba(34,201,122,0.30)' : 'rgba(232,64,64,0.30)'

  return (
    <div
      className="flex items-center gap-3 px-4 py-3"
      style={{
        backgroundColor: bg,
        border: `1px solid ${border}`,
        borderRadius: '3px',
      }}
    >
      <span style={{ color, flexShrink: 0 }}>
        {granted ? <ShieldCheck size={16} /> : <ShieldX size={16} />}
      </span>
      <div className="flex flex-col gap-0.5">
        <span className="text-xs font-semibold uppercase tracking-wider" style={{ color }}>
          {granted ? 'Access granted' : 'Access denied'}
        </span>
        <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
          {summary}
        </span>
      </div>
    </div>
  )
}

export function DiagnosticCard({ containerId, hostPath }: Props) {
  const { mutate, data, isPending, isError, error } = useDiagnostic(containerId)

  useEffect(() => {
    if (containerId && hostPath) {
      mutate(hostPath)
    }
  }, [containerId, hostPath, mutate])

  if (isPending) {
    return (
      <div
        className="flex items-center gap-2 px-4 py-4"
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
        }}
      >
        <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          Running diagnostic...
        </span>
      </div>
    )
  }

  if (isError) {
    const status =
      error instanceof Error && 'status' in error
        ? (error as { status: number }).status
        : 0

    const msg =
      status === 404
        ? 'Container not found.'
        : status === 502
          ? 'Host is unreachable. Check agent/SSH connectivity.'
          : 'Diagnostic failed. Ensure the container is running and the path exists.'

    return (
      <div
        className="flex items-start gap-2 px-4 py-3"
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
        }}
      >
        <AlertTriangle
          size={13}
          style={{ color: 'var(--status-warn)', marginTop: '1px', flexShrink: 0 }}
        />
        <span className="text-xs" style={{ color: 'var(--status-warn)' }}>
          {msg}
        </span>
      </div>
    )
  }

  if (!data) return null

  return (
    <div
      className="flex flex-col gap-3 p-4"
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
      }}
    >
      {/* Header */}
      <div className="flex flex-col gap-1">
        <p
          className="text-xs font-medium uppercase tracking-wider"
          style={{ color: 'var(--text-muted)' }}
        >
          Diagnostic — why is this broken?
        </p>
        <p className="font-mono text-xs truncate" style={{ color: 'var(--accent)' }}>
          {data.host_path}
        </p>
      </div>

      {/* Step-by-step */}
      {data.steps.length > 0 && (
        <div className="flex flex-col gap-1.5">
          {data.steps.map((step, i) => (
            <DiagnosticStep key={i} step={step} />
          ))}
        </div>
      )}

      {/* Final verdict banner */}
      <VerdictBanner
        granted={data.verdict.access_granted}
        summary={data.verdict.summary}
      />

      {/* Suggested fixes */}
      <SuggestedFixList
        fixes={data.fixes}
        accessGranted={data.verdict.access_granted}
      />
    </div>
  )
}
