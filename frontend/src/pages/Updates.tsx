import { useState } from 'react'
import { ArrowUpCircle, CheckCircle, HelpCircle, RefreshCw, Loader, RotateCcw, AlertTriangle, Info } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { useTree } from '../lib/api/tree'
import { useUpdates, useRescanUpdates } from '../lib/api/updates'
import type { UpdatesSummary, UnknownBreakdownItem, OverlapNode } from '../lib/api/updates'
import { useUpdateContainer } from '../lib/api/recreate'
import { ApiError } from '../lib/api'
import type { ImageUpdate, UpdateStatus, TreeNode } from '../types/api'

// ---- Helpers ----

function resolveContainerName(containerId: string, nodes: TreeNode[]): string {
  for (const node of nodes) {
    const found = node.containers.find((c) => c.id === containerId)
    if (found) return found.name
  }
  return containerId
}

function resolveNodeName(nodeId: string, nodes: TreeNode[]): string {
  return nodes.find((n) => n.id === nodeId)?.name ?? nodeId
}

function sortUpdates(updates: ImageUpdate[]): ImageUpdate[] {
  const order: Record<UpdateStatus, number> = {
    update_available: 0,
    unknown: 1,
    up_to_date: 2,
  }
  return [...updates].sort((a, b) => order[a.status] - order[b.status])
}

// ---- Status chip ----

interface StatusChipProps {
  status: UpdateStatus
  unknownReason?: string
}

function StatusChip({ status, unknownReason }: StatusChipProps) {
  if (status === 'update_available') {
    return (
      <span
        className="flex items-center gap-1 font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
        style={{
          color: 'var(--status-warn)',
          background: 'rgba(240,160,32,0.12)',
          border: '1px solid rgba(240,160,32,0.4)',
          borderRadius: '3px',
          fontSize: '12px',
          whiteSpace: 'nowrap',
        }}
      >
        <ArrowUpCircle size={10} />
        Update available
      </span>
    )
  }

  if (status === 'up_to_date') {
    return (
      <span
        className="flex items-center gap-1 font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
        style={{
          color: 'var(--status-ok, #40c878)',
          background: 'rgba(64,200,120,0.1)',
          border: '1px solid rgba(64,200,120,0.3)',
          borderRadius: '3px',
          fontSize: '12px',
          whiteSpace: 'nowrap',
        }}
      >
        <CheckCircle size={10} />
        Up to date
      </span>
    )
  }

  return (
    <span
      className="flex items-center gap-1 font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
      title={unknownReason || undefined}
      style={{
        color: 'var(--text-muted)',
        background: 'transparent',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        fontSize: '12px',
        whiteSpace: 'nowrap',
        cursor: unknownReason ? 'help' : undefined,
      }}
    >
      <HelpCircle size={10} />
      Unknown
    </span>
  )
}

// ---- Update confirm box ----

interface UpdateConfirmProps {
  containerName: string
  onConfirm: () => void
  onCancel: () => void
  isPending: boolean
  errorMsg: string | null
}

function UpdateConfirm({ containerName, onConfirm, onCancel, isPending, errorMsg }: UpdateConfirmProps) {
  return (
    <div
      className="flex flex-col gap-2 p-3 mt-1"
      style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--status-warn)',
        borderRadius: '3px',
      }}
    >
      <p className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
        Update <strong>{containerName}</strong>?
      </p>
      <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
        Pulls the latest image and recreates the container. A snapshot is saved first so you can roll back.
      </p>
      {errorMsg && (
        <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
          {errorMsg}
        </span>
      )}
      <div className="flex items-center gap-2 mt-1">
        <button
          type="button"
          disabled={isPending}
          onClick={onConfirm}
          className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
          style={{
            background: 'rgba(240,160,32,0.15)',
            border: '1px solid var(--status-warn)',
            color: isPending ? 'var(--text-muted)' : 'var(--status-warn)',
            borderRadius: '3px',
            cursor: isPending ? 'not-allowed' : 'pointer',
            opacity: isPending ? 0.6 : 1,
          }}
        >
          {isPending ? <Loader size={12} className="animate-spin" /> : <ArrowUpCircle size={12} />}
          Update now
        </button>
        <button
          type="button"
          disabled={isPending}
          onClick={onCancel}
          className="font-mono text-xs px-2.5 py-1"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: isPending ? 'not-allowed' : 'pointer',
          }}
        >
          Cancel
        </button>
      </div>
    </div>
  )
}

// ---- Table row ----

interface UpdateRowProps {
  update: ImageUpdate
  containerName: string
  nodeName: string
  isAdmin: boolean
}

function UpdateRow({ update, containerName, nodeName, isAdmin }: UpdateRowProps) {
  const [confirming, setConfirming] = useState(false)
  const { mutate: doUpdate, isPending, error, reset, isSuccess } = useUpdateContainer()

  const errorMsg = error
    ? (error as ApiError).status === 502
      ? 'Update failed — container could not be recreated'
      : (error as ApiError).status === 404
      ? 'Container not found'
      : 'Update failed'
    : null

  const cellStyle = { borderBottom: '1px solid var(--border-subtle)' }

  return (
    <>
      <tr>
        <td
          className="px-3 py-2 font-mono text-xs"
          style={{ color: 'var(--text-primary)', ...cellStyle }}
        >
          {containerName}
        </td>
        <td
          className="px-3 py-2 text-xs"
          style={{ color: 'var(--text-secondary)', ...cellStyle }}
        >
          {nodeName}
        </td>
        <td
          className="px-3 py-2 font-mono text-xs"
          style={{ color: 'var(--text-secondary)', ...cellStyle, maxWidth: '240px' }}
        >
          <span className="truncate block" title={update.image}>{update.image}</span>
        </td>
        <td
          className="px-3 py-2"
          style={cellStyle}
        >
          {isSuccess ? (
            <span
              className="flex items-center gap-1 font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
              style={{
                color: 'var(--status-ok, #40c878)',
                background: 'rgba(64,200,120,0.1)',
                border: '1px solid rgba(64,200,120,0.3)',
                borderRadius: '3px',
                fontSize: '12px',
                whiteSpace: 'nowrap',
              }}
            >
              <CheckCircle size={10} />
              Updated
            </span>
          ) : (
            <StatusChip status={update.status} unknownReason={update.unknown_reason} />
          )}
        </td>
        <td
          className="px-3 py-2 text-xs"
          style={{ color: 'var(--text-muted)', ...cellStyle, whiteSpace: 'nowrap' }}
        >
          {new Date(update.checked_at).toLocaleString()}
        </td>
        <td className="px-3 py-2" style={cellStyle}>
          {isAdmin && update.status === 'update_available' && !isSuccess && (
            <button
              type="button"
              disabled={confirming || isPending}
              onClick={() => { reset(); setConfirming(true) }}
              className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: confirming || isPending ? 'var(--text-muted)' : 'var(--accent)',
                borderRadius: '3px',
                cursor: confirming || isPending ? 'default' : 'pointer',
                opacity: confirming || isPending ? 0.5 : 1,
                whiteSpace: 'nowrap',
              }}
            >
              {isPending ? <Loader size={11} className="animate-spin" /> : <RotateCcw size={11} />}
              Update
            </button>
          )}
        </td>
      </tr>
      {confirming && (
        <tr>
          <td colSpan={6} className="px-3 pb-2" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
            <UpdateConfirm
              containerName={containerName}
              onConfirm={() => {
                doUpdate(update.container_id, {
                  onSuccess: () => setConfirming(false),
                  onError: () => {/* keep open to show error */},
                })
              }}
              onCancel={() => { reset(); setConfirming(false) }}
              isPending={isPending}
              errorMsg={errorMsg}
            />
          </td>
        </tr>
      )}
    </>
  )
}

// ---- Summary bar ----

function SummaryBar({ updates }: { updates: ImageUpdate[] }) {
  const available = updates.filter((u) => u.status === 'update_available').length
  const upToDate = updates.filter((u) => u.status === 'up_to_date').length
  const unknown = updates.filter((u) => u.status === 'unknown').length

  // flex-wrap lets the count spans stack on narrow screens instead of overflowing
  return (
    <div className="flex items-center gap-4 mb-5 flex-wrap">
      <span
        className="text-xs font-medium"
        style={{ color: available > 0 ? 'var(--status-warn)' : 'var(--text-muted)' }}
      >
        {available} update{available !== 1 ? 's' : ''} available
      </span>
      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
        {upToDate} up to date
      </span>
      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
        {unknown} unknown
      </span>
    </div>
  )
}

// ---- Unknown reason banner ----

type BannerVariant = 'benign' | 'actionable'

function categoryCopy(category: string, count: number): { message: string; variant: BannerVariant } {
  switch (category) {
    case 'locally_built':
      return {
        message: `${count} image${count !== 1 ? 's are' : ' is'} built locally — there's nothing to check against a registry.`,
        variant: 'benign',
      }
    case 'rate_limited':
      return {
        message: `Docker Hub anonymous rate limit hit — add registry auth to resume checks (${count} image${count !== 1 ? 's' : ''} affected).`,
        variant: 'actionable',
      }
    case 'registry_unreachable':
      return {
        message: `The backend can't reach the registry — check its network egress (${count} image${count !== 1 ? 's' : ''} affected).`,
        variant: 'actionable',
      }
    case 'auth':
      return {
        message: `Registry authentication required for ${count} image${count !== 1 ? 's' : ''}.`,
        variant: 'actionable',
      }
    case 'daemon_error':
      return {
        message: `Docker daemon reported errors while checking ${count} image${count !== 1 ? 's' : ''} — check agent/daemon health.`,
        variant: 'actionable',
      }
    case 'empty_digest':
      return {
        message: `${count} image${count !== 1 ? 's have' : ' has'} no digest — the image may not have been pushed to a registry.`,
        variant: 'actionable',
      }
    default:
      return {
        message: `${count} image${count !== 1 ? 's' : ''} could not be checked — unknown reason.`,
        variant: 'actionable',
      }
  }
}

interface UnknownReasonBannerProps {
  summary: UpdatesSummary
}

function UnknownReasonBanner({ summary }: UnknownReasonBannerProps) {
  const shouldShow =
    summary.unknown > 0 &&
    (summary.unknown >= summary.total / 2 || summary.dominant_unknown_count > 0)

  if (!shouldShow) return null

  const dominant = summary.dominant_unknown_category || 'other'
  const { message, variant } = categoryCopy(dominant, summary.dominant_unknown_count || summary.unknown)

  const isBenign = variant === 'benign'

  const bannerStyle: React.CSSProperties = isBenign
    ? {
        borderColor: 'var(--border-subtle)',
        color: 'var(--text-muted)',
        backgroundColor: 'rgba(74,82,104,0.08)',
      }
    : {
        borderColor: 'rgba(240,160,32,0.5)',
        color: 'var(--status-warn)',
        backgroundColor: 'rgba(240,160,32,0.07)',
      }

  const secondaryRows = summary.unknown_breakdown.filter(
    (b) => b.category !== dominant && b.count > 0,
  )

  return (
    <div
      className="flex flex-col gap-2 px-3 py-2.5 mb-4 text-xs"
      style={{
        border: '1px solid',
        borderRadius: '3px',
        ...bannerStyle,
      }}
    >
      <div className="flex items-start gap-2">
        {isBenign ? (
          <Info size={13} style={{ flexShrink: 0, marginTop: '1px', color: 'var(--text-muted)' }} />
        ) : (
          <AlertTriangle size={13} style={{ flexShrink: 0, marginTop: '1px' }} />
        )}
        <span>{message}</span>
      </div>

      {secondaryRows.length > 0 && (
        <div className="flex flex-wrap gap-1.5 pl-5">
          {secondaryRows.map((b: UnknownBreakdownItem) => (
            <span
              key={b.category}
              title={b.example_reason || undefined}
              className="font-mono"
              style={{
                fontSize: '11px',
                padding: '1px 6px',
                border: '1px solid var(--border-subtle)',
                borderRadius: '3px',
                color: 'var(--text-muted)',
                cursor: b.example_reason ? 'help' : undefined,
                whiteSpace: 'nowrap',
              }}
            >
              {b.category} · {b.count}
            </span>
          ))}
        </div>
      )}
    </div>
  )
}

// ---- Overlap (Watchtower/Portainer) banner ----

function OverlapBanner({ overlaps }: { overlaps: OverlapNode[] }) {
  if (overlaps.length === 0) return null

  // Any auto-updater (Watchtower) makes this an active conflict, not just a note.
  const hasConflict = overlaps.some((o) => o.auto_updates)
  const accent = hasConflict ? 'var(--status-warn)' : 'var(--text-muted)'

  return (
    <div
      className="px-3 py-2.5 mb-3 text-xs"
      style={{
        backgroundColor: hasConflict ? 'rgba(255,180,0,0.07)' : 'var(--bg-surface)',
        border: `1px solid ${hasConflict ? 'var(--status-warn)' : 'var(--border-subtle)'}`,
        borderRadius: '3px',
      }}
    >
      <div className="flex items-start gap-2">
        <AlertTriangle size={13} style={{ color: accent, flexShrink: 0, marginTop: '1px' }} />
        <div className="flex flex-col gap-1">
          <span style={{ color: 'var(--text-secondary)' }}>
            Another container-management tool is running on{' '}
            {overlaps.length === 1 ? 'a host' : `${overlaps.length} hosts`}. Updating here may
            conflict with it.
          </span>
          <div className="flex flex-col gap-0.5 pl-0.5">
            {overlaps.map((o) => (
              <span key={o.node_id} className="font-mono" style={{ color: 'var(--text-muted)' }}>
                {o.node_name}: {o.managers.map((m) => m.name).join(', ')}
                {o.auto_updates && (
                  <span style={{ color: 'var(--status-warn)' }}>
                    {' '}
                    — auto-updates containers; Stratum skips these hosts in the auto-update
                    automation
                  </span>
                )}
              </span>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

// ---- Main page ----

const TABLE_COLS = ['Container', 'Node', 'Image', 'Status', 'Last Checked', '']

export default function Updates() {
  const { data: me } = useMe()
  const isAdmin = me?.role === 'admin'

  const { data: tree } = useTree()
  const nodes = tree?.nodes ?? []

  const { data, isLoading } = useUpdates()
  const { mutate: rescan, isPending: isRescanning } = useRescanUpdates()

  const updates = sortUpdates(data?.updates ?? [])
  const summary = data?.summary

  return (
    <AppShell>
      {/* max-w-full prevents page-level horizontal overflow on narrow viewports */}
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full max-w-full p-6"
        style={{ maxWidth: '1100px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-2">
            <ArrowUpCircle size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1
              className="text-sm font-medium uppercase tracking-wider"
              style={{ color: 'var(--text-primary)' }}
            >
              Update Assistant
            </h1>
          </div>

          {isAdmin && (
            <button
              type="button"
              onClick={() => rescan()}
              disabled={isRescanning || isLoading}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{
                backgroundColor: isRescanning ? 'rgba(74,82,104,0.2)' : 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: isRescanning ? 'var(--text-muted)' : 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: isRescanning ? 'default' : 'pointer',
                opacity: isLoading ? 0.5 : 1,
              }}
            >
              {isRescanning ? (
                <>
                  <Loader size={11} className="animate-spin" />
                  Checking registries…
                </>
              ) : (
                <>
                  <RefreshCw size={11} />
                  Rescan
                </>
              )}
            </button>
          )}
        </div>

        {/* Loading state */}
        {isLoading && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading update status…
            </span>
          </div>
        )}

        {/* Content */}
        {!isLoading && (
          <>
            <OverlapBanner overlaps={data?.overlaps ?? []} />
            {updates.length === 0 ? (
              <div
                className="px-3 py-4 text-xs"
                style={{
                  color: 'var(--text-muted)',
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                }}
              >
                No containers checked yet — try Rescan.
              </div>
            ) : (
              <>
                <SummaryBar updates={updates} />

                {summary && <UnknownReasonBanner summary={summary} />}

                <div
                  style={{
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                    overflowX: 'auto',
                  }}
                >
                  <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                    <thead>
                      <tr>
                        {TABLE_COLS.map((col) => (
                          <th
                            key={col}
                            className="px-3 py-2 text-left text-xs uppercase tracking-wider font-medium"
                            style={{
                              color: 'var(--text-muted)',
                              borderBottom: '1px solid var(--border-subtle)',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {col}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {updates.map((u) => (
                        <UpdateRow
                          key={`${u.node_id}:${u.container_id}`}
                          update={u}
                          containerName={resolveContainerName(u.container_id, nodes)}
                          nodeName={resolveNodeName(u.node_id, nodes)}
                          isAdmin={isAdmin}
                        />
                      ))}
                    </tbody>
                  </table>
                </div>
              </>
            )}

            {/* Footer note */}
            <div
              className="flex items-start gap-2 mt-5 px-3 py-2.5 text-xs"
              style={{
                backgroundColor: 'rgba(74,82,104,0.12)',
                border: '1px solid var(--border-subtle)',
                borderRadius: '3px',
                color: 'var(--text-muted)',
                lineHeight: '1.6',
              }}
            >
              <HelpCircle size={12} style={{ flexShrink: 0, marginTop: '1px' }} />
              <span>
                <strong style={{ color: 'var(--text-secondary)' }}>Unknown</strong> status means the image
                was built locally, comes from a private registry, or the registry is rate-limiting checks.
                Updates pull the latest image and recreate the container — a snapshot is saved automatically before each update.
              </span>
            </div>
          </>
        )}
      </div>
    </AppShell>
  )
}
