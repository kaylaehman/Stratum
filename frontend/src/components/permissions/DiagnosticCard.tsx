import { useState } from 'react'
import { Loader, AlertTriangle, ShieldCheck, ShieldX, BookText, Check } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useDiagnostic } from '../../lib/api/diagnostic'
import { DiagnosticStep } from './DiagnosticStep'
import { SuggestedFixList } from './SuggestedFixList'
import { useCreateRunbook } from '../../lib/api/runbooks'
import { useEffect } from 'react'
import type { DiagnosticResult } from '../../types/api'

interface Props {
  containerId: string
  hostPath: string
}

function SaveAsRunbookButton({ data }: { data: DiagnosticResult }) {
  const create = useCreateRunbook()
  const navigate = useNavigate()
  const [saved, setSaved] = useState(false)

  function handleSave() {
    const steps = data.fixes.map((f) => f.command).filter(Boolean)
    const triggers = data.verdict.access_granted ? [] : [data.verdict.summary]
    create.mutate(
      {
        name: `Fix: ${data.host_path}`,
        description: data.verdict.summary,
        trigger_conditions: triggers,
        steps,
        requires_approval: true,
      },
      {
        onSuccess: () => {
          setSaved(true)
          setTimeout(() => navigate('/runbooks'), 1200)
        },
      },
    )
  }

  if (data.fixes.length === 0) return null

  return (
    <button
      type="button"
      onClick={handleSave}
      disabled={create.isPending || saved}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '5px',
        background: saved ? 'rgba(34,201,122,0.10)' : 'var(--bg-overlay)',
        border: `1px solid ${saved ? 'var(--status-ok)' : 'var(--border-strong)'}`,
        color: saved ? 'var(--status-ok)' : 'var(--text-secondary)',
        borderRadius: '3px',
        padding: '4px 10px',
        fontSize: '0.7rem',
        fontFamily: 'monospace',
        cursor: create.isPending || saved ? 'default' : 'pointer',
        transition: 'color 0.15s, border-color 0.15s',
      }}
    >
      {create.isPending
        ? <Loader size={11} className="animate-spin" />
        : saved
          ? <Check size={11} />
          : <BookText size={11} />
      }
      {saved ? 'Saved — opening runbooks…' : 'Save as runbook'}
    </button>
  )
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

      {/* Save as runbook */}
      {!data.verdict.access_granted && data.fixes.length > 0 && (
        <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
          <SaveAsRunbookButton data={data} />
        </div>
      )}
    </div>
  )
}
