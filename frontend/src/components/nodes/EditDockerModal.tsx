import { useState } from 'react'
import { X, AlertTriangle, Loader } from 'lucide-react'
import { ApiError } from '../../lib/api'
import { useUpdateNode } from '../../lib/api/nodes'
import { DockerTestConnection } from './DockerTestConnection'
import type {
  NodeView,
  CredentialMethod,
  ProbePreviewRequest,
  UpdateNodeRequest,
} from '../../types/api'

interface EditDockerModalProps {
  node: NodeView
  onClose: () => void
}

const INPUT_STYLE: React.CSSProperties = {
  backgroundColor: 'var(--bg-elevated)',
  border: '1px solid var(--border-default)',
  color: 'var(--text-primary)',
  borderRadius: '3px',
  width: '100%',
  padding: '6px 10px',
  fontSize: '12px',
  fontFamily: 'Space Mono, monospace',
  outline: 'none',
}

function StyledInput(props: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      {...props}
      style={INPUT_STYLE}
      onFocus={(e) => {
        e.currentTarget.style.borderColor = 'var(--accent)'
        props.onFocus?.(e)
      }}
      onBlur={(e) => {
        e.currentTarget.style.borderColor = 'var(--border-default)'
        props.onBlur?.(e)
      }}
    />
  )
}

function StyledTextarea(props: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      {...props}
      style={{ ...INPUT_STYLE, resize: 'vertical', minHeight: '70px' }}
      onFocus={(e) => {
        e.currentTarget.style.borderColor = 'var(--accent)'
        props.onFocus?.(e)
      }}
      onBlur={(e) => {
        e.currentTarget.style.borderColor = 'var(--border-default)'
        props.onBlur?.(e)
      }}
    />
  )
}

function Field({
  label,
  hint,
  children,
}: {
  label: string
  hint?: string
  children: React.ReactNode
}) {
  return (
    <div className="flex flex-col gap-1">
      <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
        {label}
        {hint && (
          <span className="ml-1.5 font-normal" style={{ color: 'var(--text-muted)' }}>
            {hint}
          </span>
        )}
      </label>
      {children}
    </div>
  )
}

function WarnBox({ children }: { children: React.ReactNode }) {
  return (
    <div
      className="flex items-start gap-2 px-3 py-2"
      style={{
        backgroundColor: 'rgba(240, 160, 32, 0.08)',
        border: '1px solid rgba(240, 160, 32, 0.35)',
        borderRadius: '3px',
      }}
    >
      <AlertTriangle size={12} style={{ color: 'var(--status-warn)', marginTop: '1px', flexShrink: 0 }} />
      <span className="text-xs" style={{ color: 'var(--status-warn)' }}>
        {children}
      </span>
    </div>
  )
}

/**
 * Edit the Docker configuration of an already-registered node: set/update the
 * docker_endpoint and optional TLS material, then save via PUT /api/nodes/{id}.
 *
 * The probe-preview endpoint is stateless and needs SSH credentials to reach
 * the host, but a saved node's creds live server-side and aren't exposed to the
 * UI. So "Test connection" here is gated on the operator re-entering SSH
 * credentials for a one-off reachability check — those test creds are never
 * persisted by this modal; only docker_endpoint + TLS are saved.
 */
export function EditDockerModal({ node, onClose }: EditDockerModalProps) {
  const update = useUpdateNode()

  const [dockerEndpoint, setDockerEndpoint] = useState(node.docker_endpoint ?? '')
  const [enableTls, setEnableTls] = useState(false)
  const [tlsCa, setTlsCa] = useState('')
  const [tlsCert, setTlsCert] = useState('')
  const [tlsKey, setTlsKey] = useState('')
  const [ackInsecure, setAckInsecure] = useState(false)

  // One-off SSH creds used only to run the Test-connection probe (never saved).
  const [testMethod, setTestMethod] = useState<CredentialMethod>('ssh_key')
  const [testUser, setTestUser] = useState('')
  const [testPassword, setTestPassword] = useState('')
  const [testPrivateKey, setTestPrivateKey] = useState('')
  const [testPassphrase, setTestPassphrase] = useState('')

  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const isTcpWithoutTls =
    dockerEndpoint.startsWith('tcp://') && !(enableTls && (tlsCa || tlsCert || tlsKey))

  function testCredsValid(): boolean {
    if (!testUser.trim()) return false
    if (testMethod === 'ssh_password' && !testPassword) return false
    if (testMethod === 'ssh_key' && !testPrivateKey.trim()) return false
    return true
  }

  function buildProbeBody(): ProbePreviewRequest | null {
    if (!dockerEndpoint.trim() || !testCredsValid()) return null
    const creds: ProbePreviewRequest['credentials'] = {
      method: testMethod,
      ssh_user: testUser.trim(),
    }
    if (testMethod === 'ssh_password') {
      creds.ssh_password = testPassword
    } else {
      creds.ssh_private_key = testPrivateKey
      if (testPassphrase) creds.ssh_passphrase = testPassphrase
    }
    if (enableTls) {
      if (tlsCa) creds.docker_tls_ca = tlsCa
      if (tlsCert) creds.docker_tls_cert = tlsCert
      if (tlsKey) creds.docker_tls_key = tlsKey
    }
    return {
      host: node.host,
      ssh_port: node.port,
      credentials: creds,
      docker_endpoint: dockerEndpoint.trim(),
      ...(ackInsecure ? { ack_insecure_docker: true } : {}),
    }
  }

  const saveDisabled =
    update.isPending || (isTcpWithoutTls && dockerEndpoint.trim() !== '' && !ackInsecure)

  async function handleSave() {
    setErrorMsg(null)
    const body: UpdateNodeRequest = {
      docker_endpoint: dockerEndpoint.trim(),
      ...(ackInsecure ? { ack_insecure_docker: true } : {}),
    }
    if (enableTls && (tlsCa || tlsCert || tlsKey)) {
      body.credentials = {
        ...(tlsCa ? { docker_tls_ca: tlsCa } : {}),
        ...(tlsCert ? { docker_tls_cert: tlsCert } : {}),
        ...(tlsKey ? { docker_tls_key: tlsKey } : {}),
      }
    }
    try {
      await update.mutateAsync({ id: node.id, body })
      onClose()
    } catch (err) {
      if (err instanceof ApiError) {
        const b = err.body as { error?: string }
        if (b.error === 'insecure_docker_endpoint_requires_ack') {
          setErrorMsg('You must acknowledge the insecure Docker endpoint.')
        } else {
          setErrorMsg(b.error ?? 'Failed to save Docker config.')
        }
      } else {
        setErrorMsg('Failed to save Docker config.')
      }
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      style={{ backgroundColor: 'rgba(0,0,0,0.6)' }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div
        className="flex flex-col w-full max-w-lg max-h-[90vh]"
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
        }}
      >
        {/* Header */}
        <div
          className="flex items-center justify-between px-5 py-3 shrink-0"
          style={{ borderBottom: '1px solid var(--border-subtle)' }}
        >
          <div className="flex flex-col gap-0.5">
            <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
              Docker config — {node.name}
            </span>
            <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
              {node.host}:{node.port}
            </span>
          </div>
          <button
            onClick={onClose}
            className="flex items-center justify-center"
            style={{
              background: 'transparent',
              border: 'none',
              color: 'var(--text-muted)',
              cursor: 'pointer',
              padding: '4px',
            }}
            aria-label="Close"
          >
            <X size={16} />
          </button>
        </div>

        {/* Body */}
        <div className="flex-1 overflow-auto px-5 py-4 flex flex-col gap-5">
          <Field
            label="Docker endpoint"
            hint="tcp://host:2376 · unix:///var/run/docker.sock · ssh://user@host"
          >
            <StyledInput
              type="text"
              value={dockerEndpoint}
              onChange={(e) => setDockerEndpoint(e.target.value)}
              placeholder="tcp://192.168.1.10:2376"
              autoFocus
            />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Leave blank to clear and fall back to the local socket default.
            </span>
          </Field>

          {/* TLS toggle */}
          <div className="flex flex-col gap-3">
            <div className="flex items-center gap-2">
              <input
                id="edit-docker-tls"
                type="checkbox"
                checked={enableTls}
                onChange={(e) => setEnableTls(e.target.checked)}
                style={{ accentColor: 'var(--accent)', cursor: 'pointer' }}
              />
              <label
                htmlFor="edit-docker-tls"
                className="text-xs font-medium"
                style={{ color: 'var(--text-secondary)', cursor: 'pointer' }}
              >
                Use TLS client certificates
              </label>
              <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                optional — plain tcp to a socket-proxy is valid
              </span>
            </div>

            {enableTls && (
              <>
                <Field label="TLS CA certificate" hint="PEM">
                  <StyledTextarea
                    value={tlsCa}
                    onChange={(e) => setTlsCa(e.target.value)}
                    placeholder="-----BEGIN CERTIFICATE-----"
                    rows={3}
                  />
                </Field>
                <Field label="TLS client certificate" hint="PEM">
                  <StyledTextarea
                    value={tlsCert}
                    onChange={(e) => setTlsCert(e.target.value)}
                    placeholder="-----BEGIN CERTIFICATE-----"
                    rows={3}
                  />
                </Field>
                <Field label="TLS client key" hint="PEM">
                  <StyledTextarea
                    value={tlsKey}
                    onChange={(e) => setTlsKey(e.target.value)}
                    placeholder="-----BEGIN PRIVATE KEY-----"
                    rows={3}
                  />
                </Field>
              </>
            )}
          </div>

          {/* Insecure ack */}
          {isTcpWithoutTls && dockerEndpoint.trim() !== '' && (
            <div className="flex flex-col gap-2">
              <WarnBox>
                This Docker endpoint uses TCP without TLS. The Docker daemon API will be accessible
                over the network without encryption or authentication. Use only on isolated private
                networks.
              </WarnBox>
              <div className="flex items-start gap-2">
                <input
                  id="edit-ack-insecure-docker"
                  type="checkbox"
                  checked={ackInsecure}
                  onChange={(e) => setAckInsecure(e.target.checked)}
                  style={{ accentColor: 'var(--accent)', cursor: 'pointer', marginTop: '1px' }}
                />
                <label
                  htmlFor="edit-ack-insecure-docker"
                  className="text-xs"
                  style={{ color: 'var(--text-secondary)', cursor: 'pointer' }}
                >
                  I understand the risks of an unencrypted Docker TCP endpoint
                </label>
              </div>
            </div>
          )}

          {/* Test connection — needs SSH creds (not stored client-side) */}
          <div
            className="flex flex-col gap-3 pt-3"
            style={{ borderTop: '1px solid var(--border-subtle)' }}
          >
            <p className="text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
              Test connection (optional)
            </p>
            <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Re-enter SSH credentials to verify Docker reachability. These are used only for the
              test and are not saved — only the Docker endpoint and TLS material above are stored.
            </p>

            <Field label="SSH auth method">
              <div className="flex gap-2">
                {(['ssh_key', 'ssh_password'] as CredentialMethod[]).map((m) => (
                  <button
                    key={m}
                    type="button"
                    onClick={() => setTestMethod(m)}
                    className="px-3 py-1.5 text-xs font-medium"
                    style={{
                      backgroundColor: testMethod === m ? 'var(--accent-glow)' : 'var(--bg-elevated)',
                      border: `1px solid ${testMethod === m ? 'var(--accent-dim)' : 'var(--border-default)'}`,
                      color: testMethod === m ? 'var(--accent)' : 'var(--text-secondary)',
                      borderRadius: '3px',
                      cursor: 'pointer',
                    }}
                  >
                    {m === 'ssh_key' ? 'SSH Key' : 'SSH Password'}
                  </button>
                ))}
              </div>
            </Field>

            <Field label="SSH username">
              <StyledInput
                type="text"
                value={testUser}
                onChange={(e) => setTestUser(e.target.value)}
                placeholder="root"
                autoComplete="username"
              />
            </Field>

            {testMethod === 'ssh_password' && (
              <Field label="SSH password">
                <StyledInput
                  type="password"
                  value={testPassword}
                  onChange={(e) => setTestPassword(e.target.value)}
                  autoComplete="current-password"
                />
              </Field>
            )}

            {testMethod === 'ssh_key' && (
              <>
                <Field label="Private key" hint="PEM">
                  <StyledTextarea
                    value={testPrivateKey}
                    onChange={(e) => setTestPrivateKey(e.target.value)}
                    placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                    rows={4}
                  />
                </Field>
                <Field label="Key passphrase" hint="optional">
                  <StyledInput
                    type="password"
                    value={testPassphrase}
                    onChange={(e) => setTestPassphrase(e.target.value)}
                  />
                </Field>
              </>
            )}

            <DockerTestConnection
              buildBody={buildProbeBody}
              disabledReason={
                !dockerEndpoint.trim()
                  ? 'Enter a Docker endpoint above first.'
                  : !testCredsValid()
                    ? 'Enter SSH credentials to test.'
                    : undefined
              }
            />
          </div>

          {errorMsg && (
            <p className="text-xs" style={{ color: 'var(--status-error)' }}>
              {errorMsg}
            </p>
          )}
        </div>

        {/* Footer */}
        <div
          className="flex items-center justify-between px-5 py-3 shrink-0"
          style={{ borderTop: '1px solid var(--border-subtle)' }}
        >
          <button
            type="button"
            onClick={onClose}
            className="px-3 py-1.5 text-xs"
            style={{
              backgroundColor: 'transparent',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={() => void handleSave()}
            disabled={saveDisabled}
            className="flex items-center gap-1.5 px-4 py-1.5 text-xs font-medium disabled:opacity-40"
            style={{
              backgroundColor: 'var(--accent)',
              color: 'var(--text-inverse)',
              border: 'none',
              borderRadius: '3px',
              cursor: saveDisabled ? 'not-allowed' : 'pointer',
            }}
          >
            {update.isPending && <Loader size={12} className="animate-spin" />}
            {update.isPending ? 'Saving...' : 'Save Docker config'}
          </button>
        </div>
      </div>
    </div>
  )
}
