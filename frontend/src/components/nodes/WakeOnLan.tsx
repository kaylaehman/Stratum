import { useState } from 'react'
import { Power, Save, Loader, Pencil } from 'lucide-react'
import { useWOLConfig, useSetWOL, useWakeNode } from '../../lib/api/wol'
import { useMe } from '../../hooks/useMe'
import { ApiError } from '../../lib/api'

interface WakeOnLanProps {
  nodeId: string
}

interface WOLFormProps {
  nodeId: string
  initialMac?: string
  initialBroadcast?: string
  initialPort?: number
  onCancel?: () => void
}

function WOLForm({ nodeId, initialMac = '', initialBroadcast = '255.255.255.255', initialPort = 9, onCancel }: WOLFormProps) {
  const [mac, setMac] = useState(initialMac)
  const [broadcast, setBroadcast] = useState(initialBroadcast)
  const [port, setPort] = useState(String(initialPort))
  const { mutate: setWOL, isPending, error, reset } = useSetWOL()

  const apiErr = error as ApiError | null
  const errCode = apiErr?.status === 400
    ? (apiErr.body as { error?: string })?.error
    : null

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    reset()
    setWOL(
      { nodeId, request: { mac: mac.trim(), broadcast: broadcast.trim() || undefined, port: port ? Number(port) : undefined } },
      { onSuccess: onCancel },
    )
  }

  const inputStyle: React.CSSProperties = {
    background: 'var(--bg-elevated)',
    border: '1px solid var(--border-default)',
    color: 'var(--text-primary)',
    borderRadius: '3px',
    outline: 'none',
    fontFamily: 'monospace',
    fontSize: '12px',
    padding: '4px 8px',
    width: '100%',
  }

  const errorInputStyle: React.CSSProperties = {
    ...inputStyle,
    border: '1px solid var(--status-error)',
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-2 mt-2">
      <div className="flex flex-col gap-1">
        <label className="text-xs" style={{ color: 'var(--text-muted)' }}>
          MAC address <span style={{ color: 'var(--status-error)' }}>*</span>
        </label>
        <input
          type="text"
          placeholder="AA:BB:CC:DD:EE:FF"
          value={mac}
          onChange={(e) => setMac(e.target.value)}
          required
          style={errCode === 'invalid_mac' ? errorInputStyle : inputStyle}
        />
        {errCode === 'invalid_mac' && (
          <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
            Invalid MAC address format
          </span>
        )}
      </div>

      <div className="flex gap-2">
        <div className="flex flex-col gap-1 flex-1">
          <label className="text-xs" style={{ color: 'var(--text-muted)' }}>Broadcast</label>
          <input
            type="text"
            placeholder="255.255.255.255"
            value={broadcast}
            onChange={(e) => setBroadcast(e.target.value)}
            style={inputStyle}
          />
        </div>
        <div className="flex flex-col gap-1" style={{ width: '80px' }}>
          <label className="text-xs" style={{ color: 'var(--text-muted)' }}>Port</label>
          <input
            type="number"
            placeholder="9"
            value={port}
            onChange={(e) => setPort(e.target.value)}
            min={1}
            max={65535}
            style={errCode === 'invalid_port' ? errorInputStyle : inputStyle}
          />
        </div>
      </div>
      {errCode === 'invalid_port' && (
        <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
          Invalid port
        </span>
      )}

      <div className="flex items-center gap-2 mt-1">
        <button
          type="submit"
          disabled={isPending}
          className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
          style={{
            background: 'var(--accent-glow)',
            border: '1px solid var(--accent-dim)',
            color: isPending ? 'var(--text-muted)' : 'var(--accent)',
            borderRadius: '3px',
            cursor: isPending ? 'not-allowed' : 'pointer',
            opacity: isPending ? 0.6 : 1,
          }}
        >
          {isPending ? <Loader size={12} className="animate-spin" /> : <Save size={12} />}
          Save
        </button>
        {onCancel && (
          <button
            type="button"
            onClick={onCancel}
            disabled={isPending}
            className="font-mono text-xs px-2.5 py-1"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: isPending ? 'not-allowed' : 'pointer',
            }}
          >
            Cancel
          </button>
        )}
      </div>
    </form>
  )
}

export function WakeOnLan({ nodeId }: WakeOnLanProps) {
  const { data: me } = useMe()
  const { data: config, isError, isLoading } = useWOLConfig(nodeId)
  const { mutate: wake, isPending: waking, isSuccess: woke, error: wakeError, reset: resetWake } = useWakeNode()
  const [editing, setEditing] = useState(false)

  if (me?.role !== 'admin') return null

  const wakeApiErr = wakeError as ApiError | null
  const wakeErrCode = wakeApiErr
    ? (wakeApiErr.body as { error?: string })?.error
    : null

  const wakeErrMsg =
    wakeErrCode === 'wol_not_configured'
      ? 'Wake-on-LAN not configured'
      : wakeErrCode === 'wake_failed'
        ? 'Failed to send magic packet'
        : wakeError
          ? 'Unknown error'
          : null

  function handleWake() {
    resetWake()
    wake({ nodeId })
  }

  const sectionStyle: React.CSSProperties = {
    borderTop: '1px solid var(--border-subtle)',
    paddingTop: '8px',
    marginTop: '8px',
  }

  const labelStyle: React.CSSProperties = {
    fontSize: '12px',
    color: 'var(--text-muted)',
  }

  if (isLoading) {
    return (
      <div style={sectionStyle}>
        <span className="text-xs" style={labelStyle}>Wake-on-LAN</span>
        <Loader size={12} className="animate-spin mt-1" style={{ color: 'var(--text-muted)' }} />
      </div>
    )
  }

  // Not configured (404)
  if (isError && !config) {
    return (
      <div style={sectionStyle}>
        <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
          Wake-on-LAN
        </span>
        {editing ? (
          <WOLForm nodeId={nodeId} onCancel={() => setEditing(false)} />
        ) : (
          <button
            type="button"
            onClick={() => setEditing(true)}
            className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1 mt-2"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            <Power size={12} />
            Configure Wake-on-LAN
          </button>
        )}
      </div>
    )
  }

  // Configured
  if (config) {
    return (
      <div style={sectionStyle}>
        <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
          Wake-on-LAN
        </span>
        {editing ? (
          <WOLForm
            nodeId={nodeId}
            initialMac={config.mac}
            initialBroadcast={config.broadcast}
            initialPort={config.port}
            onCancel={() => setEditing(false)}
          />
        ) : (
          <div className="flex flex-col gap-2 mt-2">
            <div className="flex items-baseline gap-3">
              <span className="text-xs w-28 shrink-0" style={{ color: 'var(--text-muted)' }}>MAC</span>
              <span className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>{config.mac}</span>
            </div>
            <div className="flex items-center gap-2 mt-1 flex-wrap">
              <button
                type="button"
                disabled={waking}
                onClick={handleWake}
                className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
                style={{
                  background: waking ? 'var(--bg-elevated)' : 'var(--accent-glow)',
                  border: `1px solid ${waking ? 'var(--border-default)' : 'var(--accent-dim)'}`,
                  color: waking ? 'var(--text-muted)' : 'var(--accent)',
                  borderRadius: '3px',
                  cursor: waking ? 'not-allowed' : 'pointer',
                  opacity: waking ? 0.6 : 1,
                }}
              >
                {waking ? <Loader size={12} className="animate-spin" /> : <Power size={12} />}
                Wake
              </button>
              <button
                type="button"
                onClick={() => { resetWake(); setEditing(true) }}
                className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
                style={{
                  background: 'var(--bg-elevated)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-secondary)',
                  borderRadius: '3px',
                  cursor: 'pointer',
                }}
              >
                <Pencil size={12} />
                Edit
              </button>
            </div>
            {woke && (
              <span className="font-mono text-xs" style={{ color: 'var(--accent)' }}>
                Magic packet sent
              </span>
            )}
            {wakeErrMsg && (
              <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
                {wakeErrMsg}
              </span>
            )}
          </div>
        )}
      </div>
    )
  }

  return null
}
