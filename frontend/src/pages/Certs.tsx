import { useState } from 'react'
import { ShieldCheck, RefreshCw, Loader, ChevronDown, ChevronRight } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { useCan } from '../lib/roles'
import { useCerts, useRescanCerts } from '../lib/api/certs'
import { useNodes } from '../lib/api/nodes'
import type { CertInfo } from '../types/api'

// ---- Helpers ----

function daysRemaining(notAfter: string | undefined): number | null {
  if (!notAfter) return null
  const ms = new Date(notAfter).getTime() - Date.now()
  return Math.floor(ms / 86_400_000)
}

function resolveNodeName(nodeId: string, nodesMap: Map<string, string>): string {
  return nodesMap.get(nodeId) ?? nodeId
}

// ---- Days chip ----

interface DaysChipProps {
  days: number | null
}

function DaysChip({ days }: DaysChipProps) {
  if (days === null) {
    return (
      <span
        className="font-mono text-xs px-1.5 py-0.5"
        style={{
          color: 'var(--text-muted)',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
        }}
      >
        —
      </span>
    )
  }

  if (days <= 0) {
    return (
      <span
        className="font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
        style={{
          color: 'var(--status-error)',
          background: 'rgba(220,60,60,0.12)',
          border: '1px solid rgba(220,60,60,0.4)',
          borderRadius: '3px',
          whiteSpace: 'nowrap',
        }}
      >
        EXPIRED
      </span>
    )
  }

  if (days < 7) {
    return (
      <span
        className="font-mono text-xs px-1.5 py-0.5"
        style={{
          color: 'var(--status-error)',
          background: 'rgba(220,60,60,0.12)',
          border: '1px solid rgba(220,60,60,0.4)',
          borderRadius: '3px',
          whiteSpace: 'nowrap',
        }}
      >
        {days}d
      </span>
    )
  }

  if (days <= 30) {
    return (
      <span
        className="font-mono text-xs px-1.5 py-0.5"
        style={{
          color: 'var(--status-warn)',
          background: 'rgba(240,160,32,0.12)',
          border: '1px solid rgba(240,160,32,0.4)',
          borderRadius: '3px',
          whiteSpace: 'nowrap',
        }}
      >
        {days}d
      </span>
    )
  }

  return (
    <span
      className="font-mono text-xs px-1.5 py-0.5"
      style={{
        color: 'var(--status-ok)',
        background: 'rgba(64,200,120,0.1)',
        border: '1px solid rgba(64,200,120,0.3)',
        borderRadius: '3px',
        whiteSpace: 'nowrap',
      }}
    >
      {days}d
    </span>
  )
}

// ---- Detail panel (expanded row) ----

interface DetailPanelProps {
  cert: CertInfo
}

function DetailPanel({ cert }: DetailPanelProps) {
  const cellMuted: React.CSSProperties = {
    color: 'var(--text-muted)',
    fontSize: '11px',
    paddingRight: '12px',
    paddingBottom: '4px',
    whiteSpace: 'nowrap',
    verticalAlign: 'top',
  }
  const cellVal: React.CSSProperties = {
    color: 'var(--text-secondary)',
    fontSize: '11px',
    fontFamily: 'monospace',
    paddingBottom: '4px',
    wordBreak: 'break-all',
  }

  return (
    <div
      className="px-4 py-3"
      style={{
        background: 'var(--bg-elevated)',
        borderTop: '1px solid var(--border-subtle)',
      }}
    >
      <table style={{ borderCollapse: 'collapse', width: '100%' }}>
        <tbody>
          <tr>
            <td style={cellMuted}>Path</td>
            <td style={cellVal}>{cert.path || '—'}</td>
          </tr>
          <tr>
            <td style={cellMuted}>Issuer</td>
            <td style={cellVal}>{cert.issuer || '—'}</td>
          </tr>
          <tr>
            <td style={cellMuted}>Not Before</td>
            <td style={cellVal}>
              {cert.not_before ? new Date(cert.not_before).toLocaleString() : '—'}
            </td>
          </tr>
          <tr>
            <td style={cellMuted}>Not After</td>
            <td style={cellVal}>
              {cert.not_after ? new Date(cert.not_after).toLocaleString() : '—'}
            </td>
          </tr>
          <tr>
            <td style={cellMuted}>Last Checked</td>
            <td style={cellVal}>{new Date(cert.last_checked).toLocaleString()}</td>
          </tr>
          {cert.sans.length > 0 && (
            <tr>
              <td style={{ ...cellMuted, verticalAlign: 'top', paddingTop: '2px' }}>SANs</td>
              <td style={{ ...cellVal, paddingTop: '2px' }}>
                <div className="flex flex-wrap gap-1">
                  {cert.sans.map((san) => (
                    <span
                      key={san}
                      className="font-mono text-xs px-1.5 py-0.5"
                      style={{
                        background: 'var(--bg-surface)',
                        border: '1px solid var(--border-subtle)',
                        borderRadius: '3px',
                        color: 'var(--text-secondary)',
                        fontSize: '10px',
                      }}
                    >
                      {san}
                    </span>
                  ))}
                </div>
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

// ---- Table row ----

interface CertRowProps {
  cert: CertInfo
  nodeName: string
}

function CertRow({ cert, nodeName }: CertRowProps) {
  const [expanded, setExpanded] = useState(false)
  const days = daysRemaining(cert.not_after)

  const cellStyle: React.CSSProperties = {
    borderBottom: expanded ? 'none' : '1px solid var(--border-subtle)',
  }

  return (
    <>
      <tr
        style={{ cursor: 'pointer' }}
        onClick={() => setExpanded((v) => !v)}
      >
        <td
          className="px-3 py-2"
          style={{ ...cellStyle, width: '20px', color: 'var(--text-muted)' }}
        >
          {expanded
            ? <ChevronDown size={12} />
            : <ChevronRight size={12} />}
        </td>
        <td
          className="px-3 py-2 font-mono text-xs"
          style={{ color: 'var(--text-primary)', ...cellStyle }}
        >
          {cert.domain}
        </td>
        <td
          className="px-3 py-2 text-xs"
          style={{ color: 'var(--text-secondary)', ...cellStyle }}
        >
          {cert.issuer || '—'}
        </td>
        <td
          className="px-3 py-2 text-xs"
          style={{ color: 'var(--text-secondary)', ...cellStyle }}
        >
          {nodeName}
        </td>
        <td
          className="px-3 py-2 text-xs"
          style={{ color: 'var(--text-muted)', ...cellStyle, whiteSpace: 'nowrap' }}
        >
          {cert.not_after ? new Date(cert.not_after).toLocaleDateString() : '—'}
        </td>
        <td className="px-3 py-2" style={cellStyle}>
          <DaysChip days={days} />
        </td>
        <td
          className="px-3 py-2 font-mono text-xs"
          style={{ color: 'var(--text-muted)', ...cellStyle }}
        >
          {cert.source}
        </td>
      </tr>
      {expanded && (
        <tr>
          <td
            colSpan={7}
            style={{ borderBottom: '1px solid var(--border-subtle)', padding: 0 }}
          >
            <DetailPanel cert={cert} />
          </td>
        </tr>
      )}
    </>
  )
}

// ---- Summary bar ----

function SummaryBar({ certs }: { certs: CertInfo[] }) {
  const expired = certs.filter((c) => {
    const d = daysRemaining(c.not_after)
    return d !== null && d <= 0
  }).length
  const critical = certs.filter((c) => {
    const d = daysRemaining(c.not_after)
    return d !== null && d > 0 && d < 7
  }).length
  const warn = certs.filter((c) => {
    const d = daysRemaining(c.not_after)
    return d !== null && d >= 7 && d <= 30
  }).length
  const ok = certs.filter((c) => {
    const d = daysRemaining(c.not_after)
    return d !== null && d > 30
  }).length

  return (
    <div className="flex items-center gap-4 mb-5">
      {expired > 0 && (
        <span className="text-xs font-medium" style={{ color: 'var(--status-error)' }}>
          {expired} expired
        </span>
      )}
      {critical > 0 && (
        <span className="text-xs font-medium" style={{ color: 'var(--status-error)' }}>
          {critical} critical (&lt;7d)
        </span>
      )}
      {warn > 0 && (
        <span className="text-xs" style={{ color: 'var(--status-warn)' }}>
          {warn} expiring soon
        </span>
      )}
      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
        {ok} healthy
      </span>
    </div>
  )
}

// ---- Rescan button ----

interface RescanButtonProps {
  isPending: boolean
  isLoading: boolean
  onRescan: () => void
}

function RescanButton({ isPending, isLoading, onRescan }: RescanButtonProps) {
  return (
    <button
      type="button"
      onClick={onRescan}
      disabled={isPending || isLoading}
      className="flex items-center gap-1.5 text-xs px-3 py-1.5"
      style={{
        backgroundColor: isPending ? 'rgba(74,82,104,0.2)' : 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        color: isPending ? 'var(--text-muted)' : 'var(--text-secondary)',
        borderRadius: '3px',
        cursor: isPending || isLoading ? 'default' : 'pointer',
        opacity: isLoading ? 0.5 : 1,
      }}
    >
      {isPending ? (
        <>
          <Loader size={11} className="animate-spin" />
          Scanning nodes…
        </>
      ) : (
        <>
          <RefreshCw size={11} />
          Rescan
        </>
      )}
    </button>
  )
}

// ---- Main page ----

const TABLE_COLS = ['', 'Domain', 'Issuer', 'Node', 'Expires', 'Days', 'Source']

export default function Certs() {
  const { isAdmin } = useCan()
  const { data, isLoading } = useCerts()
  const { mutate: rescan, isPending: isRescanning } = useRescanCerts()
  const { data: nodesData } = useNodes()

  const nodesMap = new Map<string, string>(
    (nodesData ?? []).map((n) => [n.id, n.name]),
  )

  const certs = data?.certs ?? []

  if (!isAdmin) {
    return (
      <AppShell>
        <div
          className="flex items-center justify-center flex-1 text-xs"
          style={{ color: 'var(--text-muted)' }}
        >
          Admin access required.
        </div>
      </AppShell>
    )
  }

  return (
    <AppShell>
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full p-6"
        style={{ maxWidth: '1100px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-2">
            <ShieldCheck size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1
              className="text-sm font-medium uppercase tracking-wider"
              style={{ color: 'var(--text-primary)' }}
            >
              Certificate Management
            </h1>
          </div>
          <RescanButton
            isPending={isRescanning}
            isLoading={isLoading}
            onRescan={() => rescan()}
          />
        </div>

        {/* Loading state */}
        {isLoading && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading certificates…
            </span>
          </div>
        )}

        {/* Content */}
        {!isLoading && (
          <>
            {certs.length === 0 ? (
              <div
                className="flex flex-col gap-3 px-4 py-6 text-xs"
                style={{
                  color: 'var(--text-muted)',
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                }}
              >
                <p>
                  No certificates found. Stratum scans{' '}
                  <span className="font-mono" style={{ color: 'var(--text-secondary)' }}>
                    /etc/letsencrypt/live
                  </span>
                  ,{' '}
                  <span className="font-mono" style={{ color: 'var(--text-secondary)' }}>
                    /etc/ssl
                  </span>
                  , and{' '}
                  <span className="font-mono" style={{ color: 'var(--text-secondary)' }}>
                    /opt/certs
                  </span>{' '}
                  on each node.
                </p>
                <div>
                  <RescanButton
                    isPending={isRescanning}
                    isLoading={false}
                    onRescan={() => rescan()}
                  />
                </div>
              </div>
            ) : (
              <>
                <SummaryBar certs={certs} />

                <div
                  style={{
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                    overflowX: 'auto',
                  }}
                >
                  <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                    <thead>
                      <tr>
                        {TABLE_COLS.map((col, i) => (
                          <th
                            key={`${col}-${i}`}
                            className="px-3 py-2 text-left text-xs uppercase tracking-wider font-medium"
                            style={{
                              color: 'var(--text-muted)',
                              borderBottom: '1px solid var(--border-subtle)',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {col}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {certs.map((cert) => (
                        <CertRow
                          key={cert.id}
                          cert={cert}
                          nodeName={resolveNodeName(cert.node_id, nodesMap)}
                        />
                      ))}
                    </tbody>
                  </table>
                </div>
              </>
            )}
          </>
        )}
      </div>
    </AppShell>
  )
}
