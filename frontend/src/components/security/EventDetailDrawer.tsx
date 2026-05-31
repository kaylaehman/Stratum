import { useEffect } from 'react'
import { X, ExternalLink } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import type { IncidentEntry, IncidentSource, IncidentSeverity } from '../../lib/api/incidents'
import type { TreeNode } from '../../types/api'
import { resourceLink } from '../../lib/resourceLink'

// ---- Props ----

export interface EventDetailDrawerProps {
  entry: IncidentEntry
  nodes: TreeNode[]
  onClose: () => void
}

// ---- Helpers ----

const SEVERITY_COLOR: Record<IncidentSeverity, string> = {
  critical: 'var(--status-error)',
  warning: 'var(--status-warn)',
  info: 'var(--text-muted)',
}

const SOURCE_COLOR: Record<IncidentSource, string> = {
  activity: 'var(--accent)',
  container: '#6c8ebf',
  metric: '#d48d00',
  file_event: '#7b9e5f',
}

const SOURCE_LABEL: Record<IncidentSource, string> = {
  activity: 'Activity',
  container: 'Container',
  metric: 'Metric spike',
  file_event: 'File change',
}

function resolveNodeName(nodeId: string | undefined, nodes: TreeNode[]): string | undefined {
  if (!nodeId) return undefined
  return nodes.find((n) => n.id === nodeId)?.name
}

function resolveContainerName(
  targetId: string | undefined,
  targetType: string | undefined,
  nodes: TreeNode[],
): string | undefined {
  if (!targetId || targetType !== 'container') return undefined
  for (const node of nodes) {
    const ctr = node.containers.find((c) => c.id === targetId)
    if (ctr) return ctr.name
  }
  return undefined
}

function findNodeForContainer(containerId: string, nodes: TreeNode[]): TreeNode | undefined {
  return nodes.find((n) => n.containers.some((c) => c.id === containerId))
}

// ---- Row layout helper ----

function DetailRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-start gap-3 py-1.5" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
      <span
        className="text-xs shrink-0"
        style={{ color: 'var(--text-muted)', width: '96px' }}
      >
        {label}
      </span>
      <span
        className={`text-xs flex-1 min-w-0 break-words ${mono ? 'font-mono' : ''}`}
        style={{ color: 'var(--text-primary)' }}
      >
        {value}
      </span>
    </div>
  )
}

// ---- Drawer ----

export function EventDetailDrawer({ entry, nodes, onClose }: EventDetailDrawerProps) {
  const navigate = useNavigate()

  // Close on Escape key.
  useEffect(() => {
    function handleKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [onClose])

  const sevColor = SEVERITY_COLOR[entry.severity] ?? 'var(--text-muted)'
  const srcColor = SOURCE_COLOR[entry.source] ?? 'var(--text-muted)'
  const srcLabel = SOURCE_LABEL[entry.source] ?? entry.source

  const ts = new Date(entry.timestamp)
  const localTs = ts.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })

  const nodeName = resolveNodeName(entry.node_id, nodes)
  const containerName = resolveContainerName(entry.target_id, entry.target_type, nodes)

  // Build the deep-link for "View resource" button.
  function buildResourceLink(): string | undefined {
    if (entry.target_type === 'container' && entry.target_id) {
      const ownerNode = findNodeForContainer(entry.target_id, nodes)
      return resourceLink(ownerNode?.id, entry.target_id)
    }
    if (entry.node_id) {
      return resourceLink(entry.node_id)
    }
    return undefined
  }

  const link = buildResourceLink()

  return (
    <>
      {/* Backdrop */}
      <div
        onClick={onClose}
        style={{
          position: 'fixed',
          inset: 0,
          backgroundColor: 'rgba(0,0,0,0.45)',
          zIndex: 40,
        }}
      />

      {/* Slide-over panel */}
      <div
        style={{
          position: 'fixed',
          top: 0,
          right: 0,
          bottom: 0,
          width: '420px',
          maxWidth: '90vw',
          zIndex: 50,
          backgroundColor: 'var(--bg-base)',
          borderLeft: '1px solid var(--border-subtle)',
          display: 'flex',
          flexDirection: 'column',
          boxShadow: '-4px 0 24px rgba(0,0,0,0.3)',
        }}
      >
        {/* Header */}
        <div
          className="flex items-center justify-between px-4 py-3"
          style={{ borderBottom: '1px solid var(--border-subtle)', flexShrink: 0 }}
        >
          <div className="flex items-center gap-2">
            <span
              className="font-mono text-xs px-1.5 py-0.5"
              style={{
                color: srcColor,
                backgroundColor: 'var(--bg-elevated)',
                border: `1px solid ${srcColor}`,
                borderRadius: '3px',
              }}
            >
              {srcLabel}
            </span>
            <span
              className="font-mono text-xs"
              style={{ color: sevColor }}
            >
              {entry.severity}
            </span>
          </div>
          <button
            type="button"
            onClick={onClose}
            title="Close"
            className="flex items-center justify-center"
            style={{
              background: 'transparent',
              border: 'none',
              color: 'var(--text-muted)',
              cursor: 'pointer',
              padding: '4px',
              borderRadius: '3px',
            }}
          >
            <X size={14} />
          </button>
        </div>

        {/* Body */}
        <div className="flex flex-col flex-1 overflow-y-auto px-4 py-4 gap-1">
          {/* Summary */}
          <p
            className="text-sm font-medium mb-3"
            style={{ color: 'var(--text-primary)', lineHeight: '1.4' }}
          >
            {entry.summary}
          </p>

          {/* Detail rows */}
          <DetailRow label="When" value={localTs} />
          <DetailRow label="Type" value={srcLabel} />
          <DetailRow label="Severity" value={entry.severity} />
          {nodeName && <DetailRow label="Node" value={nodeName} mono />}
          {!nodeName && entry.node_id && <DetailRow label="Node" value={entry.node_id} mono />}
          {containerName && <DetailRow label="Container" value={containerName} mono />}
          {!containerName && entry.target_id && entry.target_type === 'container' && (
            <DetailRow label="Container" value={entry.target_id} mono />
          )}
          {entry.target_id && entry.target_type !== 'container' && (
            <DetailRow label={entry.target_type ?? 'Resource'} value={entry.target_id} mono />
          )}
        </div>

        {/* Footer */}
        <div
          className="flex items-center gap-2 px-4 py-3"
          style={{ borderTop: '1px solid var(--border-subtle)', flexShrink: 0 }}
        >
          {link && (
            <button
              type="button"
              onClick={() => { navigate(link); onClose() }}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{
                backgroundColor: 'var(--accent-glow)',
                border: '1px solid var(--accent-dim)',
                color: 'var(--accent)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <ExternalLink size={11} />
              View resource
            </button>
          )}
          <button
            type="button"
            onClick={onClose}
            className="flex items-center gap-1.5 text-xs px-3 py-1.5"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Close
          </button>
        </div>
      </div>
    </>
  )
}
