import { useState } from 'react'
import { Save, Loader, Search, X } from 'lucide-react'
import { useDiscoverCloudflare, useSetProxyConfig } from '../../lib/api/proxy'
import { ApiError } from '../../lib/api'
import type { CloudflareAccount, CloudflareTunnel } from '../../types/api'

interface CloudflareApiFormProps {
  nodeId: string
  /** A token is already stored for this node (editing an existing connection). */
  hasToken: boolean
  currentAccountId?: string
  currentTunnelId?: string
  /** Shown as a cancel/close affordance when the form is opened inline. */
  onClose?: () => void
}

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

/** Setup/edit form for the cloudflare-api provider: enter an API token, discover
 *  the account + tunnel (or type the ids), and save. The token is sent over the
 *  PUT (sealed server-side) and for discovery (used in-memory only). */
export function CloudflareApiForm({
  nodeId,
  hasToken,
  currentAccountId,
  currentTunnelId,
  onClose,
}: CloudflareApiFormProps) {
  const [token, setToken] = useState('')
  const [accountId, setAccountId] = useState(currentAccountId ?? '')
  const [tunnelId, setTunnelId] = useState(currentTunnelId ?? '')
  const [accounts, setAccounts] = useState<CloudflareAccount[]>([])
  const [tunnels, setTunnels] = useState<CloudflareTunnel[]>([])

  const discover = useDiscoverCloudflare(nodeId)
  const save = useSetProxyConfig(nodeId)

  // Discovery needs a token: either freshly entered, or the one already stored.
  const canDiscover = token.trim() !== '' || hasToken
  const canSave = tunnelId.trim() !== '' && (token.trim() !== '' || hasToken)

  function runDiscover(forAccount?: string) {
    discover.reset()
    const acct = forAccount ?? (accountId.trim() || undefined)
    discover.mutate(
      { token: token.trim() || undefined, account_id: acct },
      {
        onSuccess: (res) => {
          setAccounts(res.accounts)
          setTunnels(res.tunnels)
          if (res.account_id) setAccountId(res.account_id)
          // Keep the current tunnel selected if it's still in the list.
          if (res.tunnels.length > 0 && !res.tunnels.some((t) => t.id === tunnelId)) {
            // leave tunnelId as-is so a manually-typed id survives
          }
        },
      },
    )
  }

  function handleSave() {
    save.reset()
    save.mutate(
      {
        nodeId,
        request: {
          endpoint: '',
          kind: 'cloudflare-api',
          account_id: accountId.trim() || undefined,
          tunnel_id: tunnelId.trim(),
          token: token.trim() !== '' ? token.trim() : undefined,
        },
      },
      {
        // Close the form on success when it was opened as an editable overlay.
        // For first-time setup (no onClose) the panel re-renders from the
        // invalidated query and the form unmounts as the rules load.
        onSuccess: () => onClose?.(),
      },
    )
  }

  // A failed Discover surfaces the Cloudflare message; a partial success (200
  // with an embedded error) carries it on the result instead.
  const discoverErr = apiErrorMessage(discover.error) ?? discover.data?.error ?? null
  const saveErr = save.error ? 'Save failed.' : null

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
        <p className="font-mono text-xs" style={{ color: 'var(--text-secondary)' }}>
          Read a dashboard-managed Cloudflare tunnel's routes over the API.
        </p>
        {onClose && (
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer' }}
          >
            <X size={13} />
          </button>
        )}
      </div>

      {/* API token */}
      <div className="flex flex-col gap-1">
        <label className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          API token
        </label>
        <input
          type="password"
          placeholder={hasToken ? '(leave blank to keep stored token)' : 'Cloudflare API token…'}
          value={token}
          onChange={(e) => setToken(e.target.value)}
          className="font-mono text-xs px-2 py-1.5"
          style={inputStyle}
        />
        <span className="font-mono" style={{ color: 'var(--text-muted)', fontSize: '10px' }}>
          Required scope: Account → Cloudflare Tunnel → Read.
        </span>
      </div>

      {/* Account (optional) — picker appears after discovery when multiple exist */}
      {accounts.length > 1 ? (
        <div className="flex flex-col gap-1">
          <label className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
            Account
          </label>
          <select
            value={accountId}
            onChange={(e) => {
              setAccountId(e.target.value)
              setTunnels([])
              setTunnelId('')
              if (e.target.value) runDiscover(e.target.value)
            }}
            className="font-mono text-xs px-2 py-1.5"
            style={inputStyle}
          >
            <option value="">Select an account…</option>
            {accounts.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name} ({a.id})
              </option>
            ))}
          </select>
        </div>
      ) : (
        <div className="flex flex-col gap-1">
          <label className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
            Account ID (optional — discovered automatically)
          </label>
          <input
            type="text"
            placeholder="Leave blank to auto-discover"
            value={accountId}
            onChange={(e) => setAccountId(e.target.value)}
            className="font-mono text-xs px-2 py-1.5"
            style={inputStyle}
          />
        </div>
      )}

      {/* Discover button */}
      <button
        type="button"
        disabled={!canDiscover || discover.isPending}
        onClick={() => runDiscover()}
        className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1 self-start"
        style={{
          background: 'var(--bg-surface)',
          border: '1px solid var(--border-default)',
          color: !canDiscover ? 'var(--text-muted)' : 'var(--text-secondary)',
          borderRadius: '3px',
          cursor: !canDiscover || discover.isPending ? 'not-allowed' : 'pointer',
          opacity: !canDiscover || discover.isPending ? 0.6 : 1,
        }}
      >
        {discover.isPending ? <Loader size={12} className="animate-spin" /> : <Search size={12} />}
        Discover tunnels
      </button>

      {discoverErr && (
        <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
          {discoverErr}
        </span>
      )}

      {/* Tunnel — dropdown after discovery, else manual id entry */}
      <div className="flex flex-col gap-1">
        <label className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          Tunnel
        </label>
        {tunnels.length > 0 ? (
          <select
            value={tunnelId}
            onChange={(e) => setTunnelId(e.target.value)}
            className="font-mono text-xs px-2 py-1.5"
            style={inputStyle}
          >
            <option value="">Select a tunnel…</option>
            {tunnels.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name} ({t.id})
              </option>
            ))}
          </select>
        ) : (
          <input
            type="text"
            placeholder="Tunnel ID (or use Discover above)"
            value={tunnelId}
            onChange={(e) => setTunnelId(e.target.value)}
            className="font-mono text-xs px-2 py-1.5"
            style={inputStyle}
          />
        )}
      </div>

      {saveErr && (
        <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
          {saveErr}
        </span>
      )}

      <button
        type="button"
        disabled={!canSave || save.isPending}
        onClick={handleSave}
        className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1 self-start"
        style={{
          background: 'var(--accent-glow)',
          border: '1px solid var(--accent-dim)',
          color: !canSave || save.isPending ? 'var(--text-muted)' : 'var(--accent)',
          borderRadius: '3px',
          cursor: !canSave || save.isPending ? 'not-allowed' : 'pointer',
          opacity: !canSave || save.isPending ? 0.6 : 1,
        }}
      >
        {save.isPending ? <Loader size={12} className="animate-spin" /> : <Save size={12} />}
        Save connection
      </button>
    </div>
  )
}
