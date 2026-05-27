import { Server, Monitor, Box, Terminal, Folder } from 'lucide-react'
import type { NodeType, VMKind } from '../../types/api'

interface IconProps {
  size?: number
  color?: string
}

export function NodeTypeIcon({ type, ...props }: IconProps & { type: NodeType }) {
  const size = props.size ?? 13
  if (type === 'proxmox') {
    return <Server size={size} strokeWidth={1.5} style={{ color: props.color ?? 'var(--accent)' }} />
  }
  if (type === 'standalone') {
    return <Server size={size} strokeWidth={1.5} style={{ color: props.color ?? 'var(--status-ok)' }} />
  }
  return <Terminal size={size} strokeWidth={1.5} style={{ color: props.color ?? 'var(--text-secondary)' }} />
}

export function VMIcon({ kind, ...props }: IconProps & { kind: VMKind }) {
  const size = props.size ?? 13
  if (kind === 'lxc') {
    return <Box size={size} strokeWidth={1.5} style={{ color: props.color ?? 'var(--status-info)' }} />
  }
  return <Monitor size={size} strokeWidth={1.5} style={{ color: props.color ?? 'var(--status-info)' }} />
}

export function ContainerIcon({ ...props }: IconProps) {
  const size = props.size ?? 13
  return <Box size={size} strokeWidth={1.5} style={{ color: props.color ?? 'var(--text-secondary)' }} />
}

export function FolderIcon({ ...props }: IconProps) {
  const size = props.size ?? 13
  return <Folder size={size} strokeWidth={1.5} style={{ color: props.color ?? 'var(--text-muted)' }} />
}
