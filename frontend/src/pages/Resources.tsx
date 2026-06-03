import { useState, useRef, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Play, Square, RotateCw, PowerOff, Loader, BookmarkPlus, BookmarkCheck, AlertTriangle } from 'lucide-react'
import { WakeOnLan } from '../components/nodes/WakeOnLan'
import { SSHKeys } from '../components/nodes/SSHKeys'
import { Scheduler } from '../components/nodes/Scheduler'
import { ReverseProxyPanel } from '../components/proxy/ReverseProxyPanel'
import { ContainerProxySection } from '../components/proxy/ContainerProxySection'
import { DnsPanel } from '../components/dns/DnsPanel'
import { FileWatchPanel } from '../components/security/FileWatchPanel'
import { AppShell } from '../components/layout/AppShell'
import { ResourceTree } from '../components/tree/ResourceTree'
import { FileBrowser } from '../components/filesystem/FileBrowser'
import { ContainerFileBrowser } from '../components/filesystem/ContainerFileBrowser'
import { UidGidVisualizer } from '../components/permissions/UidGidVisualizer'
import { FileUidPanel } from '../components/permissions/FileUidPanel'
import { DiagnosticCard } from '../components/permissions/DiagnosticCard'
import { MountList } from '../components/containers/MountList'
import { HealthCheck } from '../components/containers/HealthCheck'
import { SharedMountsView } from '../components/containers/SharedMountsView'
import { ReverseMountPanel } from '../components/containers/ReverseMountPanel'
import { SnapshotsPanel } from '../components/containers/SnapshotsPanel'
import { MemoryPanel } from '../components/ai/MemoryPanel'
import { SSOPanel } from '../components/security/SSOPanel'
import { useContainerInspect } from '../lib/api/permissions'
import { useTreeStore } from '../store/tree'
import { useTree } from '../lib/api/tree'
import { useContainerLifecycle } from '../lib/api/containers'
import { useVMPowerAction } from '../lib/api/vms'
import type { VMAction } from '../lib/api/vms'
import { useNodePowerAction } from '../lib/api/nodepower'
import type { NodePowerAction } from '../types/api'
import { useAddBookmark } from '../lib/api/bookmarks'
import { useNodeGuestLinks } from '../hooks/useNodeGuestLinks'
import { useCan } from '../lib/roles'
import type { TreeSelection, ContainerStatus } from '../types/api'
import type { ContainerAction } from '../lib/api/containers'

function selectionTitle(sel: TreeSelection): string {
  switch (sel.kind) {
    case 'node':
      return 'Host'
    case 'vm':
      return sel.vmKind === 'lxc' ? 'LXC Container' : 'Virtual Machine'
    case 'container':
      return 'Docker Container'
    case 'fs-root':
      return 'Filesystem'
  }
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-baseline gap-3">
      <span className="text-xs w-28 shrink-0" style={{ color: 'var(--text-muted)' }}>
        {label}
      </span>
      <span
        className={`text-xs truncate ${mono ? 'font-mono' : ''}`}
        style={{ color: 'var(--text-primary)' }}
        title={value}
      >
        {value}
      </span>
    </div>
  )
}

interface LifecycleControlsProps {
  containerId: string
  status: ContainerStatus
}

function LifecycleControls({ containerId, status }: LifecycleControlsProps) {
  const { isOperator } = useCan()
  const { mutate, isPending, variables, error } = useContainerLifecycle()

  if (!isOperator) return null

  const isRunning = status === 'running'
  const inFlight = (action: ContainerAction) =>
    isPending && variables?.containerId === containerId && variables?.action === action

  function btn(
    action: ContainerAction,
    icon: React.ReactNode,
    label: string,
    disabled: boolean,
  ) {
    const loading = inFlight(action)
    return (
      <button
        key={action}
        type="button"
        disabled={disabled || isPending}
        onClick={() => mutate({ containerId, action })}
        title={label}
        className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          color: disabled || isPending ? 'var(--text-muted)' : 'var(--text-secondary)',
          borderRadius: '3px',
          cursor: disabled || isPending ? 'not-allowed' : 'pointer',
          opacity: disabled || isPending ? 0.5 : 1,
        }}
      >
        {loading ? <Loader size={12} className="animate-spin" /> : icon}
        {label}
      </button>
    )
  }

  const errorMsg = error
    ? (error as { body?: { error?: string } }).body?.error === 'node_unreachable'
      ? 'Action failed — node unreachable'
      : 'Action failed — lifecycle error'
    : null

  return (
    <div className="flex flex-col gap-1.5 mt-2 pt-2" style={{ borderTop: '1px solid var(--border-subtle)' }}>
      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Controls</span>
      <div className="flex items-center gap-2 flex-wrap">
        {btn('start', <Play size={12} />, 'Start', isRunning)}
        {btn('stop', <Square size={12} />, 'Stop', !isRunning)}
        {btn('restart', <RotateCw size={12} />, 'Restart', !isRunning)}
      </div>
      {errorMsg && (
        <span className="font-mono text-xs mt-0.5" style={{ color: 'var(--status-error)' }}>
          {errorMsg}
        </span>
      )}
    </div>
  )
}

interface VMLifecycleControlsProps {
  nodeId: string
  vmid: number
  status: string
}

function VMLifecycleControls({ nodeId, vmid, status }: VMLifecycleControlsProps) {
  const { isOperator } = useCan()
  const { mutate, isPending, variables, error } = useVMPowerAction()
  const [pendingConfirm, setPendingConfirm] = useState<VMAction | null>(null)

  if (!isOperator) return null

  const isRunning = status === 'running'
  const inFlight = (action: VMAction) =>
    isPending && variables?.nodeId === nodeId && variables?.vmid === vmid && variables?.action === action

  function triggerAction(action: VMAction) {
    // stop / reboot require a confirm step to prevent accidental power-off.
    if ((action === 'stop' || action === 'reboot') && pendingConfirm !== action) {
      setPendingConfirm(action)
      return
    }
    setPendingConfirm(null)
    mutate({ nodeId, vmid, action })
  }

  function btn(
    action: VMAction,
    icon: React.ReactNode,
    label: string,
    disabled: boolean,
  ) {
    const loading = inFlight(action)
    const confirming = pendingConfirm === action
    return (
      <button
        key={action}
        type="button"
        disabled={disabled || isPending}
        onClick={() => triggerAction(action)}
        title={confirming ? `Click again to confirm ${label}` : label}
        className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
        style={{
          background: confirming ? 'rgba(232,64,64,0.10)' : 'var(--bg-elevated)',
          border: `1px solid ${confirming ? 'var(--status-error)' : 'var(--border-default)'}`,
          color: disabled || isPending ? 'var(--text-muted)' : confirming ? 'var(--status-error)' : 'var(--text-secondary)',
          borderRadius: '3px',
          cursor: disabled || isPending ? 'not-allowed' : 'pointer',
          opacity: disabled || isPending ? 0.5 : 1,
        }}
      >
        {loading ? <Loader size={12} className="animate-spin" /> : icon}
        {confirming ? `Confirm ${label}` : label}
      </button>
    )
  }

  const errorMsg = error
    ? (error as { body?: { error?: string } }).body?.error === 'proxmox_unreachable'
      ? 'Action failed — Proxmox unreachable'
      : 'Action failed'
    : null

  return (
    <div className="flex flex-col gap-1.5 mt-2 pt-2" style={{ borderTop: '1px solid var(--border-subtle)' }}>
      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Controls</span>
      <div className="flex items-center gap-2 flex-wrap">
        {btn('start', <Play size={12} />, 'Start', isRunning)}
        {btn('shutdown', <PowerOff size={12} />, 'Shutdown', !isRunning)}
        {btn('stop', <Square size={12} />, 'Force Stop', !isRunning)}
        {btn('reboot', <RotateCw size={12} />, 'Reboot', !isRunning)}
      </div>
      {pendingConfirm && (
        <div className="flex items-center gap-2">
          <span className="font-mono text-xs" style={{ color: 'var(--status-warn)' }}>
            Click the highlighted button again to confirm.
          </span>
          <button
            type="button"
            onClick={() => setPendingConfirm(null)}
            className="font-mono text-xs px-2 py-0.5"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-muted)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Cancel
          </button>
        </div>
      )}
      {errorMsg && (
        <span className="font-mono text-xs mt-0.5" style={{ color: 'var(--status-error)' }}>
          {errorMsg}
        </span>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// NodePowerControls — admin-only Proxmox host shutdown / reboot
// ---------------------------------------------------------------------------

interface NodePowerConfirmModal {
  action: NodePowerAction
  nodeName: string
  onConfirm: () => void
  onCancel: () => void
  isPending: boolean
}

function NodePowerConfirmModal({ action, nodeName, onConfirm, onCancel, isPending }: NodePowerConfirmModal) {
  const [typed, setTyped] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const confirmed = typed === nodeName

  const isShutdown = action === 'shutdown'
  const title = isShutdown ? 'Shut down host' : 'Reboot host'
  const warning = isShutdown
    ? 'This will power off the physical Proxmox host. Every VM, LXC container, and service running on it will go offline immediately.'
    : 'This will reboot the physical Proxmox host. Every VM, LXC container, and service running on it will go offline until the host completes its reboot.'

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 9999,
        background: 'rgba(0,0,0,0.6)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <div
        style={{
          background: 'var(--bg-surface)',
          border: '1px solid var(--status-error)',
          borderRadius: '4px',
          padding: '24px',
          maxWidth: '440px',
          width: '100%',
          display: 'flex',
          flexDirection: 'column',
          gap: '16px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <AlertTriangle size={18} style={{ color: 'var(--status-error)', flexShrink: 0 }} />
          <span
            className="font-mono text-sm font-semibold"
            style={{ color: 'var(--status-error)' }}
          >
            {title}
          </span>
        </div>

        <p className="text-xs leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
          {warning}
        </p>

        <div className="flex flex-col gap-1.5">
          <label className="text-xs" style={{ color: 'var(--text-muted)' }}>
            Type the node name <span className="font-mono" style={{ color: 'var(--text-primary)' }}>{nodeName}</span> to confirm:
          </label>
          <input
            ref={inputRef}
            type="text"
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter' && confirmed) onConfirm() }}
            placeholder={nodeName}
            className="font-mono text-xs px-2.5 py-1.5"
            style={{
              background: 'var(--bg-elevated)',
              border: `1px solid ${confirmed ? 'var(--status-error)' : 'var(--border-default)'}`,
              color: 'var(--text-primary)',
              borderRadius: '3px',
              outline: 'none',
            }}
          />
        </div>

        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
          <button
            type="button"
            onClick={onCancel}
            disabled={isPending}
            className="font-mono text-xs px-3 py-1.5"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-muted)',
              borderRadius: '3px',
              cursor: isPending ? 'not-allowed' : 'pointer',
            }}
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={!confirmed || isPending}
            className="font-mono text-xs px-3 py-1.5 flex items-center gap-1.5"
            style={{
              background: confirmed ? 'rgba(232,64,64,0.15)' : 'var(--bg-elevated)',
              border: `1px solid ${confirmed ? 'var(--status-error)' : 'var(--border-subtle)'}`,
              color: confirmed ? 'var(--status-error)' : 'var(--text-muted)',
              borderRadius: '3px',
              cursor: !confirmed || isPending ? 'not-allowed' : 'pointer',
              opacity: !confirmed ? 0.5 : 1,
              transition: 'all 0.15s',
            }}
          >
            {isPending ? <Loader size={12} className="animate-spin" /> : null}
            {title}
          </button>
        </div>
      </div>
    </div>
  )
}

interface NodePowerControlsProps {
  nodeId: string
  nodeName: string
}

function NodePowerControls({ nodeId, nodeName }: NodePowerControlsProps) {
  const { isAdmin } = useCan()
  const { mutate, isPending, isSuccess, error, reset } = useNodePowerAction()
  const [pendingAction, setPendingAction] = useState<NodePowerAction | null>(null)

  // Admin-only: non-admins see nothing.
  if (!isAdmin) return null

  function handleConfirm() {
    if (!pendingAction) return
    mutate(
      { nodeId, action: pendingAction },
      { onSettled: () => setPendingAction(null) },
    )
  }

  const errorMsg = error
    ? (error as { body?: { error?: string } }).body?.error === 'proxmox_unreachable'
      ? 'Action failed — Proxmox unreachable'
      : (error as { body?: { error?: string } }).body?.error === 'proxmox_node_name_unknown'
        ? 'Action failed — could not resolve Proxmox node name'
        : 'Action failed'
    : null

  return (
    <>
      <div className="flex flex-col gap-1.5 mt-2 pt-2" style={{ borderTop: '1px solid var(--border-subtle)' }}>
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Host power (admin)</span>
        <div className="flex items-center gap-2 flex-wrap">
          <button
            type="button"
            disabled={isPending}
            onClick={() => { reset(); setPendingAction('reboot') }}
            className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: isPending ? 'var(--text-muted)' : 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: isPending ? 'not-allowed' : 'pointer',
              opacity: isPending ? 0.5 : 1,
            }}
          >
            <RotateCw size={12} />
            Reboot host
          </button>
          <button
            type="button"
            disabled={isPending}
            onClick={() => { reset(); setPendingAction('shutdown') }}
            className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: isPending ? 'var(--text-muted)' : 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: isPending ? 'not-allowed' : 'pointer',
              opacity: isPending ? 0.5 : 1,
            }}
          >
            <PowerOff size={12} />
            Shut down host
          </button>
        </div>
        {isSuccess && (
          <span className="font-mono text-xs" style={{ color: 'var(--status-ok)' }}>
            Task started — host is responding to the command.
          </span>
        )}
        {errorMsg && (
          <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
            {errorMsg}
          </span>
        )}
      </div>

      {pendingAction && (
        <NodePowerConfirmModal
          action={pendingAction}
          nodeName={nodeName}
          onConfirm={handleConfirm}
          onCancel={() => setPendingAction(null)}
          isPending={isPending}
        />
      )}
    </>
  )
}

function BookmarkButton({ containerId, label }: { containerId: string; label: string }) {
  const { mutate: add, isPending, isSuccess, reset } = useAddBookmark()
  const saved = isSuccess

  return (
    <button
      type="button"
      disabled={isPending}
      onClick={() => {
        add(
          { label, resource_type: 'container', resource_ref: containerId },
          { onSuccess: () => { setTimeout(reset, 2000) } },
        )
      }}
      className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1 self-start mt-1"
      style={{
        background: saved ? 'var(--accent-glow)' : 'var(--bg-elevated)',
        border: `1px solid ${saved ? 'var(--accent)' : 'var(--border-default)'}`,
        color: saved ? 'var(--accent)' : 'var(--text-secondary)',
        borderRadius: '3px',
        cursor: isPending ? 'not-allowed' : 'pointer',
        opacity: isPending ? 0.6 : 1,
        transition: 'color 0.15s, border-color 0.15s, background 0.15s',
      }}
    >
      {saved ? <BookmarkCheck size={12} /> : <BookmarkPlus size={12} />}
      {saved ? 'Saved' : 'Bookmark'}
    </button>
  )
}

function ContainerDetailPane({ nodeId, containerId }: { nodeId: string; containerId: string }) {
  const { data: tree } = useTree()
  const { data: inspect } = useContainerInspect(containerId)
  const { isAdmin } = useCan()
  const [hostPath, setHostPath] = useState('')
  const [submittedPath, setSubmittedPath] = useState('')
  const [showDiagnostic, setShowDiagnostic] = useState(false)

  const node = tree?.nodes.find((n) => n.id === nodeId)
  const c = node?.containers.find((x) => x.id === containerId)

  return (
    <div className="flex flex-col gap-4 flex-1 overflow-auto p-5">
      {/* Container summary */}
      <div
        className="p-4"
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
          maxWidth: '640px',
        }}
      >
        <p
          className="text-xs font-medium uppercase tracking-wider mb-3"
          style={{ color: 'var(--text-muted)' }}
        >
          Docker Container
        </p>
        {c && (
          <div className="flex flex-col gap-2">
            <Row label="Name" value={c.name} mono />
            <Row label="Image" value={c.image} mono />
            <Row label="Status" value={c.status} mono />
            {c.compose_project && <Row label="Compose project" value={c.compose_project} mono />}
            <Row label="Docker ID" value={c.docker_id.slice(0, 12)} mono />
            <LifecycleControls containerId={containerId} status={c.status} />
            <BookmarkButton containerId={containerId} label={c.name} />
          </div>
        )}
        {inspect && (
          <div className="flex flex-col gap-2 mt-2 pt-2" style={{ borderTop: '1px solid var(--border-subtle)' }}>
            <Row
              label="Run user"
              value={inspect.config_user || `${inspect.run_uid}:${inspect.run_gid}`}
              mono
            />
            <Row label="Mounts" value={String(inspect.mounts.length)} mono />
            {inspect.privileged && (
              <div className="flex items-baseline gap-3">
                <span className="text-xs w-28 shrink-0" style={{ color: 'var(--text-muted)' }}>
                  Privileged
                </span>
                <span
                  className="text-xs font-mono px-1.5 py-0.5"
                  style={{
                    background: 'rgba(232,64,64,0.15)',
                    border: '1px solid var(--status-error)',
                    color: 'var(--status-error)',
                    borderRadius: '3px',
                  }}
                >
                  YES — elevated privileges
                </span>
              </div>
            )}
          </div>
        )}
      </div>

      {/* UID/GID Conflict Visualizer */}
      <div style={{ maxWidth: '720px' }}>
        <UidGidVisualizer containerId={containerId} />
      </div>

      {/* Snapshots & Rollback (admin only) */}
      {isAdmin && (
        <SnapshotsPanel
          containerId={containerId}
          containerName={c?.name ?? containerId}
        />
      )}

      {/* SSO Passthrough config (admin only) */}
      {isAdmin && c && (
        <SSOPanel nodeId={nodeId} containerName={c.name} />
      )}

      {/* Reverse-proxy: which public hostname serves this container + add (admin) */}
      {isAdmin && (
        <div style={{ maxWidth: '640px' }}>
          <ContainerProxySection containerId={containerId} />
        </div>
      )}

      {/* Health Check */}
      <HealthCheck containerId={containerId} />

      {/* AI Memory */}
      <MemoryPanel scope="container" scopeId={containerId} />

      {/* Bind mounts */}
      <MountList containerId={containerId} nodeId={nodeId} />

      {/* File permission verdict */}
      <div
        className="flex flex-col gap-2"
        style={{ maxWidth: '640px' }}
      >
        <p
          className="text-xs font-medium uppercase tracking-wider"
          style={{ color: 'var(--text-muted)' }}
        >
          File permission verdict
        </p>
        <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
          Enter a host path to check whether this container can access it.
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            const trimmed = hostPath.trim()
            setSubmittedPath(trimmed)
            setShowDiagnostic(false)
          }}
          className="flex items-center gap-2"
        >
          <input
            type="text"
            placeholder="/var/data/app.conf"
            value={hostPath}
            onChange={(e) => setHostPath(e.target.value)}
            className="font-mono text-xs px-2 py-1.5 flex-1"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-primary)',
              borderRadius: '3px',
              outline: 'none',
              maxWidth: '360px',
            }}
          />
          <button
            type="submit"
            className="text-xs px-3 py-1.5"
            style={{
              background: 'var(--accent-glow)',
              border: '1px solid var(--accent-dim)',
              color: 'var(--accent)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Analyze
          </button>
        </form>
        {submittedPath && (
          <FileUidPanel containerId={containerId} hostPath={submittedPath} />
        )}
        {submittedPath && (
          <div className="flex flex-col gap-2">
            <button
              type="button"
              onClick={() => setShowDiagnostic((v) => !v)}
              className="text-xs px-3 py-1.5 self-start"
              style={{
                background: showDiagnostic ? 'rgba(232,64,64,0.10)' : 'var(--bg-elevated)',
                border: `1px solid ${showDiagnostic ? 'var(--status-error)' : 'var(--border-default)'}`,
                color: showDiagnostic ? 'var(--status-error)' : 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              {showDiagnostic ? 'Hide diagnostic' : 'Why is this broken?'}
            </button>
            {showDiagnostic && (
              <DiagnosticCard containerId={containerId} hostPath={submittedPath} />
            )}
          </div>
        )}
      </div>
    </div>
  )
}

/**
 * The set of host-level panels shown for a managed Linux host. Rendered for a
 * `node` selection and also for a Proxmox guest that is linked to a standalone
 * Docker node — in that case `nodeId` is the LINKED node's id so every panel
 * (SSH keys, cron/timers, reverse-proxy, DNS, file-watch) queries the real
 * reachable host rather than the unreachable guest.
 *
 * `inCard` renders the panels that belong inside the summary card (alongside
 * the host/VM fields); `below` renders the wider panels shown beneath the card.
 */
function HostPanelsInCard({ nodeId }: { nodeId: string }) {
  return (
    <>
      <WakeOnLan nodeId={nodeId} />
      <SSHKeys nodeId={nodeId} />
      <Scheduler nodeId={nodeId} />
      <ReverseProxyPanel nodeId={nodeId} />
      <DnsPanel nodeId={nodeId} />
      <FileWatchPanel nodeId={nodeId} />
    </>
  )
}

function HostPanelsBelow({ nodeId }: { nodeId: string }) {
  return (
    <>
      <SharedMountsView nodeId={nodeId} />
      <ReverseMountPanel nodeId={nodeId} />
      <MemoryPanel scope="node" scopeId={nodeId} />
    </>
  )
}

function DetailPane() {
  const { selection } = useTreeStore()
  const { data } = useTree()
  // Reuse the same guest↔node correlation as the resource tree so a selected
  // Proxmox guest can resolve to its linked standalone host.
  const correlation = useNodeGuestLinks(data?.nodes ?? [])

  if (!selection) {
    return (
      <div
        className="flex-1 flex items-center justify-center"
        style={{ color: 'var(--text-muted)' }}
      >
        <span className="text-xs">Select a resource to inspect.</span>
      </div>
    )
  }

  // Filesystem browser takes the full pane. A container fs-root uses the
  // read-only container browser (docker exec/archive); a host fs-root uses the
  // full SFTP-backed browser.
  if (selection.kind === 'fs-root') {
    return (
      <div className="flex flex-col flex-1 min-h-0 overflow-hidden">
        {selection.containerId ? (
          <ContainerFileBrowser
            key={`ctr:${selection.containerId}`}
            containerId={selection.containerId}
          />
        ) : (
          <FileBrowser key={`node:${selection.nodeId}`} nodeId={selection.nodeId} />
        )}
      </div>
    )
  }

  // Container detail: full UID/GID visualizer pane
  if (selection.kind === 'container') {
    return (
      <ContainerDetailPane
        nodeId={selection.nodeId}
        containerId={selection.containerId}
      />
    )
  }

  const node = data?.nodes.find((n) => n.id === selection.nodeId)

  // For a VM selection, resolve the selected guest and the standalone Docker
  // node (if any) linked to it. When linked, the guest borrows that node's full
  // host detail; the panels then target `linkedNodeId` so they reach the real
  // host instead of the unreachable guest.
  const vm =
    selection.kind === 'vm' ? node?.vms.find((v) => v.id === selection.vmId) : undefined
  const linkedNode = vm ? correlation.linkedNodeByVmid.get(vm.proxmox_vmid) : undefined
  const linkedNodeId = linkedNode?.id

  // The id whose host panels we render: the node itself, or a linked guest's host.
  const hostNodeId =
    selection.kind === 'node'
      ? selection.nodeId
      : linkedNodeId

  return (
    <div className="flex-1 overflow-auto p-5 flex flex-col gap-4">
      <div
        className="p-4"
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
          maxWidth: '640px',
        }}
      >
        <p
          className="text-xs font-medium uppercase tracking-wider mb-3"
          style={{ color: 'var(--text-muted)' }}
        >
          {selectionTitle(selection)}
        </p>

        {selection.kind === 'node' && node && (
          <div className="flex flex-col gap-2">
            <Row label="Name" value={node.name} mono />
            <Row label="Host" value={node.host} mono />
            <Row label="Type" value={node.type} mono />
            <Row label="Status" value={node.status} mono />
            <Row label="OS type" value={node.status} mono />
            <HostPanelsInCard nodeId={selection.nodeId} />
            {node.type === 'proxmox' && (
              <NodePowerControls nodeId={node.id} nodeName={node.name} />
            )}
          </div>
        )}

        {selection.kind === 'vm' && !vm && (
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Not found.</span>
        )}

        {selection.kind === 'vm' && vm && (
          <div className="flex flex-col gap-2">
            {/* Guest summary — shown whether or not a host is linked, so it's
                always clear which guest is selected. */}
            <Row label="Name" value={vm.name} mono />
            <Row label="Kind" value={vm.kind.toUpperCase()} mono />
            <Row label="VMID" value={String(vm.proxmox_vmid)} mono />
            <Row label="Node" value={vm.proxmox_node} mono />
            <Row label="Status" value={vm.status} mono />
            {vm.os_type && <Row label="OS type" value={vm.os_type} mono />}
            <VMLifecycleControls
              nodeId={selection.nodeId}
              vmid={vm.proxmox_vmid}
              status={vm.status}
            />
            {linkedNode ? (
              <>
                <div
                  className="flex items-baseline gap-3 mt-1 pt-2"
                  style={{ borderTop: '1px solid var(--border-subtle)' }}
                >
                  <span className="text-xs w-28 shrink-0" style={{ color: 'var(--text-muted)' }}>
                    Linked host
                  </span>
                  <span className="text-xs font-mono truncate" style={{ color: 'var(--accent)' }} title={linkedNode.host}>
                    {linkedNode.name}
                  </span>
                </div>
                {/* Full host detail, sourced from the linked standalone node. */}
                <HostPanelsInCard nodeId={linkedNode.id} />
              </>
            ) : (
              <p className="text-xs mt-1 pt-2" style={{ color: 'var(--text-muted)', borderTop: '1px solid var(--border-subtle)' }}>
                No managed host is linked to this guest, so no host-level detail
                is available. Link a standalone Docker node to this guest to manage it here.
              </p>
            )}
          </div>
        )}
      </div>

      {/* Wider host panels — for a node, or a linked guest's host. */}
      {hostNodeId && <HostPanelsBelow nodeId={hostNodeId} />}
    </div>
  )
}

/**
 * Reads `?node=<id>` and optional `?container=<id>` query params and, once the
 * tree data has loaded, selects the target item and expands its ancestors so it
 * is visible. Runs at most once per unique target (tracked by a ref so a
 * subsequent navigation to /resources without params doesn't re-fire).
 *
 * Supports a lone `?container=<id>` (no `?node=`) — scans all nodes in the tree
 * to find which one owns the container, then selects and expands it. This allows
 * bookmark deep-links built with resourceLink(undefined, containerId) to resolve
 * correctly without knowing the node id in advance.
 */
function useResourceDeepLink() {
  const [params, setParams] = useSearchParams()
  const { data } = useTree()
  const { setSelected, setExpanded } = useTreeStore()
  const appliedRef = useRef<string | null>(null)

  const nodeIdParam = params.get('node')
  const containerId = params.get('container')

  // A target key that identifies this unique deep-link request.
  const targetKey = nodeIdParam
    ? `${nodeIdParam}:${containerId ?? ''}`
    : containerId
      ? `lone-container:${containerId}`
      : null

  useEffect(() => {
    if (!targetKey || !data || appliedRef.current === targetKey) return

    // Determine the effective node. When only a containerId is provided, scan
    // all nodes to find the one that owns this container.
    let nodeId = nodeIdParam
    if (!nodeId && containerId) {
      const ownerNode = data.nodes.find((n) =>
        n.containers.some((c) => c.id === containerId),
      )
      if (!ownerNode) return
      nodeId = ownerNode.id
    }

    if (!nodeId) return

    const node = data.nodes.find((n) => n.id === nodeId)
    if (!node) return

    // Always expand the node row.
    setExpanded(`node:${nodeId}`, true)

    if (containerId) {
      const container = node.containers.find((c) => c.id === containerId)
      if (!container) return

      // Expand the compose-stack group that contains this container.
      const UNGROUPED = ' ungrouped'
      const projectKey = container.compose_project && container.compose_project.length > 0
        ? container.compose_project
        : UNGROUPED
      setExpanded(`project:${nodeId}:${projectKey}`, true)

      setSelected({ kind: 'container', nodeId, containerId })
    } else {
      setSelected({ kind: 'node', nodeId })
    }

    appliedRef.current = targetKey

    // Strip the query params from the URL so refreshing doesn't re-apply.
    setParams({}, { replace: true })
  }, [nodeIdParam, containerId, data, targetKey, setSelected, setExpanded, setParams])
}

export default function Resources() {
  useResourceDeepLink()
  return (
    <AppShell treeSlot={<ResourceTree />}>
      <DetailPane />
    </AppShell>
  )
}
