import { ShieldCheck, AlertTriangle, Loader } from 'lucide-react'
import { usePosture } from '../../lib/api/security'
import { useFeatureEnabled } from '../../lib/api/features'
import type { PostureGrade, RemediationItem, RemediationSeverity } from '../../types/api'

// ---- Grade colour helpers ----

function gradeColor(grade: PostureGrade): string {
  switch (grade) {
    case 'A': return 'var(--status-ok)'
    case 'B': return '#4ade80'
    case 'C': return 'var(--status-warn)'
    case 'D': return '#fb923c'
    case 'F': return 'var(--status-error)'
  }
}

function gradeBg(grade: PostureGrade): string {
  switch (grade) {
    case 'A': return 'rgba(52,211,153,0.10)'
    case 'B': return 'rgba(74,222,128,0.10)'
    case 'C': return 'rgba(240,160,32,0.10)'
    case 'D': return 'rgba(251,146,60,0.10)'
    case 'F': return 'rgba(232,64,64,0.10)'
  }
}

function gradeBorder(grade: PostureGrade): string {
  switch (grade) {
    case 'A': return 'rgba(52,211,153,0.30)'
    case 'B': return 'rgba(74,222,128,0.30)'
    case 'C': return 'rgba(240,160,32,0.35)'
    case 'D': return 'rgba(251,146,60,0.35)'
    case 'F': return 'rgba(232,64,64,0.35)'
  }
}

// ---- Severity colour helpers ----

function severityColor(severity: RemediationSeverity): string {
  switch (severity) {
    case 'critical': return 'var(--status-error)'
    case 'high':     return '#fb923c'
    case 'medium':   return 'var(--status-warn)'
    case 'low':      return 'var(--text-muted)'
  }
}

function severityBg(severity: RemediationSeverity): string {
  switch (severity) {
    case 'critical': return 'rgba(232,64,64,0.12)'
    case 'high':     return 'rgba(251,146,60,0.12)'
    case 'medium':   return 'rgba(240,160,32,0.10)'
    case 'low':      return 'rgba(74,82,104,0.15)'
  }
}

function severityBorder(severity: RemediationSeverity): string {
  switch (severity) {
    case 'critical': return 'rgba(232,64,64,0.35)'
    case 'high':     return 'rgba(251,146,60,0.35)'
    case 'medium':   return 'rgba(240,160,32,0.35)'
    case 'low':      return 'var(--border-default)'
  }
}

// ---- Score bar ----

function ScoreBar({ score, grade }: { score: number; grade: PostureGrade }) {
  return (
    <div
      className="h-1.5 rounded-full overflow-hidden"
      style={{ backgroundColor: 'var(--border-subtle)', width: '100%' }}
    >
      <div
        className="h-full rounded-full transition-all"
        style={{
          width: `${score}%`,
          backgroundColor: gradeColor(grade),
        }}
      />
    </div>
  )
}

// ---- Remediation row ----

function RemediationRow({ item }: { item: RemediationItem }) {
  return (
    <div
      className="flex items-start gap-3 px-3 py-2.5"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <span
        className="font-mono text-xs px-1.5 py-0.5 shrink-0 uppercase tracking-wider"
        style={{
          background: severityBg(item.severity),
          border: `1px solid ${severityBorder(item.severity)}`,
          color: severityColor(item.severity),
          borderRadius: '3px',
          fontSize: '11px',
        }}
      >
        {item.severity}
      </span>
      <div className="flex-1 min-w-0">
        <div className="text-xs" style={{ color: 'var(--text-primary)', lineHeight: '1.4' }}>
          {item.title}
        </div>
        <div className="text-xs mt-0.5" style={{ color: 'var(--text-muted)', lineHeight: '1.4' }}>
          {item.action}
        </div>
      </div>
    </div>
  )
}

// ---- Data source pill ----

function DataSourcePill({ label, available }: { label: string; available: boolean }) {
  return (
    <span
      className="inline-flex items-center gap-1 text-xs px-1.5 py-0.5 font-mono"
      style={{
        background: available ? 'rgba(52,211,153,0.08)' : 'rgba(74,82,104,0.15)',
        border: `1px solid ${available ? 'rgba(52,211,153,0.25)' : 'var(--border-subtle)'}`,
        color: available ? 'var(--status-ok)' : 'var(--text-muted)',
        borderRadius: '3px',
        fontSize: '11px',
      }}
    >
      {label}
      {!available && (
        <span style={{ fontSize: '10px', opacity: 0.7 }}>n/a</span>
      )}
    </span>
  )
}

// ---- Public component ----

interface PostureCardProps {
  nodeId: string
}

export function PostureCard({ nodeId }: PostureCardProps) {
  const postureEnabled = useFeatureEnabled('feature.posture_score')
  const { data, isLoading, isError } = usePosture(nodeId, nodeId !== '' && postureEnabled)

  // Hidden entirely when the posture-score feature is disabled.
  if (!postureEnabled) return null

  if (isLoading) {
    return (
      <div
        className="flex items-center gap-2 px-3 py-4"
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
        }}
      >
        <Loader size={12} className="animate-spin" style={{ color: 'var(--accent)' }} />
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          Computing posture score...
        </span>
      </div>
    )
  }

  if (isError || !data) {
    return (
      <div
        className="flex items-center gap-2 px-3 py-4"
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
        }}
      >
        <AlertTriangle size={12} style={{ color: 'var(--status-warn)' }} />
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          Posture score unavailable.
        </span>
      </div>
    )
  }

  const remediations = data.remediation ?? []
  const sources = data.data_sources ?? {}

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
      }}
    >
      {/* Card header: grade + score */}
      <div
        className="px-3 py-3 flex items-center gap-3"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <ShieldCheck size={14} style={{ color: gradeColor(data.grade), flexShrink: 0 }} />
        <span
          className="text-xs font-medium uppercase tracking-wider"
          style={{ color: 'var(--text-muted)' }}
        >
          Posture Score
        </span>
        <div className="ml-auto flex items-center gap-3">
          {/* Numeric score */}
          <span
            className="font-mono text-sm font-medium"
            style={{ color: gradeColor(data.grade) }}
          >
            {data.score}/100
          </span>
          {/* Letter grade badge */}
          <span
            className="font-mono text-sm font-bold px-2 py-0.5"
            style={{
              background: gradeBg(data.grade),
              border: `1px solid ${gradeBorder(data.grade)}`,
              color: gradeColor(data.grade),
              borderRadius: '3px',
              minWidth: '28px',
              textAlign: 'center',
            }}
          >
            {data.grade}
          </span>
        </div>
      </div>

      {/* Score bar */}
      <div className="px-3 py-2" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
        <ScoreBar score={data.score} grade={data.grade} />
      </div>

      {/* Data sources */}
      <div
        className="px-3 py-2 flex flex-wrap gap-1.5"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        {Object.entries(sources).map(([key, available]) => (
          <DataSourcePill key={key} label={key} available={available} />
        ))}
      </div>

      {/* Remediation list */}
      {remediations.length === 0 ? (
        <div className="px-3 py-3 text-xs" style={{ color: 'var(--text-muted)' }}>
          No remediation items — all checked signals are clean.
        </div>
      ) : (
        <div>
          {remediations.map((item, i) => (
            <RemediationRow key={`${item.metric}-${item.severity}-${i}`} item={item} />
          ))}
        </div>
      )}
    </div>
  )
}
