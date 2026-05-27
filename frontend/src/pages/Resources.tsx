import { useState } from 'react'
import { AppShell } from '../components/layout/AppShell'
import { ResourceTree } from '../components/tree/ResourceTree'
import { FileBrowser } from '../components/filesystem/FileBrowser'
import { UidGidVisualizer } from '../components/permissions/UidGidVisualizer'
import { FileUidPanel } from '../components/permissions/FileUidPanel'
import { DiagnosticCard } from '../components/permissions/DiagnosticCard'
import { useContainerInspect } from '../lib/api/permissions'
import { useTreeStore } from '../store/tree'
import { useTree } from '../lib/api/tree'
import type { TreeSelection } from '../types/api'

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

function ContainerDetailPane({ nodeId, containerId }: { nodeId: string; containerId: string }) {
  const { data: tree } = useTree()
  const { data: inspect } = useContainerInspect(containerId)
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

function DetailPane() {
  const { selection } = useTreeStore()
  const { data } = useTree()

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

  // Filesystem browser takes the full pane
  if (selection.kind === 'fs-root') {
    return (
      <div className="flex flex-col flex-1 min-h-0 overflow-hidden">
        <FileBrowser
          nodeId={selection.nodeId}
          containerId={selection.containerId}
        />
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

  return (
    <div className="flex-1 overflow-auto p-5">
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
          </div>
        )}

        {selection.kind === 'vm' && node && (() => {
          const vm = node.vms.find((v) => v.id === selection.vmId)
          if (!vm) return <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Not found.</span>
          return (
            <div className="flex flex-col gap-2">
              <Row label="Name" value={vm.name} mono />
              <Row label="Kind" value={vm.kind.toUpperCase()} mono />
              <Row label="VMID" value={String(vm.proxmox_vmid)} mono />
              <Row label="Node" value={vm.proxmox_node} mono />
              <Row label="Status" value={vm.status} mono />
              {vm.os_type && <Row label="OS type" value={vm.os_type} mono />}
            </div>
          )
        })()}
      </div>
    </div>
  )
}

export default function Resources() {
  return (
    <AppShell treeSlot={<ResourceTree />}>
      <DetailPane />
    </AppShell>
  )
}
