import { useState } from 'react'
import { Globe, Lock, Loader, Plus, Cloud, Check, AlertTriangle, X } from 'lucide-react'
import { useContainerProxy, useAddContainerProxy } from '../../lib/api/proxy'
import { useCan } from '../../lib/roles'
import { ApiError } from '../../lib/api'
import type {
  ContainerProxyStatus,
  ProxyAddTarget,
  ProxyRule,
  AddProxyRoutePlan,
} from '../../types/api'

interface Props {
  containerId: string
}

const sectionStyle: React.CSSProperties = {
  borderTop: '1px solid var(--border-subtle)',
  paddingTop: '8px',
  marginTop: '8px',
}
const labelStyle: React.CSSProperties = { fontSize: '12px', color: 'var(--text-muted)' }
const inputStyle: React.CSSProperties = {
  background: 'var(--bg-surface)',
  border: '1px solid var(--border-default)',
  color: 'var(--text-primary)',
  borderRadius: '3px',
  outline: 'none',
}

function apiErrorMessage(err: unknown): string | null {
  if (!err) return null
  const e = err as ApiError
  const body = e.body as { error?: string } | undefined
  return body?.error ?? 'Request failed.'
}

function Header() {
  return (
    <div className="flex items-center gap-1.5 mb-1">
      <Globe size={12} style={{ color: 'var(--text-muted)' }} />
      <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
        Reverse Proxy
      </span>
    </div>
  )
}

// ── Serving routes (hostnames already pointing here) ─────────────────────────

function ServingRoute({ r }: { r: ProxyRule }) {
  return (
    <div className="flex items-center gap-2 py-0.5">
      <a
        href={`https://${r.source_host}`}
        target="_blank"
        rel="noopener noreferrer"
        className="font-mono text-xs"
        style={{
          color: 'var(--accent)',
          textDecoration: 'underline',
          textDecorationColor: 'var(--accent-dim)',
        }}
      >
        {r.source_host}
        {r.source_path ? r.source_path : ''}
      </a>
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
      <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
        via {r.adapter_type}
      </span>
    </div>
  )
}

// ── Add-route form ───────────────────────────────────────────────────────────

function AddRouteForm({
  containerId,
  target,
  suggested,
  onClose,
}: {
  containerId: string
  target: ProxyAddTarget
  suggested: string[]
  onClose: () => void
}) {
  const [host, setHost] = useState('')
  const [targetUrl, setTargetUrl] = useState(suggested[0] ?? '')
  const [createDns, setCreateDns] = useState(true)
  // The previewed plan from a dry-run; null until previewed, cleared on edits.
  const [preview, setPreview] = useState<AddProxyRoutePlan | null>(null)

  const add = useAddContainerProxy(containerId)
  const isCloudflare = target.adapter === 'cloudflare-api'
  const canSubmit = host.trim() !== '' && targetUrl.trim() !== ''

  function resetPreviewOnEdit<T>(setter: (v: T) => void) {
    return (v: T) => {
      setPreview(null)
      add.reset()
      setter(v)
    }
  }

  function runPreview() {
    add.reset()
    add.mutate(
      {
        proxy_node_id: target.node_id,
        source_host: host.trim(),
        target_url: targetUrl.trim(),
        create_dns: createDns,
        dry_run: true,
      },
      { onSuccess: (plan) => setPreview(plan) },
    )
  }

  function confirmAdd() {
    add.reset()
    add.mutate(
      {
        proxy_node_id: target.node_id,
        source_host: host.trim(),
        target_url: targetUrl.trim(),
        create_dns: createDns,
        dry_run: false,
      },
      {
        onSuccess: (plan) => {
          setPreview(plan)
          if (plan.applied && !plan.warning) {
            // Brief success state, then close so the new route renders above.
            setTimeout(onClose, 1200)
          }
        },
      },
    )
  }

  const err = apiErrorMessage(add.error)
  const applied = preview?.applied ?? false

  return (
    <div
      className="flex flex-col gap-2 mt-2 p-3"
      style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
      }}
    >
      <div className="flex items-center justify-between">
        <span className="font-mono text-xs flex items-center gap-1.5" style={{ color: 'var(--text-secondary)' }}>
          {isCloudflare && <Cloud size={12} />}
          Add a route via {target.adapter}
          {target.cf_tunnel_id ? ` (tunnel ${target.cf_tunnel_id})` : ''}
        </span>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close"
          style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer' }}
        >
          <X size={13} />
        </button>
      </div>

      {/* Public hostname */}
      <div className="flex flex-col gap-1">
        <label className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          Public hostname
        </label>
        <input
          type="text"
          placeholder="jellyfin.kaylas.systems"
          value={host}
          onChange={(e) => resetPreviewOnEdit(setHost)(e.target.value)}
          className="font-mono text-xs px-2 py-1.5"
          style={inputStyle}
        />
      </div>

      {/* Target service URL */}
      <div className="flex flex-col gap-1">
        <label className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          Target (this container)
        </label>
        {suggested.length > 0 ? (
          <select
            value={targetUrl}
            onChange={(e) => resetPreviewOnEdit(setTargetUrl)(e.target.value)}
            className="font-mono text-xs px-2 py-1.5"
            style={inputStyle}
          >
            {suggested.map((u) => (
              <option key={u} value={u}>
                {u}
              </option>
            ))}
          </select>
        ) : (
          <input
            type="text"
            placeholder="http://192.168.20.9:8096"
            value={targetUrl}
            onChange={(e) => resetPreviewOnEdit(setTargetUrl)(e.target.value)}
            className="font-mono text-xs px-2 py-1.5"
            style={inputStyle}
          />
        )}
        {suggested.length === 0 && (
          <span className="font-mono" style={{ color: 'var(--text-muted)', fontSize: '10px' }}>
            No published ports detected — enter the address the proxy can reach.
          </span>
        )}
      </div>

      {/* DNS toggle (cloudflare-api) */}
      {isCloudflare && (
        <label className="flex items-center gap-2 font-mono text-xs" style={{ color: 'var(--text-secondary)' }}>
          <input
            type="checkbox"
            checked={createDns}
            onChange={(e) => resetPreviewOnEdit(setCreateDns)(e.target.checked)}
          />
          Create the proxied DNS record (needed for the hostname to resolve)
        </label>
      )}

      {/* Preview panel (from dry-run) */}
      {preview && !applied && (
        <div
          className="flex flex-col gap-1 p-2 font-mono text-xs"
          style={{
            background: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            color: 'var(--text-secondary)',
          }}
        >
          <span style={{ color: 'var(--text-muted)' }}>This will:</span>
          <span>• add ingress {preview.source_host} → {preview.target_url}</span>
          {preview.dns_record && <span>• create DNS {preview.dns_record}</span>}
          {!preview.dns_record && isCloudflare && <span>• leave DNS unchanged</span>}
        </div>
      )}

      {/* Applied / warning state */}
      {applied && (
        <div
          className="flex items-start gap-1.5 p-2 font-mono text-xs"
          style={{
            background: preview?.warning ? 'rgba(255,180,0,0.08)' : 'rgba(0,200,100,0.08)',
            border: `1px solid ${preview?.warning ? 'var(--status-warn)' : 'var(--status-ok)'}`,
            borderRadius: '3px',
            color: preview?.warning ? 'var(--status-warn)' : 'var(--status-ok)',
          }}
        >
          {preview?.warning ? <AlertTriangle size={12} /> : <Check size={12} />}
          <span>
            Route added.{' '}
            {preview?.warning ? `Warning: ${preview.warning}` : 'It may take a moment to resolve.'}
          </span>
        </div>
      )}

      {err && (
        <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
          {err}
        </span>
      )}

      {/* Actions: Preview → Confirm */}
      {!applied && (
        <div className="flex items-center gap-2">
          {!preview ? (
            <button
              type="button"
              disabled={!canSubmit || add.isPending}
              onClick={runPreview}
              className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
              style={{
                background: 'var(--bg-surface)',
                border: '1px solid var(--border-default)',
                color: !canSubmit ? 'var(--text-muted)' : 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: !canSubmit || add.isPending ? 'not-allowed' : 'pointer',
                opacity: !canSubmit || add.isPending ? 0.6 : 1,
              }}
            >
              {add.isPending ? <Loader size={12} className="animate-spin" /> : null}
              Preview
            </button>
          ) : (
            <button
              type="button"
              disabled={add.isPending}
              onClick={confirmAdd}
              className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
              style={{
                background: 'var(--accent-glow)',
                border: '1px solid var(--accent-dim)',
                color: add.isPending ? 'var(--text-muted)' : 'var(--accent)',
                borderRadius: '3px',
                cursor: add.isPending ? 'not-allowed' : 'pointer',
              }}
            >
              {add.isPending ? <Loader size={12} className="animate-spin" /> : <Check size={12} />}
              Confirm &amp; add
            </button>
          )}
        </div>
      )}
    </div>
  )
}

// ── Main section ─────────────────────────────────────────────────────────────

export function ContainerProxySection({ containerId }: Props) {
  const { isAdmin } = useCan()
  const { data, isLoading, isError } = useContainerProxy(isAdmin ? containerId : undefined)
  const [adding, setAdding] = useState(false)

  if (!isAdmin) return null

  if (isLoading) {
    return (
      <div style={sectionStyle}>
        <Header />
        <Loader size={12} className="animate-spin mt-1" style={{ color: 'var(--text-muted)' }} />
      </div>
    )
  }
  // A 403/404 or any error degrades quietly — this is a secondary panel.
  if (isError || !data) return null

  const status: ContainerProxyStatus = data
  const addTarget = status.add_targets[0]

  return (
    <div style={sectionStyle}>
      <Header />

      {status.routes.length > 0 ? (
        <div className="flex flex-col">
          {status.routes.map((r) => (
            <ServingRoute key={`${r.adapter_type}:${r.id}`} r={r} />
          ))}
        </div>
      ) : (
        <p className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          Not exposed via a reverse proxy.
        </p>
      )}

      {addTarget && !adding && (
        <button
          type="button"
          onClick={() => setAdding(true)}
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
          <Plus size={12} />
          Add reverse proxy
        </button>
      )}

      {addTarget && adding && (
        <AddRouteForm
          containerId={containerId}
          target={addTarget}
          suggested={status.suggested_targets}
          onClose={() => setAdding(false)}
        />
      )}

      {!addTarget && status.routes.length === 0 && (
        <p className="font-mono text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
          No proxy with add capability is configured. Connect a Cloudflare tunnel (API) on a host to
          add routes here.
        </p>
      )}
    </div>
  )
}
