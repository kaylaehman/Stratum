import { AppShell } from '../components/layout/AppShell'
import { ResourceTree } from '../components/tree/ResourceTree'
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

        {selection.kind === 'container' && node && (() => {
          const c = node.containers.find((x) => x.id === selection.containerId)
          if (!c) return <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Not found.</span>
          return (
            <div className="flex flex-col gap-2">
              <Row label="Name" value={c.name} mono />
              <Row label="Image" value={c.image} mono />
              <Row label="Status" value={c.status} mono />
              {c.compose_project && <Row label="Compose project" value={c.compose_project} mono />}
              <Row label="Docker ID" value={c.docker_id.slice(0, 12)} mono />
            </div>
          )
        })()}

        {selection.kind === 'fs-root' && (
          <div className="flex flex-col gap-2">
            <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
              Filesystem browser is coming in a future sub-project. Select a path in the tree to
              navigate.
            </p>
            {node && <Row label="Host" value={node.host} mono />}
            {selection.containerId && (
              <Row label="Container ID" value={selection.containerId.slice(0, 12)} mono />
            )}
          </div>
        )}
      </div>
    </div>
  )
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

export default function Resources() {
  return (
    <AppShell treeSlot={<ResourceTree />}>
      <DetailPane />
    </AppShell>
  )
}
