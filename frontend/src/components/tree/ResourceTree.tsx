import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { Loader, Plus, ShieldAlert } from 'lucide-react'
import { useTree, useTreeLiveUpdates } from '../../lib/api/tree'
import { useSecurityBadges } from '../../lib/api/security'
import { useTreeStore } from '../../store/tree'
import { useMe } from '../../hooks/useMe'
import { TreeNodeRow } from './TreeNodeRow'
import { NodeTypeIcon, VMIcon, ContainerIcon, FolderIcon } from './icons'
import type { TreeNode, VM, Container } from '../../types/api'

// ---- VM rows ----

interface VMRowsProps {
  node: TreeNode
  vms: VM[]
}

function VMRows({ node, vms }: VMRowsProps) {
  const { expanded, toggleExpanded, selection, setSelected } = useTreeStore()

  return (
    <>
      {vms.map((vm) => {
        const vmKey = `vm:${vm.id}`
        const vmExpanded = expanded.has(vmKey)
        const vmSelected = selection?.kind === 'vm' && selection.vmId === vm.id

        return (
          <div key={vm.id}>
            <TreeNodeRow
              depth={2}
              icon={<VMIcon kind={vm.kind} />}
              label={vm.name}
              sublabel={`${vm.kind.toUpperCase()} ${vm.proxmox_vmid}`}
              status={vm.status}
              stale={vm.stale}
              expandable
              expanded={vmExpanded}
              selected={vmSelected}
              onToggle={() => toggleExpanded(vmKey)}
              onClick={() => setSelected({ kind: 'vm', nodeId: node.id, vmId: vm.id, vmKind: vm.kind })}
            />
            {vmExpanded && (
              <TreeNodeRow
                depth={3}
                icon={<FolderIcon />}
                label="Filesystem"
                expandable={false}
                selected={selection?.kind === 'fs-root' && (selection as { vmId?: string }).vmId === vm.id}
                onClick={() => setSelected({ kind: 'fs-root', nodeId: node.id })}
              />
            )}
          </div>
        )
      })}
    </>
  )
}

// ---- Container rows ----

interface ContainerRowsProps {
  node: TreeNode
  containers: Container[]
  badges: Record<string, boolean>
}

function ContainerRows({ node, containers, badges }: ContainerRowsProps) {
  const { expanded, toggleExpanded, selection, setSelected } = useTreeStore()

  return (
    <>
      {containers.map((c) => {
        const cKey = `container:${c.id}`
        const cExpanded = expanded.has(cKey)
        const cSelected = selection?.kind === 'container' && selection.containerId === c.id
        const hasBadge = badges[c.id] === true

        return (
          <div key={c.id}>
            <TreeNodeRow
              depth={2}
              icon={<ContainerIcon />}
              label={c.name}
              sublabel={c.compose_project ?? c.image.split(':')[0]}
              status={c.status}
              stale={c.stale}
              expandable
              expanded={cExpanded}
              selected={cSelected}
              badge={
                hasBadge ? (
                  <span title="Unacknowledged security flags" style={{ display: 'flex' }}>
                    <ShieldAlert size={11} style={{ color: 'var(--status-error)' }} />
                  </span>
                ) : undefined
              }
              onToggle={() => toggleExpanded(cKey)}
              onClick={() => setSelected({ kind: 'container', nodeId: node.id, containerId: c.id })}
            />
            {cExpanded && (
              <TreeNodeRow
                depth={3}
                icon={<FolderIcon />}
                label="Filesystem"
                expandable={false}
                selected={
                  selection?.kind === 'fs-root' &&
                  selection.nodeId === node.id &&
                  selection.containerId === c.id
                }
                onClick={() =>
                  setSelected({ kind: 'fs-root', nodeId: node.id, containerId: c.id })
                }
              />
            )}
          </div>
        )
      })}
    </>
  )
}

// ---- Single node subtree ----

interface NodeSubtreeProps {
  node: TreeNode
  badges: Record<string, boolean>
}

function NodeSubtree({ node, badges }: NodeSubtreeProps) {
  const { expanded, toggleExpanded, selection, setSelected } = useTreeStore()
  const nodeKey = `node:${node.id}`
  const nodeExpanded = expanded.has(nodeKey)
  const nodeSelected = selection?.kind === 'node' && selection.nodeId === node.id

  const hasGuests = node.capabilities.proxmox && node.vms.length > 0
  const hasContainers = node.capabilities.docker && node.containers.length > 0
  const hasChildren = hasGuests || hasContainers || true // always show fs-root

  return (
    <div>
      {/* Node row */}
      <TreeNodeRow
        depth={1}
        icon={<NodeTypeIcon type={node.type} />}
        label={node.name}
        sublabel={node.host}
        status={node.status}
        expandable={hasChildren}
        expanded={nodeExpanded}
        selected={nodeSelected}
        onToggle={() => toggleExpanded(nodeKey)}
        onClick={() => setSelected({ kind: 'node', nodeId: node.id })}
      />

      {/* Children */}
      {nodeExpanded && (
        <>
          {/* VMs / LXCs — only for proxmox capability */}
          {node.capabilities.proxmox && node.vms.length > 0 && (
            <VMRows node={node} vms={node.vms} />
          )}

          {/* Containers — only for docker capability */}
          {node.capabilities.docker && node.containers.length > 0 && (
            <ContainerRows node={node} containers={node.containers} badges={badges} />
          )}

          {/* Host filesystem entry point */}
          <TreeNodeRow
            depth={2}
            icon={<FolderIcon />}
            label="Filesystem"
            expandable={false}
            selected={
              selection?.kind === 'fs-root' &&
              selection.nodeId === node.id &&
              !selection.containerId
            }
            onClick={() => setSelected({ kind: 'fs-root', nodeId: node.id })}
          />
        </>
      )}
    </div>
  )
}

// ---- Root component ----

export function ResourceTree() {
  useTreeLiveUpdates()

  const { data, isLoading, isError } = useTree()
  const { nodes: liveNodes, expanded, toggleExpanded } = useTreeStore()

  const { data: meData } = useMe()
  const isAdmin = meData?.role === 'admin'
  const { data: badgesData } = useSecurityBadges(isAdmin)
  const badges: Record<string, boolean> = badgesData?.badges ?? {}

  // Auto-expand when nodes first load
  useEffect(() => {
    if (!data) return
    for (const node of data.nodes) {
      const key = `node:${node.id}`
      if (!expanded.has(key)) {
        toggleExpanded(key)
      }
    }
  // Run only once when data first arrives
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data])

  // Prefer live (Zustand) nodes once we have them; fall back to React Query data
  const displayNodes = liveNodes.length > 0 ? liveNodes : (data?.nodes ?? [])

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 px-4 py-4">
        <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          Loading...
        </span>
      </div>
    )
  }

  if (isError) {
    return (
      <div className="px-3 py-2">
        <span className="text-xs" style={{ color: 'var(--status-error)' }}>
          Failed to load tree.
        </span>
      </div>
    )
  }

  return (
    <div
      role="tree"
      aria-label="Connected Hosts"
      style={{ overflowY: 'auto', overflowX: 'hidden' }}
    >
      {/* Section header */}
      <div
        className="px-3 pt-3 pb-1.5 flex items-center justify-between"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <span
          className="text-xs font-medium uppercase tracking-wider"
          style={{ color: 'var(--text-muted)' }}
        >
          Connected Hosts
        </span>
      </div>

      {displayNodes.length === 0 ? (
        <div className="px-4 py-5 flex flex-col items-center gap-3 text-center">
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
            No connected hosts.
          </span>
          <Link
            to="/nodes"
            className="flex items-center gap-1 text-xs px-2 py-1"
            style={{
              backgroundColor: 'var(--accent-glow)',
              border: '1px solid var(--accent-dim)',
              color: 'var(--accent)',
              borderRadius: '3px',
              textDecoration: 'none',
            }}
          >
            <Plus size={11} />
            Add a host
          </Link>
        </div>
      ) : (
        <div>
          {displayNodes.map((node) => (
            <NodeSubtree key={node.id} node={node} badges={badges} />
          ))}
        </div>
      )}
    </div>
  )
}
