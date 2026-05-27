import { useEffect, useState } from 'react'
import { ShieldCheck, LogIn, KeyRound, Plus, X, Save, Loader } from 'lucide-react'
import { useSSOConfigs, useUpsertSSO, useDeleteSSO } from '../../lib/api/sso'
import { ApiError } from '../../lib/api'
import type { SSOMethod, SSOUpsertRequest } from '../../types/api'

const METHOD_LABELS: Record<SSOMethod, string> = {
  local: 'Stratum local',
  totp: 'TOTP',
  oidc: 'OIDC',
  forward: 'Forward-auth header',
}

const METHODS: SSOMethod[] = ['local', 'totp', 'oidc', 'forward']

function needsProvider(method: SSOMethod): boolean {
  return method === 'oidc' || method === 'forward'
}

// ---- Allowed-groups list editor ----

interface GroupListProps {
  groups: string[]
  onChange: (groups: string[]) => void
}

function GroupList({ groups, onChange }: GroupListProps) {
  const [draft, setDraft] = useState('')

  function add() {
    const val = draft.trim()
    if (!val || groups.includes(val)) return
    onChange([...groups, val])
    setDraft('')
  }

  function remove(g: string) {
    onChange(groups.filter((x) => x !== g))
  }

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex flex-wrap gap-1.5">
        {groups.map((g) => (
          <span
            key={g}
            className="flex items-center gap-1 font-mono text-xs px-1.5 py-0.5"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              borderRadius: '3px',
              color: 'var(--text-secondary)',
            }}
          >
            {g}
            <button
              type="button"
              onClick={() => remove(g)}
              style={{ color: 'var(--text-muted)', cursor: 'pointer', lineHeight: 0 }}
            >
              <X size={10} />
            </button>
          </span>
        ))}
      </div>
      <div className="flex items-center gap-2">
        <input
          type="text"
          placeholder="group name…"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); add() } }}
          className="font-mono text-xs px-2 py-1"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-primary)',
            borderRadius: '3px',
            outline: 'none',
            width: '160px',
          }}
        />
        <button
          type="button"
          onClick={add}
          className="flex items-center gap-1 font-mono text-xs px-2 py-1"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: 'pointer',
          }}
        >
          <Plus size={11} />
          Add
        </button>
      </div>
    </div>
  )
}

// ---- Main panel ----

export interface SSOPanelProps {
  nodeId: string
  containerName: string
}

export function SSOPanel({ nodeId, containerName }: SSOPanelProps) {
  const { data, isLoading } = useSSOConfigs()
  const { mutate: upsert, isPending: isSaving, error: saveError, reset: resetSave } = useUpsertSSO()
  const { mutate: del, isPending: isDeleting } = useDeleteSSO()

  const existing = data?.configs.find(
    (c) => c.node_id === nodeId && c.container_name === containerName,
  ) ?? null

  // Form state
  const [enabled, setEnabled] = useState(false)
  const [method, setMethod] = useState<SSOMethod>('local')
  const [providerUrl, setProviderUrl] = useState('')
  const [clientId, setClientId] = useState('')
  const [secretMode, setSecretMode] = useState<'keep' | 'replace' | 'clear'>('keep')
  const [secretValue, setSecretValue] = useState('')
  const [groups, setGroups] = useState<string[]>([])
  const [durationHours, setDurationHours] = useState(8)

  // Sync form from loaded config
  useEffect(() => {
    if (existing) {
      setEnabled(existing.enabled)
      setMethod(existing.method)
      setProviderUrl(existing.provider_url ?? '')
      setClientId(existing.client_id ?? '')
      setGroups(existing.allowed_groups ?? [])
      setDurationHours(Math.round(existing.session_duration_secs / 3600) || 8)
      setSecretMode('keep')
      setSecretValue('')
    } else {
      setEnabled(false)
      setMethod('local')
      setProviderUrl('')
      setClientId('')
      setGroups([])
      setDurationHours(8)
      setSecretMode('keep')
      setSecretValue('')
    }
  }, [existing])

  function buildRequest(): SSOUpsertRequest {
    const req: SSOUpsertRequest = {
      node_id: nodeId,
      container_name: containerName,
      enabled,
      method,
      allowed_groups: groups,
      session_duration_secs: durationHours * 3600,
    }
    if (needsProvider(method)) {
      req.provider_url = providerUrl
      req.client_id = clientId
    }
    if (method === 'oidc') {
      if (secretMode === 'replace') req.client_secret = secretValue
      else if (secretMode === 'clear') req.client_secret = ''
      // 'keep' → omit client_secret
    }
    return req
  }

  function handleSave(e: React.FormEvent) {
    e.preventDefault()
    resetSave()
    upsert(buildRequest())
  }

  function handleRemove() {
    if (!existing) return
    del(existing.id)
  }

  const errorMsg = saveError
    ? (saveError as ApiError).status === 400
      ? ((saveError as ApiError).body as { error?: string })?.error === 'invalid_config'
        ? 'Invalid config — check method, provider URL, or credentials.'
        : 'Save failed — bad request.'
      : 'Save failed — server error.'
    : null

  return (
    <div
      className="flex flex-col gap-3"
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        padding: '16px',
        maxWidth: '640px',
      }}
    >
      {/* Header */}
      <div className="flex items-center gap-2">
        <ShieldCheck size={13} style={{ color: 'var(--text-muted)' }} />
        <span
          className="text-xs font-medium uppercase tracking-wider"
          style={{ color: 'var(--text-muted)' }}
        >
          SSO Passthrough
        </span>
      </div>

      {/* Config-only notice */}
      <div
        className="font-mono text-xs px-3 py-2"
        style={{
          background: 'rgba(240,160,32,0.08)',
          border: '1px solid rgba(240,160,32,0.3)',
          borderRadius: '3px',
          color: 'var(--text-muted)',
          lineHeight: '1.5',
        }}
      >
        Configuration only — Stratum stores these settings; the enforcing auth gateway is a
        planned follow-on. Settings are not yet applied to live traffic.
      </div>

      {isLoading && (
        <div className="flex items-center gap-2 py-2">
          <Loader size={12} className="animate-spin" style={{ color: 'var(--accent)' }} />
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading…</span>
        </div>
      )}

      {!isLoading && (
        <form onSubmit={handleSave} className="flex flex-col gap-3">
          {/* Enable toggle */}
          <label className="flex items-center gap-2 cursor-pointer select-none w-fit">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              style={{ accentColor: 'var(--accent)', width: '14px', height: '14px' }}
            />
            <span className="font-mono text-xs" style={{ color: 'var(--text-secondary)' }}>
              Enable SSO passthrough
            </span>
          </label>

          {/* Method select */}
          <div className="flex flex-col gap-1">
            <label className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Method
            </label>
            <select
              value={method}
              onChange={(e) => { setMethod(e.target.value as SSOMethod); resetSave() }}
              className="font-mono text-xs px-2 py-1"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-primary)',
                borderRadius: '3px',
                width: '220px',
              }}
            >
              {METHODS.map((m) => (
                <option key={m} value={m}>{METHOD_LABELS[m]}</option>
              ))}
            </select>
          </div>

          {/* Provider URL + Client ID (oidc / forward) */}
          {needsProvider(method) && (
            <>
              <div className="flex flex-col gap-1">
                <label className="text-xs" style={{ color: 'var(--text-muted)' }}>
                  <LogIn size={11} style={{ display: 'inline', marginRight: '4px' }} />
                  Provider URL
                </label>
                <input
                  type="url"
                  value={providerUrl}
                  onChange={(e) => setProviderUrl(e.target.value)}
                  placeholder="https://auth.example.com"
                  className="font-mono text-xs px-2 py-1"
                  style={{
                    background: 'var(--bg-elevated)',
                    border: '1px solid var(--border-default)',
                    color: 'var(--text-primary)',
                    borderRadius: '3px',
                    width: '100%',
                    maxWidth: '360px',
                    outline: 'none',
                  }}
                />
              </div>

              <div className="flex flex-col gap-1">
                <label className="text-xs" style={{ color: 'var(--text-muted)' }}>
                  Client ID
                </label>
                <input
                  type="text"
                  value={clientId}
                  onChange={(e) => setClientId(e.target.value)}
                  placeholder="my-client-id"
                  className="font-mono text-xs px-2 py-1"
                  style={{
                    background: 'var(--bg-elevated)',
                    border: '1px solid var(--border-default)',
                    color: 'var(--text-primary)',
                    borderRadius: '3px',
                    width: '100%',
                    maxWidth: '360px',
                    outline: 'none',
                  }}
                />
              </div>
            </>
          )}

          {/* Client secret (oidc only) */}
          {method === 'oidc' && (
            <div className="flex flex-col gap-1.5">
              <label className="text-xs" style={{ color: 'var(--text-muted)' }}>
                <KeyRound size={11} style={{ display: 'inline', marginRight: '4px' }} />
                Client secret
              </label>
              {existing?.has_client_secret && secretMode === 'keep' && (
                <span className="font-mono text-xs" style={{ color: 'var(--status-ok)' }}>
                  Secret set
                </span>
              )}
              <div className="flex items-center gap-2 flex-wrap">
                {(['keep', 'replace', 'clear'] as const).map((m) => (
                  <button
                    key={m}
                    type="button"
                    onClick={() => setSecretMode(m)}
                    className="font-mono text-xs px-2 py-0.5"
                    style={{
                      background: secretMode === m ? 'var(--accent-glow, rgba(64,140,255,0.15))' : 'var(--bg-elevated)',
                      border: `1px solid ${secretMode === m ? 'var(--accent)' : 'var(--border-default)'}`,
                      color: secretMode === m ? 'var(--accent)' : 'var(--text-muted)',
                      borderRadius: '3px',
                      cursor: 'pointer',
                    }}
                  >
                    {m === 'keep' ? 'Keep existing' : m === 'replace' ? 'Replace' : 'Clear'}
                  </button>
                ))}
              </div>
              {secretMode === 'replace' && (
                <input
                  type="password"
                  value={secretValue}
                  onChange={(e) => setSecretValue(e.target.value)}
                  placeholder="new client secret"
                  className="font-mono text-xs px-2 py-1"
                  style={{
                    background: 'var(--bg-elevated)',
                    border: '1px solid var(--border-default)',
                    color: 'var(--text-primary)',
                    borderRadius: '3px',
                    width: '100%',
                    maxWidth: '360px',
                    outline: 'none',
                  }}
                />
              )}
            </div>
          )}

          {/* Allowed groups */}
          <div className="flex flex-col gap-1.5">
            <label className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Allowed groups (leave empty to allow all authenticated users)
            </label>
            <GroupList groups={groups} onChange={setGroups} />
          </div>

          {/* Session duration */}
          <div className="flex flex-col gap-1">
            <label className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Session duration (hours)
            </label>
            <input
              type="number"
              min={1}
              max={720}
              value={durationHours}
              onChange={(e) => setDurationHours(Math.max(1, parseInt(e.target.value) || 1))}
              className="font-mono text-xs px-2 py-1"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-primary)',
                borderRadius: '3px',
                width: '96px',
                outline: 'none',
              }}
            />
          </div>

          {/* Inline error */}
          {errorMsg && (
            <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
              {errorMsg}
            </span>
          )}

          {/* Actions */}
          <div className="flex items-center gap-2 pt-1">
            <button
              type="submit"
              disabled={isSaving}
              className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
              style={{
                background: 'var(--accent-glow, rgba(64,140,255,0.15))',
                border: '1px solid var(--accent-dim, rgba(64,140,255,0.5))',
                color: isSaving ? 'var(--text-muted)' : 'var(--accent)',
                borderRadius: '3px',
                cursor: isSaving ? 'not-allowed' : 'pointer',
                opacity: isSaving ? 0.6 : 1,
              }}
            >
              {isSaving ? <Loader size={12} className="animate-spin" /> : <Save size={12} />}
              Save
            </button>

            {existing && (
              <button
                type="button"
                disabled={isDeleting}
                onClick={handleRemove}
                className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
                style={{
                  background: 'rgba(232,64,64,0.08)',
                  border: '1px solid var(--status-error)',
                  color: isDeleting ? 'var(--text-muted)' : 'var(--status-error)',
                  borderRadius: '3px',
                  cursor: isDeleting ? 'not-allowed' : 'pointer',
                  opacity: isDeleting ? 0.6 : 1,
                }}
              >
                {isDeleting ? <Loader size={12} className="animate-spin" /> : <X size={12} />}
                Remove
              </button>
            )}
          </div>
        </form>
      )}
    </div>
  )
}
