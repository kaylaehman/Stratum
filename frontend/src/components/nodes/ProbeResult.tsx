import { useState } from 'react'
import { Server, Box, Terminal, Check, X, AlertTriangle, ChevronDown, ChevronRight } from 'lucide-react'
import type { PreviewResult, NodeType } from '../../types/api'

// SSH-auth subcategories the backend may emit. The wizard treats any of these
// (plus the generic ssh_auth_failed fallback) as a blocking failure — a node
// that can't authenticate is useless to persist.
const SSH_AUTH_CATEGORIES = new Set([
  'ssh_auth_failed',
  'ssh_auth_pubkey_rejected',
  'ssh_auth_password_rejected',
  'ssh_passphrase_required',
  'ssh_passphrase_wrong',
])

export function isBlockingProbeError(category: string | undefined): boolean {
  if (!category) return false
  return SSH_AUTH_CATEGORIES.has(category) || category === 'ssh_host_key_mismatch'
}

// Common causes shown in the "What to check" expander when SSH auth fails.
// Ordered by how often each is the actual culprit, based on the typical
// homelab/cloud-VM setup pattern.
const SSH_AUTH_CHECKLIST = [
  'The matching public key is in ~/.ssh/authorized_keys for the user you supplied (not root unless you set "root" above).',
  'The target permits the credential type you chose — PubkeyAuthentication / PasswordAuthentication in sshd_config, and PermitRootLogin if your user is root.',
  '~/.ssh is mode 700 and ~/.ssh/authorized_keys is mode 600 — sshd silently ignores the file if either is world-accessible.',
  'You pasted the private key (not the .pub) and copied the full PEM block including BEGIN/END lines.',
  'If the key has a passphrase, it is filled into the Key passphrase field above.',
]

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
  const sshCategory = result.probe_errors?.ssh
  const sshHint = result.probe_hints?.ssh
  const sshAuthFailed = sshCategory ? SSH_AUTH_CATEGORIES.has(sshCategory) : false
  const [checklistOpen, setChecklistOpen] = useState(false)

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

      {/* SSH auth hint — prominent, actionable, with a collapsible checklist.
          Shown for any ssh_auth_* category. Rendered above the generic probe
          warnings block so it's the first thing the user sees. */}
      {sshAuthFailed && (
        <div
          className="flex flex-col gap-2 p-3"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--status-warn)',
            borderRadius: '3px',
          }}
        >
          <div className="flex items-start gap-2">
            <AlertTriangle
              size={14}
              style={{ color: 'var(--status-warn)', marginTop: '1px', flexShrink: 0 }}
            />
            <div className="flex flex-col gap-1">
              <p className="text-xs font-medium" style={{ color: 'var(--status-warn)' }}>
                SSH authentication failed — fix this before saving
              </p>
              {sshHint && (
                <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                  {sshHint}
                </p>
              )}
            </div>
          </div>
          <button
            type="button"
            onClick={() => setChecklistOpen((v) => !v)}
            className="self-start flex items-center gap-1 text-xs"
            style={{
              color: 'var(--text-secondary)',
              background: 'none',
              border: 'none',
              padding: 0,
              cursor: 'pointer',
            }}
          >
            {checklistOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
            What to check
          </button>
          {checklistOpen && (
            <ul className="flex flex-col gap-1 pl-4" style={{ color: 'var(--text-secondary)' }}>
              {SSH_AUTH_CHECKLIST.map((item, i) => (
                <li key={i} className="text-xs list-disc">
                  {item}
                </li>
              ))}
            </ul>
          )}
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

      {/* SSH host key — render only when the handshake actually captured one.
          With auth failure the backend still returns the fingerprint (the
          host-key callback fires before auth), so the user can verify the
          host while fixing creds; but if SSH never connected at all we have
          nothing to show, and rendering an empty card is misleading. */}
      {result.ssh_host_key_sha256 && (
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
      )}
    </div>
  )
}
