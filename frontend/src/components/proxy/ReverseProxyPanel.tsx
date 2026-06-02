import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Globe, Lock, Save, Loader, Network, Cloud } from 'lucide-react'
import { useNodeProxy, useSetProxyConfig } from '../../lib/api/proxy'
import { useCan } from '../../lib/roles'
import { ApiError } from '../../lib/api'
import { resourceLink } from '../../lib/resourceLink'
import { CloudflareApiForm } from './CloudflareApiForm'
import type { ProxyRule, SupportedProxy } from '../../types/api'

// Small underline-style button used for the cloudflare-api entry points.
function LinkButton({ icon, label, onClick }: { icon: React.ReactNode; label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center gap-1.5 font-mono text-xs self-start mt-2"
      style={{
        background: 'none',
        border: 'none',
        color: 'var(--accent)',
        cursor: 'pointer',
        padding: 0,
        textDecoration: 'underline',
        textDecorationColor: 'var(--accent-dim)',
      }}
    >
      {icon}
      {label}
    </button>
  )
}

interface ReverseProxyPanelProps {
  nodeId: string
}

// ── Supported-tools catalog shown when nothing is detected ──────────────────

function SupportedCatalog({ tools }: { tools: SupportedProxy[] }) {
  return (
    <div className="flex flex-col gap-1 mt-2">
      {tools.map((t) => {
        const hasAny = t.capabilities.list || t.capabilities.create
        return (
          <div key={t.name} className="flex items-center gap-2">
            <Network size={11} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
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

// ── Rules table ──────────────────────────────────────────────────────────────

function RulesTable({ rules }: { rules: ProxyRule[] }) {
  return (
    <div
      className="mt-2 overflow-x-auto"
      style={{
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
      }}
    >
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
            {['Source host', 'Path', 'Target', 'SSL', 'Auth'].map((h) => (
              <th
                key={h}
                className="font-mono text-xs text-left px-2 py-1"
                style={{ color: 'var(--text-muted)', background: 'var(--bg-elevated)' }}
              >
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rules.map((r) => (
            <tr
              key={r.id}
              style={{ borderBottom: '1px solid var(--border-subtle)' }}
            >
              {/* Source host — external link to the public hostname */}
              <td className="font-mono text-xs px-2 py-1.5">
                <a
                  href={`https://${r.source_host}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  style={{
                    color: 'var(--text-primary)',
                    textDecoration: 'underline',
                    textDecorationColor: 'var(--border-subtle)',
                  }}
                >
                  {r.source_host}
                </a>
              </td>
              <td className="font-mono text-xs px-2 py-1.5" style={{ color: 'var(--text-muted)' }}>
                {r.source_path ?? '/'}
              </td>
              {/* Target — resolved container link, or plain url with not-found hint */}
              <td className="font-mono text-xs px-2 py-1.5 max-w-xs">
                {r.resolved ? (
                  <Link
                    to={resourceLink(r.resolved.node_id, r.resolved.container_id)}
                    title={r.target_url}
                    style={{
                      color: 'var(--accent)',
                      textDecoration: 'underline',
                      textDecorationColor: 'var(--accent-dim)',
                    }}
                  >
                    {r.resolved.name}
                  </Link>
                ) : (
                  <span className="flex flex-col gap-0.5">
                    <span className="truncate" style={{ color: 'var(--text-secondary)' }}>
                      {r.target_url}
                    </span>
                    <span style={{ color: 'var(--text-muted)', fontSize: '10px' }}>
                      not found
                    </span>
                  </span>
                )}
              </td>
              <td className="px-2 py-1.5">
                {r.ssl_enabled && (
                  <span
                    className="inline-flex items-center gap-1 font-mono text-xs px-1"
                    style={{
                      background: 'rgba(0,200,100,0.10)',
                      border: '1px solid var(--status-ok)',
                      color: 'var(--status-ok)',
                      borderRadius: '3px',
                    }}
                  >
                    <Lock size={10} />
                    TLS
                  </span>
                )}
              </td>
              <td className="px-2 py-1.5">
                {r.auth_enabled && (
                  <span
                    className="font-mono text-xs px-1"
                    style={{
                      background: 'rgba(255,180,0,0.10)',
                      border: '1px solid var(--status-warn)',
                      color: 'var(--status-warn)',
                      borderRadius: '3px',
                    }}
                  >
                    auth
                  </span>
                )}
              </td>
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
}

function ConfigForm({ nodeId, detected, currentEndpoint, hasToken }: ConfigFormProps) {
  const [endpoint, setEndpoint] = useState(currentEndpoint ?? '')
  // undefined = unchanged, '' = clear, 'value' = set new
  const [tokenDraft, setTokenDraft] = useState<string | undefined>(undefined)
  const [tokenAction, setTokenAction] = useState<'keep' | 'replace' | 'clear'>(
    hasToken ? 'keep' : 'replace',
  )

  const { mutate, isPending, error, reset } = useSetProxyConfig(nodeId)

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
    const body: { endpoint: string; token?: string } = { endpoint: endpoint.trim() }
    if (tokenAction === 'replace' && tokenDraft !== undefined) body.token = tokenDraft
    if (tokenAction === 'clear') body.token = ''
    mutate({ nodeId, request: body })
  }

  const needsToken = detected === 'nginx-proxy-manager'
  const showTokenField = needsToken || tokenAction === 'replace'

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
        Configure the admin endpoint so Stratum can read proxy rules.
      </p>

      {/* Endpoint field */}
      <div className="flex flex-col gap-1">
        <label className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          Admin endpoint
        </label>
        <input
          type="text"
          placeholder={
            detected === 'traefik'
              ? 'http://traefik.lan:8080'
              : detected === 'nginx-proxy-manager'
                ? 'http://npm.lan:81'
                : detected === 'caddy'
                  ? 'http://caddy.lan:2019'
                  : 'http://proxy.lan:8080'
          }
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

      {/* Token field — always optional, shown for NPM or when user opts in */}
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
              {needsToken ? 'API token' : 'Token (optional)'}
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

// ── Cloudflared SSH-required notice ─────────────────────────────────────────

function CloudflaredSshNotice() {
  return (
    <p
      className="font-mono text-xs mb-2"
      style={{
        color: 'var(--text-secondary)',
        background: 'rgba(255,180,0,0.07)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        padding: '4px 8px',
      }}
    >
      cloudflared ingress rules are read from the on-disk config file, which requires SSH
      access to this node.{' '}
      <Link
        to="/nodes"
        style={{ color: 'var(--accent)', textDecoration: 'underline' }}
      >
        Add SSH credentials for this node
      </Link>{' '}
      to enable rule listing.
    </p>
  )
}

// ── Main panel ───────────────────────────────────────────────────────────────

export function ReverseProxyPanel({ nodeId }: ReverseProxyPanelProps) {
  const { isAdmin } = useCan()
  const { data, isLoading, isError } = useNodeProxy(nodeId)
  // Toggles the cloudflare-api setup/edit form (empty state, dashboard-managed
  // cloudflared notice, or editing an existing cloudflare-api connection).
  const [cfFormOpen, setCfFormOpen] = useState(false)

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
            Reverse Proxy
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
            Reverse Proxy
          </span>
        </div>
        <p className="font-mono text-xs mt-2" style={{ color: 'var(--status-error)' }}>
          Could not load proxy status.
        </p>
      </div>
    )
  }

  // ── No proxy detected ──
  if (data.detected === '') {
    return (
      <div style={sectionStyle}>
        <div className="flex items-center gap-1.5 mb-1">
          <Globe size={12} style={{ color: 'var(--text-muted)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
            Reverse Proxy
          </span>
        </div>
        <p className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          No supported reverse proxy detected on this node.
        </p>
        {data.supported.length > 0 && (
          <>
            <p
              className="font-mono text-xs mt-2"
              style={{ color: 'var(--text-muted)' }}
            >
              Stratum recognises:
            </p>
            <SupportedCatalog tools={data.supported} />
          </>
        )}
        {/* Cloudflare API: connect a dashboard-managed tunnel (no local container). */}
        {cfFormOpen ? (
          <CloudflareApiForm nodeId={nodeId} hasToken={data.has_token} onClose={() => setCfFormOpen(false)} />
        ) : (
          <LinkButton
            icon={<Cloud size={12} />}
            label="Connect a Cloudflare tunnel via API"
            onClick={() => setCfFormOpen(true)}
          />
        )}
      </div>
    )
  }

  // ── Proxy detected ──
  const hasListCapability = data.capabilities.list
  const hasAnyCapability = hasListCapability || data.capabilities.create

  // Detection-only: no capabilities at all
  if (!hasAnyCapability) {
    return (
      <div style={sectionStyle}>
        <div className="flex items-center gap-1.5 mb-2">
          <Globe size={12} style={{ color: 'var(--text-muted)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
            Reverse Proxy
          </span>
          <DetectedBadge name={data.detected} />
          <CapabilityHint capabilities={data.capabilities} />
        </div>
        <p className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          Detected — manage via its config file in the File Browser.
        </p>
      </div>
    )
  }

  // ── Has list capability: may need config, or may have rules ──
  // File-based adapters (cloudflared) never need the admin-endpoint config form —
  // they read from the host filesystem. Only show the form for API-based adapters
  // when they are unconfigured or in error.
  const isFileBased = data.detected === 'cloudflared'
  const isCloudflareApi = data.detected === 'cloudflare-api'
  // The generic endpoint ConfigForm only applies to HTTP-admin adapters
  // (traefik, NPM, caddy). cloudflared is file-based; cloudflare-api has its own
  // token/tunnel form, so both are excluded here.
  const needsConfig =
    !isFileBased &&
    !isCloudflareApi &&
    (!data.configured || (data.rule_error !== undefined && data.rule_error !== ''))
  const showConfigForm = hasListCapability && needsConfig

  // cloudflare-api: show its setup form when unconfigured, in error, or when the
  // user opts to edit an existing connection.
  const showCloudflareForm =
    isCloudflareApi &&
    (!data.configured || cfFormOpen || (data.rule_error !== undefined && data.rule_error !== ''))

  // Detect the specific case: cloudflared + rule_error indicating SSH is missing.
  // The backend surfaces this as a rule_error when SSH access is unavailable.
  const cloudflaredNeedsSsh =
    isFileBased &&
    !data.dashboard_managed &&
    data.rule_error !== undefined &&
    data.rule_error !== ''

  return (
    <div style={sectionStyle}>
      <div className="flex items-center gap-1.5 mb-2 flex-wrap">
        <Globe size={12} style={{ color: 'var(--text-muted)' }} />
        <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
          Reverse Proxy
        </span>
        <DetectedBadge name={data.detected} />
        <CapabilityHint capabilities={data.capabilities} />
      </div>

      {/* cloudflared: dashboard-managed tunnel — ingress is defined in the
          Cloudflare Zero Trust dashboard, not in a local config file. Offer to
          read the routes over the Cloudflare API instead.                      */}
      {isFileBased && data.dashboard_managed && (
        <>
          <p
            className="font-mono text-xs mb-2"
            style={{
              color: 'var(--text-secondary)',
              background: 'rgba(100,149,237,0.07)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
              padding: '4px 8px',
            }}
          >
            cloudflared tunnel detected — ingress managed in the Cloudflare dashboard. Rules are not
            available locally, but Stratum can read them over the Cloudflare API.
          </p>
          {cfFormOpen ? (
            <CloudflareApiForm
              nodeId={nodeId}
              hasToken={data.has_token}
              onClose={() => setCfFormOpen(false)}
            />
          ) : (
            <LinkButton
              icon={<Cloud size={12} />}
              label="Read routes via Cloudflare API"
              onClick={() => setCfFormOpen(true)}
            />
          )}
        </>
      )}

      {/* cloudflared: SSH not configured — actionable notice with link to add credentials */}
      {cloudflaredNeedsSsh && <CloudflaredSshNotice />}

      {/* Rule-fetch error notice — HTTP-admin adapters only (not cloudflared/cloudflare-api) */}
      {data.rule_error && !data.dashboard_managed && !isFileBased && !isCloudflareApi && (
        <p
          className="font-mono text-xs mb-2"
          style={{
            color: 'var(--status-warn)',
            background: 'rgba(255,180,0,0.07)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            padding: '4px 8px',
          }}
        >
          {data.rule_error.toLowerCase().startsWith('admin endpoint not configured')
            ? 'Admin endpoint not configured — set one below to load rules.'
            : `Couldn't reach the proxy admin API: ${data.rule_error}`}
        </p>
      )}

      {/* cloudflare-api: surface the Cloudflare error verbatim (bad token,
          missing scope, tunnel not found, locally-managed tunnel). */}
      {isCloudflareApi && data.rule_error && (
        <p
          className="font-mono text-xs mb-2"
          style={{
            color: 'var(--status-warn)',
            background: 'rgba(255,180,0,0.07)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            padding: '4px 8px',
          }}
        >
          {data.rule_error}
        </p>
      )}

      {/* Config form when unconfigured or in error — HTTP-admin adapters only */}
      {showConfigForm && (
        <ConfigForm
          nodeId={nodeId}
          detected={data.detected}
          currentEndpoint={data.endpoint}
          hasToken={data.has_token}
        />
      )}

      {/* cloudflare-api setup/edit form */}
      {showCloudflareForm && (
        <CloudflareApiForm
          nodeId={nodeId}
          hasToken={data.has_token}
          currentAccountId={data.cf_account_id}
          currentTunnelId={data.cf_tunnel_id}
          onClose={cfFormOpen ? () => setCfFormOpen(false) : undefined}
        />
      )}

      {/* cloudflare-api: edit affordance when a connection is active with rules */}
      {isCloudflareApi && !showCloudflareForm && data.configured && (
        <LinkButton
          icon={<Cloud size={12} />}
          label="Edit Cloudflare connection"
          onClick={() => setCfFormOpen(true)}
        />
      )}

      {/* Rules table */}
      {!showConfigForm && !showCloudflareForm && data.rules.length > 0 && (
        <>
          <div className="flex items-center gap-1.5 mb-1">
            <span
              className="font-mono text-xs px-1"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-subtle)',
                color: 'var(--text-muted)',
                borderRadius: '3px',
              }}
            >
              {data.rules.length} rule{data.rules.length !== 1 ? 's' : ''}
            </span>
          </div>
          <RulesTable rules={data.rules} />
        </>
      )}

      {!showConfigForm && !showCloudflareForm && data.rules.length === 0 && !data.rule_error && !data.dashboard_managed && (
        <p className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          No proxy rules found.
        </p>
      )}
    </div>
  )
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
    <span
      className="font-mono text-xs"
      style={{ color: 'var(--text-muted)' }}
    >
      {label}
    </span>
  )
}
