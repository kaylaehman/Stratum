import { Server, Box, Terminal, Check, X, AlertTriangle } from 'lucide-react'
import type { PreviewResult, NodeType } from '../../types/api'

const NODE_ICONS: Record<NodeType, React.ReactNode> = {
  proxmox: <Server size={14} style={{ color: 'var(--accent)' }} />,
  standalone: <Box size={14} style={{ color: 'var(--status-ok)' }} />,
  ssh: <Terminal size={14} style={{ color: 'var(--text-secondary)' }} />,
}

const NODE_LABELS: Record<NodeType, string> = {
  proxmox: 'Proxmox VE',
  standalone: 'Standalone (Docker)',
  ssh: 'SSH Host',
}

const CAP_LABELS: Record<string, string> = {
  proxmox: 'Proxmox',
  docker: 'Docker',
  agent: 'Agent',
  systemd: 'systemd',
  cron: 'cron',
}

function proxmoxStatusMessage(status: PreviewResult['proxmox_auth_status']): {
  text: string
  color: string
} {
  if (status === 'confirmed') {
    return { text: 'API reachable and token confirmed — VM enumeration active', color: 'var(--status-ok)' }
  }
  if (status === 'unauthed') {
    return {
      text: 'API reachable but token invalid — VM enumeration disabled until fixed',
      color: 'var(--status-warn)',
    }
  }
  if (status === 'marker_only') {
    return {
      text: 'Detected via /etc/pve — add a token to enable VM enumeration',
      color: 'var(--status-warn)',
    }
  }
  return { text: 'No Proxmox detected', color: 'var(--text-muted)' }
}

interface CapChipProps {
  label: string
  active: boolean
}

function CapChip({ label, active }: CapChipProps) {
  return (
    <span
      className="inline-flex items-center gap-1 px-1.5 py-0.5 text-xs font-mono"
      style={{
        backgroundColor: active ? 'var(--accent-glow)' : 'var(--bg-elevated)',
        border: `1px solid ${active ? 'var(--accent-dim)' : 'var(--border-subtle)'}`,
        color: active ? 'var(--accent)' : 'var(--text-muted)',
        borderRadius: '3px',
      }}
    >
      {active
        ? <Check size={10} />
        : <X size={10} />
      }
      {label}
    </span>
  )
}

interface ProbeResultProps {
  result: PreviewResult
  acceptedKey: boolean
  onAcceptKey: () => void
  typeOverride: NodeType | ''
  onTypeOverride: (t: NodeType | '') => void
}

export function ProbeResult({
  result,
  acceptedKey,
  onAcceptKey,
  typeOverride,
  onTypeOverride,
}: ProbeResultProps) {
  const displayType = (typeOverride || result.type) as NodeType
  const pxStatus = proxmoxStatusMessage(result.proxmox_auth_status)

  return (
    <div className="flex flex-col gap-4">
      {/* Detected type */}
      <div
        className="flex flex-col gap-2 p-3"
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
        }}
      >
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {NODE_ICONS[displayType]}
            <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
              {NODE_LABELS[displayType]}
            </span>
            <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
              {result.os_type}
            </span>
          </div>

          {/* Type override */}
          <div className="flex items-center gap-1.5">
            <label className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Override:
            </label>
            <select
              value={typeOverride}
              onChange={(e) => onTypeOverride(e.target.value as NodeType | '')}
              className="text-xs font-mono px-1.5 py-0.5 outline-none"
              style={{
                backgroundColor: 'var(--bg-overlay)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-primary)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <option value="">auto ({result.type})</option>
              <option value="proxmox">proxmox</option>
              <option value="standalone">standalone</option>
              <option value="ssh">ssh</option>
            </select>
          </div>
        </div>

        {/* Capability chips */}
        <div className="flex flex-wrap gap-1.5">
          {(Object.keys(CAP_LABELS) as Array<keyof typeof CAP_LABELS>).map((cap) => (
            <CapChip
              key={cap}
              label={CAP_LABELS[cap]}
              active={Boolean(result.capabilities[cap as keyof typeof result.capabilities])}
            />
          ))}
        </div>

        {/* Versions */}
        {(result.docker_version || result.proxmox_version) && (
          <div className="flex gap-3">
            {result.docker_version && (
              <span className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                Docker {result.docker_version}
              </span>
            )}
            {result.proxmox_version && (
              <span className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                PVE {result.proxmox_version}
              </span>
            )}
          </div>
        )}
      </div>

      {/* Proxmox auth status (only when relevant) */}
      {result.proxmox_auth_status !== 'none' && (
        <div
          className="flex items-start gap-2 px-3 py-2"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
          }}
        >
          <AlertTriangle size={12} style={{ color: pxStatus.color, marginTop: '1px', flexShrink: 0 }} />
          <span className="text-xs" style={{ color: pxStatus.color }}>
            {pxStatus.text}
          </span>
        </div>
      )}

      {/* Probe errors */}
      {result.probe_errors && Object.keys(result.probe_errors).length > 0 && (
        <div
          className="flex flex-col gap-1 px-3 py-2"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
          }}
        >
          <span className="text-xs font-medium" style={{ color: 'var(--status-warn)' }}>
            Probe warnings
          </span>
          {Object.entries(result.probe_errors).map(([probe, msg]) => (
            <div key={probe} className="flex gap-2">
              <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
                {probe}:
              </span>
              <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                {msg}
              </span>
            </div>
          ))}
        </div>
      )}

      {/* SSH host key */}
      <div
        className="flex flex-col gap-2 p-3"
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: `1px solid ${acceptedKey ? 'var(--accent-dim)' : 'var(--border-default)'}`,
          borderRadius: '3px',
        }}
      >
        <p className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
          SSH host key fingerprint
        </p>
        <p
          className="font-mono text-xs break-all"
          style={{ color: 'var(--text-secondary)' }}
        >
          {result.ssh_host_key_sha256}
        </p>
        {!acceptedKey ? (
          <button
            type="button"
            onClick={onAcceptKey}
            className="self-start flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium"
            style={{
              backgroundColor: 'var(--accent)',
              color: 'var(--text-inverse)',
              border: 'none',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            <Check size={12} />
            Accept this host key
          </button>
        ) : (
          <div className="flex items-center gap-1.5">
            <Check size={12} style={{ color: 'var(--status-ok)' }} />
            <span className="text-xs" style={{ color: 'var(--status-ok)' }}>
              Host key accepted
            </span>
          </div>
        )}
      </div>
    </div>
  )
}
