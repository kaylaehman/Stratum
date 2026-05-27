import { CheckCircle2, AlertTriangle, XCircle } from 'lucide-react'
import type { DiagnosticStep as DiagnosticStepType } from '../../types/api'

interface StepConfig {
  color: string
  bg: string
  icon: React.ReactNode
}

function stepConfig(status: DiagnosticStepType['status']): StepConfig {
  switch (status) {
    case 'ok':
      return {
        color: 'var(--status-ok)',
        bg: 'rgba(34,201,122,0.06)',
        icon: <CheckCircle2 size={13} />,
      }
    case 'warn':
      return {
        color: 'var(--status-warn)',
        bg: 'rgba(240,160,32,0.06)',
        icon: <AlertTriangle size={13} />,
      }
    case 'bad':
      return {
        color: 'var(--status-error)',
        bg: 'rgba(232,64,64,0.06)',
        icon: <XCircle size={13} />,
      }
  }
}

interface Props {
  step: DiagnosticStepType
}

export function DiagnosticStep({ step }: Props) {
  const cfg = stepConfig(step.status)

  return (
    <div
      className="flex items-start gap-3 px-3 py-2.5"
      style={{
        borderLeft: `3px solid ${cfg.color}`,
        backgroundColor: cfg.bg,
        borderRadius: '0 3px 3px 0',
      }}
    >
      <span style={{ color: cfg.color, marginTop: '1px', flexShrink: 0 }}>
        {cfg.icon}
      </span>
      <div className="flex flex-col gap-0.5 min-w-0">
        <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
          {step.label}
        </span>
        {step.detail && (
          <span
            className="font-mono text-xs break-all"
            style={{ color: 'var(--text-secondary)' }}
          >
            {step.detail}
          </span>
        )}
      </div>
    </div>
  )
}
