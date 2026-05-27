import { useState } from 'react'
import { X, AlertTriangle, Loader } from 'lucide-react'
import { ApiError } from '../../lib/api'
import { useCreateNode, useProbePreview } from '../../lib/api/nodes'
import { ProbeResult } from './ProbeResult'
import type { NodeType, CredentialMethod, PreviewResult } from '../../types/api'

interface AddNodeWizardProps {
  onClose: () => void
}

interface FieldProps {
  label: string
  hint?: string
  children: React.ReactNode
}

function Field({ label, hint, children }: FieldProps) {
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
      style={{ ...INPUT_STYLE, resize: 'vertical', minHeight: '80px' }}
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

function StepIndicator({ current, total }: { current: number; total: number }) {
  return (
    <div className="flex items-center gap-1.5">
      {Array.from({ length: total }, (_, i) => (
        <div
          key={i}
          style={{
            width: i + 1 === current ? '20px' : '6px',
            height: '4px',
            borderRadius: '2px',
            backgroundColor: i + 1 <= current ? 'var(--accent)' : 'var(--border-default)',
            transition: 'all 0.15s',
          }}
        />
      ))}
      <span className="text-xs ml-1" style={{ color: 'var(--text-muted)' }}>
        Step {current} of {total}
      </span>
    </div>
  )
}

const STEP_TITLES = ['Connection', 'Credentials', 'Probe', 'Confirm']

export function AddNodeWizard({ onClose }: AddNodeWizardProps) {
  const [step, setStep] = useState(1)

  // Step 1 fields
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [sshPort, setSshPort] = useState('22')

  // Step 2 credential fields
  const [credMethod, setCredMethod] = useState<CredentialMethod>('ssh_key')
  const [sshUser, setSshUser] = useState('')
  const [sshPassword, setSshPassword] = useState('')
  const [sshPrivateKey, setSshPrivateKey] = useState('')
  const [sshPassphrase, setSshPassphrase] = useState('')

  // Proxmox optional
  const [proxmoxEndpoint, setProxmoxEndpoint] = useState('')
  const [proxmoxTokenId, setProxmoxTokenId] = useState('')
  const [proxmoxSecret, setProxmoxSecret] = useState('')
  const [proxmoxTlsInsecure, setProxmoxTlsInsecure] = useState(false)

  // Docker optional
  const [dockerEndpoint, setDockerEndpoint] = useState('')
  const [dockerTlsCa, setDockerTlsCa] = useState('')
  const [dockerTlsCert, setDockerTlsCert] = useState('')
  const [dockerTlsKey, setDockerTlsKey] = useState('')
  const [ackInsecureDocker, setAckInsecureDocker] = useState(false)

  // Step 3 probe state
  const [probeResult, setProbeResult] = useState<PreviewResult | null>(null)
  const [keyAccepted, setKeyAccepted] = useState(false)
  const [typeOverride, setTypeOverride] = useState<NodeType | ''>('')

  // Error state
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const probePreview = useProbePreview()
  const createNode = useCreateNode()

  const isDockerTcpWithoutTls =
    dockerEndpoint.startsWith('tcp://') && !dockerTlsCa && !dockerTlsCert && !dockerTlsKey

  function buildCredentials() {
    const creds: Parameters<typeof createNode.mutate>[0]['credentials'] = {
      method: credMethod,
      ssh_user: sshUser,
    }
    if (credMethod === 'ssh_password') {
      creds.ssh_password = sshPassword
    } else {
      creds.ssh_private_key = sshPrivateKey
      if (sshPassphrase) creds.ssh_passphrase = sshPassphrase
    }
    if (proxmoxTokenId) {
      creds.proxmox_token_id = proxmoxTokenId
      creds.proxmox_secret = proxmoxSecret
    }
    if (dockerTlsCa) {
      creds.docker_tls_ca = dockerTlsCa
      creds.docker_tls_cert = dockerTlsCert
      creds.docker_tls_key = dockerTlsKey
    }
    return creds
  }

  function buildProbeBody() {
    return {
      host,
      ssh_port: parseInt(sshPort, 10) || 22,
      credentials: buildCredentials(),
      ...(proxmoxEndpoint ? { proxmox_endpoint: proxmoxEndpoint } : {}),
      ...(proxmoxTlsInsecure ? { proxmox_tls_insecure: true } : {}),
      ...(dockerEndpoint ? { docker_endpoint: dockerEndpoint } : {}),
      ...(ackInsecureDocker ? { ack_insecure_docker: true } : {}),
    }
  }

  function step1Valid() {
    return name.trim() && host.trim()
  }

  function step2Valid() {
    if (!sshUser.trim()) return false
    if (credMethod === 'ssh_password' && !sshPassword) return false
    if (credMethod === 'ssh_key' && !sshPrivateKey.trim()) return false
    if (isDockerTcpWithoutTls && !ackInsecureDocker) return false
    return true
  }

  async function handleProbe() {
    setErrorMsg(null)
    setProbeResult(null)
    setKeyAccepted(false)
    try {
      const result = await probePreview.mutateAsync(buildProbeBody())
      setProbeResult(result)
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 403) {
          setErrorMsg('Admin role required to probe nodes.')
        } else if (err.status === 429) {
          setErrorMsg('Too many probe requests. Wait a moment and try again.')
        } else {
          const body = err.body as { error?: string }
          setErrorMsg(body.error ?? 'Probe failed.')
        }
      } else {
        setErrorMsg('Probe failed.')
      }
    }
  }

  async function handleCreate() {
    if (!probeResult) return
    setErrorMsg(null)
    const body = {
      name: name.trim(),
      host: host.trim(),
      ssh_port: parseInt(sshPort, 10) || 22,
      credentials: buildCredentials(),
      accepted_host_key: probeResult.ssh_host_key_line,
      pinned_host_key: probeResult.ssh_host_key_line,
      ...(proxmoxEndpoint ? { proxmox_endpoint: proxmoxEndpoint } : {}),
      ...(proxmoxTlsInsecure ? { proxmox_tls_insecure: true } : {}),
      ...(dockerEndpoint ? { docker_endpoint: dockerEndpoint } : {}),
      ...(ackInsecureDocker ? { ack_insecure_docker: true } : {}),
      ...(typeOverride ? { type_override: typeOverride } : {}),
    }
    try {
      await createNode.mutateAsync(body)
      onClose()
    } catch (err) {
      if (err instanceof ApiError) {
        const b = err.body as { error?: string }
        if (b.error === 'host_key_required') {
          setErrorMsg('Host key must be accepted before saving.')
        } else if (b.error === 'host_key_mismatch') {
          setErrorMsg('Host key mismatch — the server key changed. Re-probe to verify.')
        } else if (b.error === 'name_and_host_required') {
          setErrorMsg('Name and host are required.')
        } else if (b.error === 'insecure_docker_endpoint_requires_ack') {
          setErrorMsg('You must acknowledge the insecure Docker endpoint.')
        } else {
          setErrorMsg(b.error ?? 'Failed to save node.')
        }
      } else {
        setErrorMsg('Failed to save node.')
      }
    }
  }

  function goNext() {
    setErrorMsg(null)
    if (step === 2) {
      setStep(3)
      void handleProbe()
    } else {
      setStep((s) => s + 1)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      style={{ backgroundColor: 'rgba(0,0,0,0.6)' }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
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
          <div className="flex flex-col gap-1">
            <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
              Add Node — {STEP_TITLES[step - 1]}
            </span>
            <StepIndicator current={step} total={4} />
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
        <div className="flex-1 overflow-auto px-5 py-4">
          {/* Step 1: Connection */}
          {step === 1 && (
            <div className="flex flex-col gap-4">
              <Field label="Node name">
                <StyledInput
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. homelab-01"
                  autoFocus
                />
              </Field>
              <Field label="Host" hint="hostname or IP">
                <StyledInput
                  type="text"
                  value={host}
                  onChange={(e) => setHost(e.target.value)}
                  placeholder="192.168.1.10"
                />
              </Field>
              <Field label="SSH port">
                <StyledInput
                  type="number"
                  value={sshPort}
                  onChange={(e) => setSshPort(e.target.value)}
                  min={1}
                  max={65535}
                  style={{ ...INPUT_STYLE, width: '120px' }}
                />
              </Field>
            </div>
          )}

          {/* Step 2: Credentials */}
          {step === 2 && (
            <div className="flex flex-col gap-5">
              {/* Auth method picker */}
              <Field label="Authentication method">
                <div className="flex gap-2">
                  {(['ssh_key', 'ssh_password'] as CredentialMethod[]).map((m) => (
                    <button
                      key={m}
                      type="button"
                      onClick={() => setCredMethod(m)}
                      className="px-3 py-1.5 text-xs font-medium"
                      style={{
                        backgroundColor: credMethod === m ? 'var(--accent-glow)' : 'var(--bg-elevated)',
                        border: `1px solid ${credMethod === m ? 'var(--accent-dim)' : 'var(--border-default)'}`,
                        color: credMethod === m ? 'var(--accent)' : 'var(--text-secondary)',
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
                  value={sshUser}
                  onChange={(e) => setSshUser(e.target.value)}
                  placeholder="root"
                  autoComplete="username"
                />
              </Field>

              {credMethod === 'ssh_password' && (
                <Field label="SSH password">
                  <StyledInput
                    type="password"
                    value={sshPassword}
                    onChange={(e) => setSshPassword(e.target.value)}
                    autoComplete="current-password"
                  />
                </Field>
              )}

              {credMethod === 'ssh_key' && (
                <>
                  <Field label="Private key" hint="PEM format">
                    <StyledTextarea
                      value={sshPrivateKey}
                      onChange={(e) => setSshPrivateKey(e.target.value)}
                      placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                      rows={5}
                    />
                  </Field>
                  <Field label="Key passphrase" hint="optional">
                    <StyledInput
                      type="password"
                      value={sshPassphrase}
                      onChange={(e) => setSshPassphrase(e.target.value)}
                    />
                  </Field>
                </>
              )}

              {/* Proxmox optional section */}
              <div
                className="flex flex-col gap-3 pt-3"
                style={{ borderTop: '1px solid var(--border-subtle)' }}
              >
                <p className="text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
                  Proxmox API (optional)
                </p>
                <Field label="Proxmox endpoint" hint="https://host:8006">
                  <StyledInput
                    type="text"
                    value={proxmoxEndpoint}
                    onChange={(e) => setProxmoxEndpoint(e.target.value)}
                    placeholder="https://192.168.1.10:8006"
                  />
                </Field>
                {proxmoxEndpoint && (
                  <>
                    <Field label="Token ID" hint="user@realm!tokenname">
                      <StyledInput
                        type="text"
                        value={proxmoxTokenId}
                        onChange={(e) => setProxmoxTokenId(e.target.value)}
                        placeholder="root@pam!mytoken"
                      />
                    </Field>
                    <Field label="Token secret">
                      <StyledInput
                        type="password"
                        value={proxmoxSecret}
                        onChange={(e) => setProxmoxSecret(e.target.value)}
                      />
                    </Field>
                    <div className="flex items-center gap-2">
                      <input
                        id="px-tls-insecure"
                        type="checkbox"
                        checked={proxmoxTlsInsecure}
                        onChange={(e) => setProxmoxTlsInsecure(e.target.checked)}
                        style={{ accentColor: 'var(--accent)', cursor: 'pointer' }}
                      />
                      <label htmlFor="px-tls-insecure" className="text-xs" style={{ color: 'var(--text-secondary)', cursor: 'pointer' }}>
                        Allow self-signed TLS certificate
                      </label>
                    </div>
                    {proxmoxTlsInsecure && (
                      <WarnBox>
                        Self-signed TLS verification is disabled. Traffic is still encrypted but the
                        certificate will not be validated. Use only on trusted private networks.
                      </WarnBox>
                    )}
                  </>
                )}
              </div>

              {/* Docker optional section */}
              <div
                className="flex flex-col gap-3 pt-3"
                style={{ borderTop: '1px solid var(--border-subtle)' }}
              >
                <p className="text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
                  Docker endpoint (optional — leave blank for socket default)
                </p>
                <Field label="Docker endpoint">
                  <StyledInput
                    type="text"
                    value={dockerEndpoint}
                    onChange={(e) => setDockerEndpoint(e.target.value)}
                    placeholder="tcp://192.168.1.10:2376"
                  />
                </Field>
                {dockerEndpoint && !isDockerTcpWithoutTls && (
                  <>
                    <Field label="TLS CA certificate" hint="optional">
                      <StyledTextarea
                        value={dockerTlsCa}
                        onChange={(e) => setDockerTlsCa(e.target.value)}
                        placeholder="-----BEGIN CERTIFICATE-----"
                        rows={3}
                      />
                    </Field>
                    <Field label="TLS client certificate" hint="optional">
                      <StyledTextarea
                        value={dockerTlsCert}
                        onChange={(e) => setDockerTlsCert(e.target.value)}
                        rows={3}
                      />
                    </Field>
                    <Field label="TLS client key" hint="optional">
                      <StyledTextarea
                        value={dockerTlsKey}
                        onChange={(e) => setDockerTlsKey(e.target.value)}
                        rows={3}
                      />
                    </Field>
                  </>
                )}
                {isDockerTcpWithoutTls && (
                  <>
                    <WarnBox>
                      This Docker endpoint uses TCP without TLS. The Docker daemon API will be
                      accessible over the network without encryption or authentication. This is a
                      significant security risk — use only on isolated private networks.
                    </WarnBox>
                    <div className="flex items-start gap-2">
                      <input
                        id="ack-insecure-docker"
                        type="checkbox"
                        checked={ackInsecureDocker}
                        onChange={(e) => setAckInsecureDocker(e.target.checked)}
                        style={{ accentColor: 'var(--accent)', cursor: 'pointer', marginTop: '1px' }}
                      />
                      <label htmlFor="ack-insecure-docker" className="text-xs" style={{ color: 'var(--text-secondary)', cursor: 'pointer' }}>
                        I understand the risks of an unencrypted Docker TCP endpoint
                      </label>
                    </div>
                  </>
                )}
              </div>
            </div>
          )}

          {/* Step 3: Probe */}
          {step === 3 && (
            <div className="flex flex-col gap-4">
              {probePreview.isPending && (
                <div className="flex items-center gap-2 py-6 justify-center">
                  <Loader size={16} className="animate-spin" style={{ color: 'var(--accent)' }} />
                  <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                    Probing {host}...
                  </span>
                </div>
              )}
              {errorMsg && !probePreview.isPending && (
                <p className="text-xs" style={{ color: 'var(--status-error)' }}>
                  {errorMsg}
                </p>
              )}
              {probeResult && !probePreview.isPending && (
                <ProbeResult
                  result={probeResult}
                  acceptedKey={keyAccepted}
                  onAcceptKey={() => setKeyAccepted(true)}
                  typeOverride={typeOverride}
                  onTypeOverride={setTypeOverride}
                />
              )}
              {!probePreview.isPending && !probeResult && !errorMsg && (
                <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
                  Waiting for probe response...
                </p>
              )}
            </div>
          )}

          {/* Step 4: Confirm */}
          {step === 4 && probeResult && (
            <div className="flex flex-col gap-4">
              <div
                className="flex flex-col gap-2 p-3"
                style={{
                  backgroundColor: 'var(--bg-elevated)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                }}
              >
                <Row label="Name" value={name} mono />
                <Row label="Host" value={host} mono />
                <Row label="SSH port" value={sshPort} mono />
                <Row label="Type" value={typeOverride || probeResult.type} mono />
                <Row label="OS" value={probeResult.os_type} mono />
                <Row label="Auth" value={credMethod === 'ssh_key' ? 'SSH key' : 'SSH password'} />
              </div>
              {errorMsg && (
                <p className="text-xs" style={{ color: 'var(--status-error)' }}>
                  {errorMsg}
                </p>
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div
          className="flex items-center justify-between px-5 py-3 shrink-0"
          style={{ borderTop: '1px solid var(--border-subtle)' }}
        >
          <button
            type="button"
            onClick={() => {
              if (step === 1) {
                onClose()
              } else {
                setErrorMsg(null)
                setStep((s) => s - 1)
              }
            }}
            className="px-3 py-1.5 text-xs"
            style={{
              backgroundColor: 'transparent',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            {step === 1 ? 'Cancel' : 'Back'}
          </button>

          {step < 4 && (
            <button
              type="button"
              onClick={goNext}
              disabled={
                (step === 1 && !step1Valid()) ||
                (step === 2 && !step2Valid()) ||
                (step === 3 && (!probeResult || !keyAccepted))
              }
              className="px-4 py-1.5 text-xs font-medium disabled:opacity-40"
              style={{
                backgroundColor: 'var(--accent)',
                color: 'var(--text-inverse)',
                border: 'none',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              {step === 2 ? 'Probe' : 'Next'}
            </button>
          )}

          {step === 4 && (
            <button
              type="button"
              onClick={() => void handleCreate()}
              disabled={createNode.isPending}
              className="flex items-center gap-1.5 px-4 py-1.5 text-xs font-medium disabled:opacity-40"
              style={{
                backgroundColor: 'var(--accent)',
                color: 'var(--text-inverse)',
                border: 'none',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              {createNode.isPending && <Loader size={12} className="animate-spin" />}
              {createNode.isPending ? 'Saving...' : 'Save node'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center gap-3">
      <span className="text-xs w-20 shrink-0" style={{ color: 'var(--text-muted)' }}>
        {label}
      </span>
      <span
        className={`text-xs ${mono ? 'font-mono' : ''}`}
        style={{ color: 'var(--text-primary)' }}
      >
        {value}
      </span>
    </div>
  )
}
