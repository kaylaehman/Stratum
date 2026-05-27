import { useState } from 'react'
import { Globe, ServerCog, List, Save, Loader } from 'lucide-react'
import { useNodeDns, useSetDnsConfig } from '../../lib/api/dns'
import { useCan } from '../../lib/roles'
import { ApiError } from '../../lib/api'
import type { DnsRecord, DnsRecordType, SupportedDns } from '../../types/api'

interface DnsPanelProps {
  nodeId: string
}

// ── Supported-tools catalog shown when nothing is detected ──────────────────

function SupportedCatalog({ tools }: { tools: SupportedDns[] }) {
  return (
    <div className="flex flex-col gap-1 mt-2">
      {tools.map((t) => {
        const hasAny = t.capabilities.list || t.capabilities.create
        return (
          <div key={t.name} className="flex items-center gap-2">
            <ServerCog size={11} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
            <span className="font-mono text-xs" style={{ color: 'var(--text-secondary)' }}>
              {t.name}
            </span>
            <span
              className="font-mono text-xs px-1"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-subtle)',
                color: 'var(--text-muted)',
                borderRadius: '3px',
              }}
            >
              {hasAny ? 'api' : 'config-file'}
            </span>
          </div>
        )
      })}
    </div>
  )
}

// ── Record-type badge ────────────────────────────────────────────────────────

function RecordTypeBadge({ type }: { type: DnsRecordType }) {
  const isAccent = type === 'A' || type === 'AAAA'
  const isWarn = type === 'CNAME'

  const color = isAccent ? 'var(--accent)' : isWarn ? 'var(--status-warn)' : 'var(--text-muted)'
  const bg = isAccent
    ? 'rgba(var(--accent-rgb, 100,149,237),0.10)'
    : isWarn
      ? 'rgba(255,180,0,0.10)'
      : 'var(--bg-elevated)'
  const border = isAccent
    ? 'var(--accent-dim)'
    : isWarn
      ? 'var(--status-warn)'
      : 'var(--border-subtle)'

  return (
    <span
      className="font-mono text-xs px-1"
      style={{ background: bg, border: `1px solid ${border}`, color, borderRadius: '3px' }}
    >
      {type}
    </span>
  )
}

// ── Records table ─────────────────────────────────────────────────────────────

function RecordsTable({ records }: { records: DnsRecord[] }) {
  const hasttl = records.some((r) => r.ttl !== undefined)
  const hasComment = records.some((r) => r.comment)

  return (
    <div
      className="mt-2 overflow-x-auto"
      style={{ border: '1px solid var(--border-subtle)', borderRadius: '3px' }}
    >
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
            {(['Type', 'Name', 'Value'] as const).map((h) => (
              <th
                key={h}
                className="font-mono text-xs text-left px-2 py-1"
                style={{ color: 'var(--text-muted)', background: 'var(--bg-elevated)' }}
              >
                {h}
              </th>
            ))}
            {hasttl && (
              <th
                className="font-mono text-xs text-left px-2 py-1"
                style={{ color: 'var(--text-muted)', background: 'var(--bg-elevated)' }}
              >
                TTL
              </th>
            )}
            {hasComment && (
              <th
                className="font-mono text-xs text-left px-2 py-1"
                style={{ color: 'var(--text-muted)', background: 'var(--bg-elevated)' }}
              >
                Comment
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {records.map((r) => (
            <tr key={r.id} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
              <td className="px-2 py-1.5">
                <RecordTypeBadge type={r.type} />
              </td>
              <td className="font-mono text-xs px-2 py-1.5" style={{ color: 'var(--text-primary)' }}>
                {r.name}
              </td>
              <td
                className="font-mono text-xs px-2 py-1.5 max-w-xs truncate"
                style={{ color: 'var(--text-secondary)' }}
              >
                {r.value}
              </td>
              {hasttl && (
                <td className="font-mono text-xs px-2 py-1.5" style={{ color: 'var(--text-muted)' }}>
                  {r.ttl ?? '—'}
                </td>
              )}
              {hasComment && (
                <td
                  className="font-mono text-xs px-2 py-1.5 max-w-xs truncate"
                  style={{ color: 'var(--text-muted)' }}
                >
                  {r.comment ?? ''}
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// ── Admin-endpoint config form ───────────────────────────────────────────────

interface ConfigFormProps {
  nodeId: string
  detected: string
  currentEndpoint: string | undefined
  hasToken: boolean
  recordError?: string
}

function ConfigForm({ nodeId, detected, currentEndpoint, hasToken, recordError }: ConfigFormProps) {
  const [endpoint, setEndpoint] = useState(currentEndpoint ?? '')
  // undefined = unchanged (keep), '' = clear, string = new value
  const [tokenDraft, setTokenDraft] = useState<string | undefined>(undefined)
  const [tokenAction, setTokenAction] = useState<'keep' | 'replace' | 'clear'>(
    hasToken ? 'keep' : 'replace',
  )

  const { mutate, isPending, error, reset } = useSetDnsConfig(nodeId)

  const apiErr = error as ApiError | null
  const errCode = apiErr ? (apiErr.body as { error?: string })?.error : null
  const errorMsg =
    errCode === 'invalid_endpoint'
      ? 'Invalid endpoint — must be a host-only http(s) URL (no path or query).'
      : errCode === 'invalid_body'
        ? 'Invalid request body.'
        : apiErr
          ? 'Save failed.'
          : null

  function handleSave() {
    reset()
    const body: SetDnsConfigRequestLocal = { endpoint: endpoint.trim() }
    if (tokenAction === 'replace' && tokenDraft !== undefined) body.token = tokenDraft
    if (tokenAction === 'clear') body.token = ''
    mutate({ nodeId, request: body })
  }

  const placeholder =
    detected === 'adguard'
      ? 'http://adguard.lan'
      : detected === 'pihole'
        ? 'http://pihole.lan'
        : detected === 'bind9'
          ? 'http://bind9.lan:8080'
          : 'http://dns.lan'

  const showTokenField = tokenAction === 'replace'

  return (
    <div
      className="flex flex-col gap-2 mt-2 p-3"
      style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
      }}
    >
      <p className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
        Configure the admin endpoint so Stratum can read DNS records.
      </p>

      {recordError && (
        <p
          className="font-mono text-xs"
          style={{
            color: 'var(--status-warn)',
            background: 'rgba(255,180,0,0.07)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            padding: '4px 8px',
          }}
        >
          {recordError.toLowerCase().startsWith('admin endpoint not configured')
            ? 'Admin endpoint not configured — set one below to load records.'
            : `Couldn't reach DNS admin API: ${recordError}`}
        </p>
      )}

      {/* Endpoint field */}
      <div className="flex flex-col gap-1">
        <label className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          Admin endpoint
        </label>
        <input
          type="text"
          placeholder={placeholder}
          value={endpoint}
          onChange={(e) => setEndpoint(e.target.value)}
          className="font-mono text-xs px-2 py-1.5"
          style={{
            background: 'var(--bg-surface)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-primary)',
            borderRadius: '3px',
            outline: 'none',
          }}
        />
      </div>

      {/* Token — keep / replace / clear */}
      {hasToken && tokenAction === 'keep' ? (
        <div className="flex items-center gap-2">
          <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
            Token: <span style={{ color: 'var(--status-ok)' }}>set</span>
          </span>
          <button
            type="button"
            onClick={() => setTokenAction('replace')}
            className="font-mono text-xs px-1.5 py-0.5"
            style={{
              background: 'var(--bg-surface)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Replace
          </button>
          <button
            type="button"
            onClick={() => setTokenAction('clear')}
            className="font-mono text-xs px-1.5 py-0.5"
            style={{
              background: 'var(--bg-surface)',
              border: '1px solid var(--border-default)',
              color: 'var(--status-warn)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Clear
          </button>
        </div>
      ) : (
        showTokenField && (
          <div className="flex flex-col gap-1">
            <label className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
              Token (optional)
            </label>
            <div className="flex items-center gap-2">
              <input
                type="password"
                placeholder={hasToken ? '(leave blank to keep existing)' : 'Bearer token…'}
                value={tokenDraft ?? ''}
                onChange={(e) => setTokenDraft(e.target.value)}
                className="font-mono text-xs px-2 py-1.5 flex-1"
                style={{
                  background: 'var(--bg-surface)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-primary)',
                  borderRadius: '3px',
                  outline: 'none',
                }}
              />
              {hasToken && (
                <button
                  type="button"
                  onClick={() => { setTokenAction('keep'); setTokenDraft(undefined) }}
                  className="font-mono text-xs px-1.5 py-0.5"
                  style={{
                    background: 'var(--bg-surface)',
                    border: '1px solid var(--border-default)',
                    color: 'var(--text-muted)',
                    borderRadius: '3px',
                    cursor: 'pointer',
                  }}
                >
                  Cancel
                </button>
              )}
            </div>
          </div>
        )
      )}

      {!showTokenField && !hasToken && (
        <button
          type="button"
          onClick={() => setTokenAction('replace')}
          className="font-mono text-xs self-start px-1.5 py-0.5"
          style={{
            background: 'none',
            border: 'none',
            color: 'var(--text-muted)',
            cursor: 'pointer',
            padding: 0,
            textDecoration: 'underline',
          }}
        >
          + Add token
        </button>
      )}

      {errorMsg && (
        <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
          {errorMsg}
        </span>
      )}

      <button
        type="button"
        disabled={isPending || endpoint.trim() === ''}
        onClick={handleSave}
        className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1 self-start"
        style={{
          background: 'var(--accent-glow)',
          border: '1px solid var(--accent-dim)',
          color: isPending ? 'var(--text-muted)' : 'var(--accent)',
          borderRadius: '3px',
          cursor: isPending || endpoint.trim() === '' ? 'not-allowed' : 'pointer',
          opacity: isPending || endpoint.trim() === '' ? 0.6 : 1,
        }}
      >
        {isPending ? <Loader size={12} className="animate-spin" /> : <Save size={12} />}
        Save
      </button>
    </div>
  )
}

// Local alias for the request body shape (token omit = keep)
interface SetDnsConfigRequestLocal {
  endpoint: string
  token?: string
}

// ── Small sub-components ─────────────────────────────────────────────────────

function DetectedBadge({ name }: { name: string }) {
  return (
    <span
      className="font-mono text-xs px-1.5 py-0.5"
      style={{
        background: 'rgba(var(--accent-rgb, 100,149,237),0.10)',
        border: '1px solid var(--accent-dim)',
        color: 'var(--accent)',
        borderRadius: '3px',
      }}
    >
      {name}
    </span>
  )
}

function CapabilityHint({ capabilities }: { capabilities: { list: boolean; create: boolean } }) {
  const label = capabilities.list
    ? capabilities.create
      ? 'read-write'
      : 'read-only'
    : 'config-file managed'
  return (
    <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
      {label}
    </span>
  )
}

// ── Main panel ───────────────────────────────────────────────────────────────

export function DnsPanel({ nodeId }: DnsPanelProps) {
  const { isAdmin } = useCan()
  const { data, isLoading, isError } = useNodeDns(nodeId)

  if (!isAdmin) return null

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
        <div className="flex items-center gap-1.5">
          <Globe size={12} style={{ color: 'var(--text-muted)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
            DNS Records
          </span>
        </div>
        <Loader size={12} className="animate-spin mt-2" style={{ color: 'var(--text-muted)' }} />
      </div>
    )
  }

  if (isError || !data) {
    return (
      <div style={sectionStyle}>
        <div className="flex items-center gap-1.5">
          <Globe size={12} style={{ color: 'var(--text-muted)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
            DNS Records
          </span>
        </div>
        <p className="font-mono text-xs mt-2" style={{ color: 'var(--status-error)' }}>
          Could not load DNS status.
        </p>
      </div>
    )
  }

  // ── No DNS tool detected ──
  if (data.detected === '') {
    return (
      <div style={sectionStyle}>
        <div className="flex items-center gap-1.5 mb-1">
          <Globe size={12} style={{ color: 'var(--text-muted)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
            DNS Records
          </span>
        </div>
        <p className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          No supported DNS tool detected on this node.
        </p>
        {data.supported.length > 0 && (
          <>
            <p className="font-mono text-xs mt-2" style={{ color: 'var(--text-muted)' }}>
              Stratum recognises:
            </p>
            <SupportedCatalog tools={data.supported} />
          </>
        )}
      </div>
    )
  }

  // ── DNS detected ──
  const hasListCapability = data.capabilities.list
  const hasAnyCapability = hasListCapability || data.capabilities.create

  // Detection-only: no API capabilities
  if (!hasAnyCapability) {
    return (
      <div style={sectionStyle}>
        <div className="flex items-center gap-1.5 mb-2">
          <Globe size={12} style={{ color: 'var(--text-muted)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
            DNS Records
          </span>
          <DetectedBadge name={data.detected} />
          <CapabilityHint capabilities={data.capabilities} />
        </div>
        <p className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          Detected — manage via its config file / admin UI.
        </p>
      </div>
    )
  }

  // ── Has list capability: may need config, or may have records ──
  const needsConfig =
    !data.configured || (data.record_error !== undefined && data.record_error !== '')
  const showConfigForm = hasListCapability && needsConfig

  return (
    <div style={sectionStyle}>
      <div className="flex items-center gap-1.5 mb-2 flex-wrap">
        <Globe size={12} style={{ color: 'var(--text-muted)' }} />
        <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
          DNS Records
        </span>
        <DetectedBadge name={data.detected} />
        <CapabilityHint capabilities={data.capabilities} />
      </div>

      {/* Config form when unconfigured or errored */}
      {showConfigForm && (
        <ConfigForm
          nodeId={nodeId}
          detected={data.detected}
          currentEndpoint={data.endpoint}
          hasToken={data.has_token}
          recordError={data.record_error}
        />
      )}

      {/* Records table */}
      {!showConfigForm && data.records.length > 0 && (
        <>
          <div className="flex items-center gap-1.5 mb-1">
            <List size={11} style={{ color: 'var(--text-muted)' }} />
            <span
              className="font-mono text-xs px-1"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-subtle)',
                color: 'var(--text-muted)',
                borderRadius: '3px',
              }}
            >
              {data.records.length} record{data.records.length !== 1 ? 's' : ''}
            </span>
          </div>
          <RecordsTable records={data.records} />
        </>
      )}

      {!showConfigForm && data.records.length === 0 && !data.record_error && (
        <p className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          No DNS records found.
        </p>
      )}
    </div>
  )
}
