import { useState } from 'react'
import {
  ShieldCheck,
  ShieldAlert,
  ShieldX,
  CheckCircle,
  XCircle,
  Play,
  Loader,
  ChevronDown,
  ChevronUp,
  Terminal,
  AlertTriangle,
} from 'lucide-react'
import { useCan } from '../../lib/roles'
import { useFeatureEnabled } from '../../lib/api/features'
import {
  useProposals,
  useApproveProposal,
  useRejectProposal,
  useExecuteProposal,
} from '../../lib/api/remediation'
import { ApiError } from '../../lib/api'
import type { RemediationProposal, RemediationRisk, RemediationStatus } from '../../types/api'

// ── Styles ────────────────────────────────────────────────────────────────────

function riskColor(risk: RemediationRisk): string {
  switch (risk) {
    case 'low':
      return 'var(--status-ok)'
    case 'medium':
      return 'var(--status-warn)'
    case 'high':
      return 'var(--status-error)'
    case 'destructive':
      return '#e83232'
    default:
      return 'var(--text-muted)'
  }
}

function statusIcon(status: RemediationStatus) {
  switch (status) {
    case 'proposed':
      return <ShieldAlert size={12} style={{ color: 'var(--status-warn)' }} />
    case 'approved':
      return <ShieldCheck size={12} style={{ color: 'var(--status-ok)' }} />
    case 'rejected':
      return <ShieldX size={12} style={{ color: 'var(--text-muted)' }} />
    case 'executed':
      return <CheckCircle size={12} style={{ color: 'var(--status-ok)' }} />
    case 'failed':
      return <XCircle size={12} style={{ color: 'var(--status-error)' }} />
    default:
      return null
  }
}

function badgeStyle(risk: RemediationRisk): React.CSSProperties {
  const color = riskColor(risk)
  return {
    padding: '1px 6px',
    borderRadius: '3px',
    border: `1px solid ${color}`,
    color,
    backgroundColor: `${color}18`,
    fontSize: '0.6rem',
    fontFamily: 'monospace',
    fontWeight: 600,
    textTransform: 'uppercase' as const,
    letterSpacing: '0.05em',
    flexShrink: 0,
  }
}

function btnStyle(
  variant: 'ok' | 'err' | 'warn' | 'neutral',
  disabled: boolean,
): React.CSSProperties {
  const colors = {
    ok: { bg: 'rgba(64,200,120,0.10)', border: 'var(--status-ok)', color: 'var(--status-ok)' },
    err: { bg: 'rgba(232,64,64,0.10)', border: 'var(--status-error)', color: 'var(--status-error)' },
    warn: { bg: 'rgba(240,160,32,0.10)', border: 'var(--status-warn)', color: 'var(--status-warn)' },
    neutral: { bg: 'var(--bg-elevated)', border: 'var(--border-default)', color: 'var(--text-secondary)' },
  }[variant]
  return {
    display: 'flex',
    alignItems: 'center',
    gap: '4px',
    background: colors.bg,
    border: `1px solid ${colors.border}`,
    color: disabled ? 'var(--text-muted)' : colors.color,
    borderRadius: '3px',
    padding: '4px 10px',
    fontSize: '0.7rem',
    fontFamily: 'monospace',
    cursor: disabled ? 'not-allowed' : 'pointer',
    opacity: disabled ? 0.6 : 1,
  }
}

function apiErrMsg(err: unknown): string {
  if (err instanceof ApiError) {
    const code = (err.body as { error?: string })?.error
    switch (code) {
      case 'not_approved': return 'Proposal must be approved before execution.'
      case 'already_terminal': return 'This proposal is already in a terminal state.'
      case 'invalid_transition': return 'Status transition is not valid.'
      case 'admin_required_for_destructive': return 'Only admins may approve/execute destructive proposals.'
      case 'node_cannot_exec': return 'Target node does not support remote execution.'
      case '2fa_required': return 'Confirm your identity (2FA) to proceed.'
      default: return `Error: ${code ?? err.status}`
    }
  }
  return 'An unexpected error occurred.'
}

// ── ProposalCard ──────────────────────────────────────────────────────────────

interface ProposalCardProps {
  proposal: RemediationProposal
  isAdmin: boolean
  isOperator: boolean
}

function ProposalCard({ proposal, isAdmin, isOperator }: ProposalCardProps) {
  const [expanded, setExpanded] = useState(false)
  const approve = useApproveProposal()
  const reject = useRejectProposal()
  const execute = useExecuteProposal()

  const isDestructive = proposal.risk_level === 'destructive'
  const canApprove = isDestructive ? isAdmin : isOperator
  const canExecute = canApprove
  const isPending = approve.isPending || reject.isPending || execute.isPending
  const isProposed = proposal.status === 'proposed'
  const isApproved = proposal.status === 'approved'
  const isTerminal = ['rejected', 'executed', 'failed'].includes(proposal.status)

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-elevated)',
        border: `1px solid ${isDestructive ? 'rgba(232,50,50,0.3)' : 'var(--border-subtle)'}`,
        borderRadius: '3px',
        overflow: 'hidden',
      }}
    >
      {/* Header */}
      <div
        style={{
          display: 'flex',
          alignItems: 'flex-start',
          gap: '8px',
          padding: '10px 12px',
          cursor: 'pointer',
        }}
        onClick={() => setExpanded((v) => !v)}
      >
        <div style={{ marginTop: '1px', flexShrink: 0 }}>
          {statusIcon(proposal.status)}
        </div>

        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
            <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
              {proposal.title}
            </span>
            <span style={badgeStyle(proposal.risk_level)}>{proposal.risk_level}</span>
            <span
              className="font-mono text-xs"
              style={{ color: 'var(--text-muted)', fontSize: '0.65rem' }}
            >
              {proposal.status}
            </span>
          </div>
          {proposal.rationale && (
            <p className="text-xs" style={{ color: 'var(--text-muted)', margin: '2px 0 0', lineHeight: 1.4 }}>
              {proposal.rationale}
            </p>
          )}
          {isDestructive && (
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '4px',
                marginTop: '4px',
                color: '#e83232',
                fontSize: '0.65rem',
                fontFamily: 'monospace',
              }}
            >
              <AlertTriangle size={10} />
              Destructive — requires admin approval and 2FA
            </div>
          )}
        </div>

        <div style={{ flexShrink: 0, color: 'var(--text-muted)' }}>
          {expanded ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
        </div>
      </div>

      {/* Expanded body */}
      {expanded && (
        <div
          style={{
            borderTop: '1px solid var(--border-subtle)',
            padding: '10px 12px',
            display: 'flex',
            flexDirection: 'column',
            gap: '10px',
          }}
        >
          {/* Dry-run / command preview */}
          <div>
            <div
              className="font-mono text-xs"
              style={{ color: 'var(--text-muted)', marginBottom: '4px', display: 'flex', alignItems: 'center', gap: '4px' }}
            >
              <Terminal size={10} />
              Commands (dry-run preview)
            </div>
            <pre
              style={{
                margin: 0,
                padding: '8px 10px',
                backgroundColor: 'var(--bg-surface)',
                border: '1px solid var(--border-subtle)',
                borderRadius: '3px',
                fontSize: '0.72rem',
                fontFamily: 'monospace',
                color: 'var(--text-secondary)',
                overflowX: 'auto',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
              }}
            >
              {proposal.commands.join('\n')}
            </pre>
          </div>

          {/* Source */}
          <div className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
            Source: {proposal.source} &nbsp;&bull;&nbsp; Node: {proposal.node_id}
            {proposal.container_id ? ` · Container: ${proposal.container_id}` : ''}
          </div>

          {/* Execution output */}
          {(proposal.stdout || proposal.stderr) && (
            <div>
              <div className="font-mono text-xs" style={{ color: 'var(--text-muted)', marginBottom: '4px' }}>
                Execution output
              </div>
              {proposal.stdout && (
                <pre
                  style={{
                    margin: 0,
                    padding: '6px 8px',
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                    fontSize: '0.7rem',
                    fontFamily: 'monospace',
                    color: 'var(--status-ok)',
                    overflowX: 'auto',
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-all',
                  }}
                >
                  {proposal.stdout}
                </pre>
              )}
              {proposal.stderr && (
                <pre
                  style={{
                    margin: '4px 0 0',
                    padding: '6px 8px',
                    backgroundColor: 'rgba(232,64,64,0.07)',
                    border: '1px solid var(--status-error)',
                    borderRadius: '3px',
                    fontSize: '0.7rem',
                    fontFamily: 'monospace',
                    color: 'var(--status-error)',
                    overflowX: 'auto',
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-all',
                  }}
                >
                  {proposal.stderr}
                </pre>
              )}
            </div>
          )}

          {/* Error display */}
          {(approve.isError || reject.isError || execute.isError) && (
            <p className="font-mono text-xs" style={{ color: 'var(--status-error)', margin: 0 }}>
              {apiErrMsg(approve.error ?? reject.error ?? execute.error)}
            </p>
          )}

          {/* Actions */}
          {!isTerminal && (
            <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
              {isProposed && canApprove && (
                <button
                  type="button"
                  disabled={isPending}
                  onClick={() => approve.mutate(proposal.id)}
                  style={btnStyle('ok', isPending)}
                >
                  {approve.isPending ? <Loader size={11} className="animate-spin" /> : <CheckCircle size={11} />}
                  Approve
                </button>
              )}
              {isProposed && isOperator && (
                <button
                  type="button"
                  disabled={isPending}
                  onClick={() => reject.mutate(proposal.id)}
                  style={btnStyle('err', isPending)}
                >
                  {reject.isPending ? <Loader size={11} className="animate-spin" /> : <XCircle size={11} />}
                  Reject
                </button>
              )}
              {isApproved && canExecute && (
                <button
                  type="button"
                  disabled={isPending}
                  onClick={() => execute.mutate(proposal.id)}
                  style={btnStyle(isDestructive ? 'err' : 'warn', isPending)}
                >
                  {execute.isPending ? <Loader size={11} className="animate-spin" /> : <Play size={11} />}
                  {isDestructive ? 'Execute (destructive)' : 'Execute'}
                </button>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// ── RemediationPanel (exported) ───────────────────────────────────────────────

interface RemediationPanelProps {
  /** Optional: filter proposals to a specific node. */
  nodeId?: string
}

export function RemediationPanel({ nodeId }: RemediationPanelProps) {
  const { isAdmin, isOperator } = useCan()
  const remediationEnabled = useFeatureEnabled('feature.remediation')
  const { data, isLoading } = useProposals(nodeId)

  if (!isOperator) return null
  // Hidden when the agentic-remediation feature is disabled.
  if (!remediationEnabled) return null

  const proposals = data?.proposals ?? []

  return (
    <section
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        padding: '16px 20px',
      }}
    >
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '14px' }}>
        <ShieldCheck size={15} style={{ color: 'var(--accent)', flexShrink: 0 }} />
        <div>
          <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)', margin: 0 }}>
            Remediation Proposals
          </h2>
          <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
            AI-generated or diagnostic-driven fixes — review, approve, then execute.
          </p>
        </div>
      </div>

      {isLoading && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-muted)' }}>
          <Loader size={13} className="animate-spin" />
          <span className="text-xs">Loading proposals…</span>
        </div>
      )}

      {!isLoading && proposals.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {proposals.map((p) => (
            <ProposalCard
              key={p.id}
              proposal={p}
              isAdmin={isAdmin}
              isOperator={isOperator}
            />
          ))}
        </div>
      )}

      {!isLoading && proposals.length === 0 && (
        <div
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            padding: '14px',
          }}
        >
          <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
            No remediation proposals yet. Proposals are generated automatically from diagnostic
            results or AI suggestions and appear here for review before any command is run.
          </p>
        </div>
      )}
    </section>
  )
}
