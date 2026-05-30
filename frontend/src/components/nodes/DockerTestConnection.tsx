import { useState } from 'react'
import { Box, Check, AlertTriangle, Loader } from 'lucide-react'
import { ApiError } from '../../lib/api'
import { useProbePreview } from '../../lib/api/nodes'
import type { ProbePreviewRequest, PreviewResult } from '../../types/api'

interface DockerTestConnectionProps {
  /**
   * Builds the probe-preview body (host + creds + docker_endpoint + docker TLS).
   * Return null to signal the inputs are incomplete — the button stays disabled
   * and the reason is shown.
   */
  buildBody: () => ProbePreviewRequest | null
  /** Human-readable reason the test can't run yet (shown when buildBody() is null). */
  disabledReason?: string
}

/**
 * Compact "Test connection" control for the Docker config section. Calls
 * /api/nodes/probe-preview and reports only Docker reachability — green when
 * docker_version is returned, amber with the probe_errors.docker /
 * probe_hints.docker hint on failure. Independent of the wizard's main probe
 * step so it can be reused on the edit-node modal.
 */
export function DockerTestConnection({ buildBody, disabledReason }: DockerTestConnectionProps) {
  const probe = useProbePreview()
  const [result, setResult] = useState<PreviewResult | null>(null)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const canTest = !disabledReason

  async function handleTest() {
    setResult(null)
    setErrorMsg(null)
    const body = buildBody()
    if (!body) return
    try {
      const r = await probe.mutateAsync(body)
      setResult(r)
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 403) {
          setErrorMsg('Admin role required to test connections.')
        } else if (err.status === 429) {
          setErrorMsg('Too many probe requests. Wait a moment and try again.')
        } else {
          const b = err.body as { error?: string }
          setErrorMsg(b.error ?? 'Test failed.')
        }
      } else {
        setErrorMsg('Test failed.')
      }
    }
  }

  const dockerErr = result?.probe_errors?.docker
  const dockerHint = result?.probe_hints?.docker
  const reachable = Boolean(result?.docker_version)

  return (
    <div className="flex flex-col gap-2">
      <button
        type="button"
        onClick={() => void handleTest()}
        disabled={!canTest || probe.isPending}
        title={disabledReason}
        className="self-start flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium disabled:opacity-40"
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          color: 'var(--text-secondary)',
          borderRadius: '3px',
          cursor: canTest && !probe.isPending ? 'pointer' : 'not-allowed',
        }}
      >
        {probe.isPending ? (
          <Loader size={12} className="animate-spin" />
        ) : (
          <Box size={12} />
        )}
        {probe.isPending ? 'Testing...' : 'Test connection'}
      </button>

      {disabledReason && !probe.isPending && !result && !errorMsg && (
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          {disabledReason}
        </span>
      )}

      {/* Request-level failure (auth / rate-limit / network) */}
      {errorMsg && !probe.isPending && (
        <div
          className="flex items-start gap-2 px-3 py-2"
          style={{
            backgroundColor: 'rgba(232,64,64,0.08)',
            border: '1px solid rgba(232,64,64,0.3)',
            borderRadius: '3px',
          }}
        >
          <AlertTriangle size={12} style={{ color: 'var(--status-error)', marginTop: '1px', flexShrink: 0 }} />
          <span className="text-xs" style={{ color: 'var(--status-error)' }}>
            {errorMsg}
          </span>
        </div>
      )}

      {/* Docker reachable */}
      {result && !probe.isPending && reachable && (
        <div
          className="flex items-center gap-2 px-3 py-2"
          style={{
            backgroundColor: 'rgba(64,200,120,0.08)',
            border: '1px solid rgba(64,200,120,0.35)',
            borderRadius: '3px',
          }}
        >
          <Check size={12} style={{ color: 'var(--status-ok)', flexShrink: 0 }} />
          <span className="text-xs" style={{ color: 'var(--status-ok)' }}>
            Docker reachable — v{result.docker_version}
          </span>
        </div>
      )}

      {/* Docker NOT reachable — show the docker-specific hint/error if present */}
      {result && !probe.isPending && !reachable && (
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
            Docker not reachable
            {(dockerHint || dockerErr) && ` — ${dockerHint ?? dockerErr}`}
          </span>
        </div>
      )}
    </div>
  )
}
