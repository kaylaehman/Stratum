import { Link } from 'react-router-dom'
import { HardDrive, Terminal, Server, Box, Container as ContainerIcon, Boxes } from 'lucide-react'
import type { TreeNode, VM, Container, NodeType } from '../../types/api'
import type { Correlation } from '../../hooks/useNodeGuestLinks'

// Status → dot color. Covers node, VM (proxmox), and container status strings.
function statusColor(status: string): string {
  switch (status) {
    case 'ok':
    case 'running':
      return 'var(--status-ok)'
    case 'error':
    case 'unreachable':
    case 'dead':
      return 'var(--status-error)'
    case 'paused':
    case 'restarting':
      return 'var(--status-warn)'
    default:
      return 'var(--text-muted)' // stopped/exited/created/unknown
  }
}

function Dot({ status, size = 8 }: { status: string; size?: number }) {
  return (
    <span
      title={status}
      style={{ width: size, height: size, borderRadius: '50%', backgroundColor: statusColor(status), flexShrink: 0 }}
    />
  )
}

function HostIcon({ type }: { type: NodeType }) {
  const color = 'var(--text-secondary)'
  if (type === 'proxmox') return <HardDrive size={14} style={{ color }} />
  if (type === 'ssh') return <Terminal size={14} style={{ color }} />
  return <Boxes size={14} style={{ color }} /> // standalone Docker host
}

function GuestIcon({ kind }: { kind: VM['kind'] }) {
  // qemu = full VM, lxc = container-based guest
  return kind === 'lxc' ? <Box size={13} style={{ color: '#bf8a4a' }} /> : <Server size={13} style={{ color: '#4a8abf' }} />
}

function Badge({ children, color }: { children: React.ReactNode; color?: string }) {
  return (
    <span
      className="font-mono uppercase"
      style={{
        fontSize: '11px',
        letterSpacing: '0.05em',
        color: color ?? 'var(--text-muted)',
        background: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        padding: '0 4px',
      }}
    >
      {children}
    </span>
  )
}

function ContainerRow({ c }: { c: Container }) {
  return (
    <Link
      to="/resources"
      className="flex items-center gap-2 px-2 py-1"
      style={{ textDecoration: 'none' }}
    >
      <ContainerIcon size={12} style={{ color: 'var(--accent)', flexShrink: 0 }} />
      <Dot status={c.status} size={6} />
      <span className="font-mono text-xs truncate" style={{ color: 'var(--text-primary)' }}>
        {c.name}
      </span>
      <span className="font-mono text-xs truncate" style={{ color: 'var(--text-muted)' }}>
        {c.image}
      </span>
      {c.compose_project && (
        <span className="font-mono ml-auto shrink-0" style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
          {c.compose_project}
        </span>
      )}
    </Link>
  )
}

// Indented child group with a left connector line conveying hierarchy.
function Branch({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ marginLeft: '14px', paddingLeft: '10px', borderLeft: '1px solid var(--border-subtle)' }}>
      {children}
    </div>
  )
}

interface InfraMapProps {
  nodes: TreeNode[]
  correlation: Correlation
}

export function InfraMap({ nodes, correlation }: InfraMapProps) {
  const { linkedNodeByVmid, hiddenNodeIds } = correlation
  // Top-level hosts: standalone nodes that nest under a Proxmox guest are hidden
  // there and instead rendered under that guest.
  const topNodes = nodes.filter((n) => !hiddenNodeIds.has(n.id))

  return (
    <div className="flex flex-col gap-3">
      {topNodes.map((host) => {
        const vms = [...host.vms].sort((a, b) => a.proxmox_vmid - b.proxmox_vmid)
        const directContainers = [...host.containers].sort((a, b) => a.name.localeCompare(b.name))
        const empty = vms.length === 0 && directContainers.length === 0

        return (
          <div
            key={host.id}
            style={{
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
              overflow: 'hidden',
            }}
          >
            {/* Host header */}
            <div
              className="flex items-center gap-2 px-3 py-2"
              style={{ borderBottom: empty ? 'none' : '1px solid var(--border-subtle)', backgroundColor: 'var(--bg-elevated)' }}
            >
              <HostIcon type={host.type} />
              <Dot status={host.status} />
              <span className="font-medium text-xs" style={{ color: 'var(--text-primary)' }}>
                {host.name}
              </span>
              <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
                {host.host}
              </span>
              <span className="ml-auto">
                <Badge>{host.type}</Badge>
              </span>
            </div>

            {!empty && (
              <div className="px-3 py-2">
                <Branch>
                  {/* Proxmox guests (VMs / LXCs) */}
                  {vms.map((vm) => {
                    const linked = linkedNodeByVmid.get(vm.proxmox_vmid)
                    const guestContainers = linked
                      ? [...linked.containers].sort((a, b) => a.name.localeCompare(b.name))
                      : []
                    return (
                      <div key={vm.id} className="mb-1">
                        <div className="flex items-center gap-2 px-1 py-0.5">
                          <GuestIcon kind={vm.kind} />
                          <Dot status={vm.status} size={6} />
                          <span className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
                            {vm.name}
                          </span>
                          <Badge color={vm.kind === 'lxc' ? '#bf8a4a' : '#4a8abf'}>
                            {vm.kind === 'lxc' ? 'LXC' : 'VM'} {vm.proxmox_vmid}
                          </Badge>
                          {linked && (
                            <span className="font-mono ml-auto" style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                              {guestContainers.length} container{guestContainers.length === 1 ? '' : 's'}
                            </span>
                          )}
                        </div>
                        {guestContainers.length > 0 && (
                          <Branch>
                            {guestContainers.map((c) => (
                              <ContainerRow key={c.id} c={c} />
                            ))}
                          </Branch>
                        )}
                      </div>
                    )
                  })}

                  {/* Containers attached directly to this host (standalone/ssh). */}
                  {directContainers.map((c) => (
                    <ContainerRow key={c.id} c={c} />
                  ))}
                </Branch>
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

// Aggregate counts for the page header.
export function infraStats(nodes: TreeNode[], correlation: Correlation) {
  const topNodes = nodes.filter((n) => !correlation.hiddenNodeIds.has(n.id))
  let vms = 0
  let lxcs = 0
  let containers = 0
  for (const n of nodes) {
    for (const vm of n.vms) {
      if (vm.kind === 'lxc') lxcs++
      else vms++
    }
    containers += n.containers.length
  }
  return { hosts: topNodes.length, vms, lxcs, containers }
}
