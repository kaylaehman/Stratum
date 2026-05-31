import { useState, useMemo } from 'react'
import { ShieldAlert, AlertTriangle, RefreshCw, Check, Network, Loader } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { useTree } from '../lib/api/tree'
import {
  usePrivileged,
  usePorts,
  useAcknowledgeFlag,
  useRescan,
} from '../lib/api/security'
import { useIncidentTimeline } from '../lib/api/incidents'
import type { IncidentEntry } from '../lib/api/incidents'
import { PostureCard } from '../components/security/PostureCard'
import { EventDetailDrawer } from '../components/security/EventDetailDrawer'
import type {
  FlaggedContainer,
  SecurityFlag,
  PortExposure,
  Listener,
  InterfaceClass,
} from '../types/api'
import type { TreeNode, Container } from '../types/api'

// ---- Helpers ----

function useContainerName(
  containerId: string,
  nodeId: string,
  nodes: TreeNode[],
): { containerName: string; nodeName: string } {
  const node = nodes.find((n) => n.id === nodeId)
  const container = node?.containers.find((c: Container) => c.id === containerId)
  return {
    containerName: container?.name ?? containerId,
    nodeName: node?.name ?? nodeId,
  }
}

function interfaceColor(cls: InterfaceClass): string {
  if (cls === 'all' || cls === 'external') return 'var(--status-error)'
  return 'var(--text-muted)'
}

function interfaceLabel(cls: InterfaceClass): string {
  if (cls === 'all') return 'ALL INTERFACES'
  if (cls === 'external') return 'EXTERNAL'
  return 'loopback'
}

// ---- Flag badge chip ----

function FlagTypeBadge({ type }: { type: string }) {
  return (
    <span
      className="font-mono text-xs px-1.5 py-0.5 shrink-0 uppercase tracking-wider"
      style={{
        background: 'rgba(232,64,64,0.12)',
        border: '1px solid rgba(232,64,64,0.35)',
        color: 'var(--status-error)',
        borderRadius: '3px',
        fontSize: '12px',
      }}
    >
      {type}
    </span>
  )
}

// ---- Flag row ----

interface FlagRowProps {
  flag: SecurityFlag
  containerId: string
  onAcknowledge: () => void
  isPending: boolean
}

function FlagRow({ flag, onAcknowledge, isPending }: FlagRowProps) {
  return (
    <div
      className="flex items-start gap-3 px-3 py-2.5"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <div className="flex items-center gap-2 shrink-0 pt-0.5">
        <FlagTypeBadge type={flag.type} />
        {flag.key && (
          <span
            className="font-mono text-xs"
            style={{ color: 'var(--text-secondary)', fontSize: '12px' }}
          >
            {flag.key}
          </span>
        )}
      </div>

      <span
        className="text-xs flex-1 min-w-0"
        style={{ color: 'var(--text-secondary)', lineHeight: '1.5' }}
      >
        {flag.risk}
      </span>

      <div className="shrink-0">
        {flag.acknowledged ? (
          <span
            className="flex items-center gap-1 text-xs px-2 py-1"
            style={{ color: 'var(--text-muted)' }}
          >
            <Check size={10} />
            acknowledged
          </span>
        ) : (
          <button
            type="button"
            onClick={onAcknowledge}
            disabled={isPending}
            className="flex items-center gap-1 text-xs px-2 py-1"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: isPending ? 'default' : 'pointer',
              opacity: isPending ? 0.6 : 1,
            }}
          >
            {isPending ? (
              <Loader size={10} className="animate-spin" />
            ) : (
              <Check size={10} />
            )}
            {isPending ? 'Acknowledging…' : 'Acknowledge'}
          </button>
        )}
      </div>
    </div>
  )
}

// ---- Flagged container card ----

interface FlaggedCardProps {
  item: FlaggedContainer
  nodes: TreeNode[]
}

function FlaggedCard({ item, nodes }: FlaggedCardProps) {
  const { containerName, nodeName } = useContainerName(item.container_id, item.node_id, nodes)
  const { mutate: acknowledge, isPending } = useAcknowledgeFlag()

  const unacknowledgedCount = item.flags.filter((f) => !f.acknowledged).length

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        marginBottom: '12px',
      }}
    >
      {/* Card header */}
      <div
        className="px-3 py-2 flex items-center gap-2"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <ShieldAlert size={13} style={{ color: 'var(--status-error)', flexShrink: 0 }} />
        <span
          className="font-mono text-xs font-medium"
          style={{ color: 'var(--text-primary)' }}
        >
          {containerName}
        </span>
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          on {nodeName}
        </span>
        <span
          className="ml-auto text-xs px-1.5 py-0.5 font-mono"
          style={{
            background:
              unacknowledgedCount > 0 ? 'rgba(232,64,64,0.12)' : 'rgba(74,82,104,0.2)',
            border: `1px solid ${unacknowledgedCount > 0 ? 'rgba(232,64,64,0.35)' : 'var(--border-default)'}`,
            color: unacknowledgedCount > 0 ? 'var(--status-error)' : 'var(--text-muted)',
            borderRadius: '3px',
          }}
        >
          {unacknowledgedCount} unacknowledged
        </span>
      </div>

      {/* Flags */}
      {item.flags.map((flag, i) => (
        <FlagRow
          key={`${flag.type}-${flag.key}-${i}`}
          flag={flag}
          containerId={item.container_id}
          isPending={isPending}
          onAcknowledge={() =>
            acknowledge({
              container_id: item.container_id,
              flag_type: flag.type,
              flag_key: flag.key,
            })
          }
        />
      ))}
    </div>
  )
}

// ---- Port row ----

interface PortRowProps {
  port: PortExposure
  nodes: TreeNode[]
}

function PortRow({ port, nodes }: PortRowProps) {
  const { containerName, nodeName } = useContainerName(port.container_id, port.node_id, nodes)

  return (
    <tr>
      <td
        className="px-3 py-2 font-mono text-xs"
        style={{ color: 'var(--text-primary)', borderBottom: '1px solid var(--border-subtle)' }}
      >
        {containerName}
        <span className="ml-1" style={{ color: 'var(--text-muted)', fontSize: '12px' }}>
          ({nodeName})
        </span>
      </td>
      <td
        className="px-3 py-2 font-mono text-xs"
        style={{ color: 'var(--text-primary)', borderBottom: '1px solid var(--border-subtle)' }}
      >
        {port.host_ip}:{port.host_port}
      </td>
      <td
        className="px-3 py-2 font-mono text-xs"
        style={{ color: 'var(--text-secondary)', borderBottom: '1px solid var(--border-subtle)' }}
      >
        {port.container_port}
      </td>
      <td
        className="px-3 py-2 font-mono text-xs uppercase"
        style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-subtle)' }}
      >
        {port.protocol}
      </td>
      <td
        className="px-3 py-2 font-mono text-xs"
        style={{
          color: interfaceColor(port.interface_class),
          borderBottom: '1px solid var(--border-subtle)',
        }}
      >
        {interfaceLabel(port.interface_class)}
      </td>
      <td
        className="px-3 py-2"
        style={{ borderBottom: '1px solid var(--border-subtle)', width: '48px' }}
      >
        {port.is_new && (
          <span
            className="text-xs px-1.5 py-0.5 font-mono"
            style={{
              background: 'rgba(240,160,32,0.12)',
              border: '1px solid rgba(240,160,32,0.4)',
              color: 'var(--status-warn)',
              borderRadius: '3px',
            }}
          >
            NEW
          </span>
        )}
      </td>
    </tr>
  )
}

// ---- Non-docker listener row ----

interface ListenerRowProps {
  listener: Listener
  nodes: TreeNode[]
}

function ListenerRow({ listener, nodes }: ListenerRowProps) {
  const node = nodes.find((n) => n.id === listener.node_id)
  const nodeName = node?.name ?? listener.node_id

  return (
    <div
      className="flex items-center gap-4 px-3 py-2"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <span className="font-mono text-xs" style={{ color: 'var(--text-muted)', width: '100px', flexShrink: 0 }}>
        {nodeName}
      </span>
      <span className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
        {listener.address}:{listener.port}
      </span>
      <span
        className="font-mono text-xs uppercase"
        style={{ color: 'var(--text-muted)', width: '40px', flexShrink: 0 }}
      >
        {listener.protocol}
      </span>
      <span className="font-mono text-xs truncate" style={{ color: 'var(--text-secondary)' }}>
        {listener.process}
      </span>
    </div>
  )
}

// ---- Event row (incidents list on security page) ----

const INCIDENT_SEVERITY_COLOR: Record<string, string> = {
  critical: 'var(--status-error)',
  warning: 'var(--status-warn)',
  info: 'var(--text-muted)',
}

const INCIDENT_SOURCE_COLOR: Record<string, string> = {
  activity: 'var(--accent)',
  container: '#6c8ebf',
  metric: '#d48d00',
  file_event: '#7b9e5f',
}

const INCIDENT_SOURCE_LABEL: Record<string, string> = {
  activity: 'activity',
  container: 'container',
  metric: 'metric',
  file_event: 'file',
}

interface EventRowProps {
  entry: IncidentEntry
  onClick: () => void
}

function EventRow({ entry, onClick }: EventRowProps) {
  const sevColor = INCIDENT_SEVERITY_COLOR[entry.severity] ?? 'var(--text-muted)'
  const srcColor = INCIDENT_SOURCE_COLOR[entry.source] ?? 'var(--text-muted)'
  const srcLabel = INCIDENT_SOURCE_LABEL[entry.source] ?? entry.source

  const ts = new Date(entry.timestamp)
  const timeStr = ts.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  const dateStr = ts.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })

  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center gap-3 px-3 py-2.5 w-full text-left"
      style={{
        background: 'transparent',
        border: 'none',
        borderBottom: '1px solid var(--border-subtle)',
        cursor: 'pointer',
      }}
    >
      {/* Severity dot */}
      <span
        title={entry.severity}
        style={{ width: 8, height: 8, borderRadius: '50%', backgroundColor: sevColor, flexShrink: 0 }}
      />

      {/* Timestamp */}
      <div className="shrink-0" style={{ minWidth: 88 }}>
        <span className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
          {timeStr}
        </span>
        <span className="text-xs font-mono ml-1" style={{ color: 'var(--text-muted)' }}>
          {dateStr}
        </span>
      </div>

      {/* Source badge */}
      <span
        className="font-mono text-xs px-1.5 py-0.5 shrink-0"
        style={{
          color: srcColor,
          backgroundColor: 'var(--bg-elevated)',
          border: `1px solid ${srcColor}`,
          borderRadius: '3px',
          opacity: 0.9,
        }}
      >
        {srcLabel}
      </span>

      {/* Summary */}
      <span
        className="text-xs truncate flex-1"
        style={{ color: 'var(--text-primary)' }}
        title={entry.summary}
      >
        {entry.summary}
      </span>

      {/* Severity label */}
      <span
        className="font-mono text-xs shrink-0"
        style={{ color: sevColor, minWidth: 52, textAlign: 'right' }}
      >
        {entry.severity}
      </span>
    </button>
  )
}

// ---- Section header ----

function SectionHeader({ label, count }: { label: string; count?: number }) {
  return (
    <div
      className="px-0 py-2 mb-3 flex items-center gap-2"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <span
        className="text-xs font-medium uppercase tracking-wider"
        style={{ color: 'var(--text-muted)' }}
      >
        {label}
      </span>
      {count !== undefined && (
        <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
          ({count})
        </span>
      )}
    </div>
  )
}

// ---- Admin gate ----

function AdminRequired() {
  return (
    <div
      className="flex flex-col items-center justify-center gap-3 py-16"
      style={{ color: 'var(--text-muted)' }}
    >
      <ShieldAlert size={28} style={{ color: 'var(--text-muted)' }} />
      <span className="text-xs uppercase tracking-wider">Admin access required</span>
      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
        The security audit is only accessible to administrators.
      </span>
    </div>
  )
}

// ---- Main Security page ----

export default function Security() {
  const { data: me, isLoading: meLoading } = useMe()
  const isAdmin = me?.role === 'admin'

  const { data: tree } = useTree()
  const nodes = tree?.nodes ?? []

  const [postureNodeId, setPostureNodeId] = useState('')
  const [selectedEvent, setSelectedEvent] = useState<IncidentEntry | null>(null)

  const { data: privileged, isLoading: privLoading } = usePrivileged(isAdmin)
  const { data: ports, isLoading: portsLoading } = usePorts(isAdmin)
  const { mutate: rescan, isPending: rescanning } = useRescan()

  // Load recent incidents (last 24h) for the events section.
  // useMemo keeps the filter object reference stable so React Query doesn't
  // treat each render as a new query key.
  const incidentFilters = useMemo(
    () => ({
      from: new Date(Date.now() - 24 * 3_600_000).toISOString(),
      to: new Date().toISOString(),
    }),
     
    [],
  )
  const { data: incidentData, isLoading: incidentsLoading } = useIncidentTimeline(
    isAdmin ? incidentFilters : {},
  )

  const isLoading = meLoading || privLoading || portsLoading || (isAdmin && incidentsLoading)

  // The backend marshals empty Go slices as JSON null, so ports.ports /
  // ports.non_docker_listeners can be null even when the object is present.
  // Normalize to arrays here so the render never does null.length / null.map.
  const portList = ports?.ports ?? []
  const listeners = ports?.non_docker_listeners ?? []

  return (
    <AppShell>
      <div className="flex flex-col flex-1 min-h-0 h-full w-full p-6" style={{ maxWidth: '900px', margin: '0 auto' }}>
        {/* Page header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-2">
            <ShieldAlert size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1 className="text-sm font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
              Security Audit
            </h1>
          </div>
          {isAdmin && (
            <button
              type="button"
              onClick={() => rescan(undefined)}
              disabled={rescanning}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: rescanning ? 'default' : 'pointer',
                opacity: rescanning ? 0.6 : 1,
              }}
            >
              <RefreshCw size={11} className={rescanning ? 'animate-spin' : ''} />
              {rescanning ? 'Rescanning...' : 'Rescan'}
            </button>
          )}
        </div>

        {/* Loading state */}
        {isLoading && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading security data...
            </span>
          </div>
        )}

        {/* Admin gate */}
        {!meLoading && !isAdmin && <AdminRequired />}

        {/* Content — only when admin and loaded */}
        {isAdmin && !isLoading && (
          <>
            {/* Section 0: Posture score (per-node) */}
            <div className="mb-8">
              <SectionHeader label="Security Posture Score" />
              <div className="flex items-center gap-2 mb-3">
                <span className="text-xs" style={{ color: 'var(--text-muted)', flexShrink: 0 }}>
                  Node
                </span>
                <select
                  value={postureNodeId}
                  onChange={(e) => setPostureNodeId(e.target.value)}
                  className="text-xs px-2 py-1 flex-1"
                  style={{
                    backgroundColor: 'var(--bg-elevated)',
                    border: '1px solid var(--border-default)',
                    color: 'var(--text-secondary)',
                    borderRadius: '3px',
                    maxWidth: '260px',
                  }}
                >
                  <option value="">— select a node —</option>
                  {nodes.map((n) => (
                    <option key={n.id} value={n.id}>{n.name}</option>
                  ))}
                </select>
              </div>
              {postureNodeId ? (
                <PostureCard nodeId={postureNodeId} />
              ) : (
                <div
                  className="px-3 py-4 text-xs"
                  style={{
                    color: 'var(--text-muted)',
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                  }}
                >
                  Select a node above to view its posture score.
                </div>
              )}
            </div>

            {/* Section 1: Privileged & flagged containers */}
            <div className="mb-8">
              <SectionHeader
                label="Privileged & Flagged Containers"
                count={privileged?.containers.length ?? 0}
              />
              {!privileged || privileged.containers.length === 0 ? (
                <div
                  className="px-3 py-4 text-xs"
                  style={{
                    color: 'var(--text-muted)',
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                  }}
                >
                  No flagged containers detected.
                </div>
              ) : (
                <div>
                  {privileged.containers.map((item) => (
                    <FlaggedCard
                      key={`${item.node_id}:${item.container_id}`}
                      item={item}
                      nodes={nodes}
                    />
                  ))}
                </div>
              )}
            </div>

            {/* Section 2: Exposed ports audit */}
            <div className="mb-8">
              <SectionHeader
                label="Exposed Ports Audit"
                count={portList.length}
              />

              {portList.length === 0 ? (
                <div
                  className="px-3 py-4 text-xs"
                  style={{
                    color: 'var(--text-muted)',
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                  }}
                >
                  No published ports detected.
                </div>
              ) : (
                <div
                  style={{
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                    overflowX: 'auto',
                    marginBottom: '16px',
                  }}
                >
                  <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                    <thead>
                      <tr>
                        {['Container', 'Host (ip:port)', 'Ctr Port', 'Proto', 'Interface', ''].map(
                          (col) => (
                            <th
                              key={col}
                              className="px-3 py-2 text-left text-xs uppercase tracking-wider font-medium"
                              style={{
                                color: 'var(--text-muted)',
                                borderBottom: '1px solid var(--border-subtle)',
                              }}
                            >
                              {col}
                            </th>
                          ),
                        )}
                      </tr>
                    </thead>
                    <tbody>
                      {portList.map((p) => (
                        <PortRow key={p.id} port={p} nodes={nodes} />
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              {/* Non-Docker host listeners */}
              <div className="mt-4">
                <div
                  className="flex items-center gap-2 mb-2"
                  style={{ borderBottom: '1px solid var(--border-subtle)', paddingBottom: '6px' }}
                >
                  <Network size={12} style={{ color: 'var(--text-muted)' }} />
                  <span
                    className="text-xs uppercase tracking-wider"
                    style={{ color: 'var(--text-muted)' }}
                  >
                    Non-Docker Host Listeners
                  </span>
                </div>
                {listeners.length === 0 ? (
                  <div className="px-3 py-3 text-xs" style={{ color: 'var(--text-muted)' }}>
                    None detected.
                  </div>
                ) : (
                  <div
                    style={{
                      backgroundColor: 'var(--bg-surface)',
                      border: '1px solid var(--border-subtle)',
                      borderRadius: '3px',
                    }}
                  >
                    {listeners.map((l: Listener, i: number) => (
                      <ListenerRow
                        key={`${l.node_id}:${l.address}:${l.port}:${i}`}
                        listener={l}
                        nodes={nodes}
                      />
                    ))}
                  </div>
                )}
              </div>
            </div>

            {/* Tip: external risk callout */}
            <div
              className="flex items-start gap-2 px-3 py-2.5 text-xs"
              style={{
                backgroundColor: 'rgba(232,64,64,0.07)',
                border: '1px solid rgba(232,64,64,0.2)',
                borderRadius: '3px',
                color: 'var(--text-secondary)',
              }}
            >
              <AlertTriangle size={12} style={{ color: 'var(--status-error)', flexShrink: 0, marginTop: '1px' }} />
              <span>
                Ports bound to <strong style={{ color: 'var(--status-error)' }}>0.0.0.0</strong> (all
                interfaces) are reachable from any network. Bind to <code>127.0.0.1</code> where
                external access is not required.
              </span>
            </div>

            {/* Section 3: Recent events (last 24 h) */}
            <div className="mt-8 mb-8">
              <SectionHeader
                label="Recent Events"
                count={incidentData?.entries.length ?? 0}
              />
              {!incidentData || incidentData.entries.length === 0 ? (
                <div
                  className="px-3 py-4 text-xs"
                  style={{
                    color: 'var(--text-muted)',
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                  }}
                >
                  No events in the last 24 hours.
                </div>
              ) : (
                <div
                  style={{
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                    overflow: 'hidden',
                  }}
                >
                  {incidentData.entries.map((entry, i) => (
                    <EventRow
                      key={`${entry.timestamp}-${entry.source}-${i}`}
                      entry={entry}
                      onClick={() => setSelectedEvent(entry)}
                    />
                  ))}
                </div>
              )}
            </div>

            {/* Event detail drawer */}
            {selectedEvent && (
              <EventDetailDrawer
                entry={selectedEvent}
                nodes={nodes}
                onClose={() => setSelectedEvent(null)}
              />
            )}
          </>
        )}
      </div>
    </AppShell>
  )
}
