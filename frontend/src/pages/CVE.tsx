import { useState } from 'react'
import {
  Bug,
  RefreshCw,
  ChevronDown,
  ChevronRight,
  Loader,
  ExternalLink,
  ShieldAlert,
  ShieldCheck,
  Clock,
  Trash2,
  Plus,
} from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { useTree } from '../lib/api/tree'
import {
  useCVEScans,
  useCVEDetail,
  useScanContainer,
  useCVEStatus,
  useBulkScan,
  useCVESchedules,
  useCreateCveSchedule,
  useToggleCveSchedule,
  useDeleteCveSchedule,
} from '../lib/api/cve'
import { ApiError } from '../lib/api'
import type { ImageScan, CVEVuln, CveSchedule } from '../types/api'

// ---- Types ----

type SeverityFilter = 'all' | 'critical' | 'high' | 'medium' | 'low'

// ---- Helpers ----

const SEVERITY_ORDER: Record<string, number> = {
  critical: 4,
  high: 3,
  medium: 2,
  low: 1,
  unknown: 0,
}

function severityColor(sev: string): string {
  const s = sev.toLowerCase()
  if (s === 'critical') return 'var(--status-error)'
  if (s === 'high') return 'var(--status-warn)'
  if (s === 'medium') return '#d4a017'
  return 'var(--text-muted)'
}

function severityBg(sev: string): string {
  const s = sev.toLowerCase()
  if (s === 'critical') return 'rgba(232,64,64,0.12)'
  if (s === 'high') return 'rgba(240,160,32,0.12)'
  if (s === 'medium') return 'rgba(212,160,23,0.12)'
  return 'rgba(74,82,104,0.15)'
}

function severityBorder(sev: string): string {
  const s = sev.toLowerCase()
  if (s === 'critical') return 'rgba(232,64,64,0.35)'
  if (s === 'high') return 'rgba(240,160,32,0.4)'
  if (s === 'medium') return 'rgba(212,160,23,0.35)'
  return 'var(--border-subtle)'
}

function formatDate(iso: string): string {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return iso
  }
}

function isCveId(id: string): boolean {
  return /^CVE-\d{4}-\d+$/i.test(id)
}

function sortScans(scans: ImageScan[]): ImageScan[] {
  return [...scans].sort((a, b) => {
    // Newest scans first; fall back to severity for same-timestamp ties.
    const at = Date.parse(a.scanned_at) || 0
    const bt = Date.parse(b.scanned_at) || 0
    if (bt !== at) return bt - at
    if (b.critical !== a.critical) return b.critical - a.critical
    return b.high - a.high
  })
}

function passesTableFilter(scan: ImageScan, filter: SeverityFilter): boolean {
  if (filter === 'all') return true
  if (filter === 'critical') return scan.critical > 0
  if (filter === 'high') return scan.high > 0
  if (filter === 'medium') return scan.medium > 0
  if (filter === 'low') return scan.low > 0
  return true
}

type TimeWindow = 'day' | 'week' | 'month' | 'all'

const TIME_WINDOWS: { value: TimeWindow; label: string; ms: number }[] = [
  { value: 'day', label: 'Last 24h', ms: 24 * 60 * 60 * 1000 },
  { value: 'week', label: 'Last 7d', ms: 7 * 24 * 60 * 60 * 1000 },
  { value: 'month', label: 'Last 30d', ms: 30 * 24 * 60 * 60 * 1000 },
  { value: 'all', label: 'All time', ms: 0 },
]

function passesTimeFilter(scan: ImageScan, window: TimeWindow, now: number): boolean {
  const w = TIME_WINDOWS.find((t) => t.value === window)
  if (!w || w.ms === 0) return true // 'all'
  const t = Date.parse(scan.scanned_at)
  if (Number.isNaN(t)) return true // undated scans never hidden
  return now - t <= w.ms
}

// ---- Sub-components ----

function SeverityBadge({ count, severity }: { count: number; severity: string }) {
  if (count === 0) return null
  return (
    <span
      className="font-mono text-xs px-1.5 py-0.5"
      style={{
        background: severityBg(severity),
        border: `1px solid ${severityBorder(severity)}`,
        color: severityColor(severity),
        borderRadius: '3px',
        fontSize: '12px',
        marginRight: '4px',
      }}
    >
      {severity.toUpperCase()[0]} {count}
    </span>
  )
}

interface CVEDetailPanelProps {
  digest: string
}

function CVEDetailPanel({ digest }: CVEDetailPanelProps) {
  const [detailFilter, setDetailFilter] = useState<SeverityFilter>('all')
  const { data, isLoading } = useCVEDetail(digest, true)

  const vulns: CVEVuln[] = data?.vulns ?? []
  const filtered = vulns.filter((v) => {
    if (detailFilter === 'all') return true
    return v.severity.toLowerCase() === detailFilter
  })
  const sorted = [...filtered].sort(
    (a, b) => (SEVERITY_ORDER[b.severity.toLowerCase()] ?? 0) - (SEVERITY_ORDER[a.severity.toLowerCase()] ?? 0),
  )

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-elevated)',
        borderTop: '1px solid var(--border-subtle)',
        padding: '12px 16px',
      }}
    >
      {isLoading ? (
        <div className="flex items-center gap-2 py-2">
          <Loader size={12} className="animate-spin" style={{ color: 'var(--accent)' }} />
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
            Loading CVEs…
          </span>
        </div>
      ) : (
        <>
          <div className="flex items-center gap-3 mb-3">
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              {vulns.length} vulnerabilities
            </span>
            <select
              value={detailFilter}
              onChange={(e) => setDetailFilter(e.target.value as SeverityFilter)}
              className="font-mono text-xs px-2 py-0.5"
              style={{
                background: 'var(--bg-surface)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <option value="all">All severities</option>
              <option value="critical">Critical only</option>
              <option value="high">High only</option>
              <option value="medium">Medium only</option>
              <option value="low">Low only</option>
            </select>
          </div>

          {sorted.length === 0 ? (
            <div className="text-xs py-2" style={{ color: 'var(--text-muted)' }}>
              No vulnerabilities match the filter.
            </div>
          ) : (
            <div
              style={{
                border: '1px solid var(--border-subtle)',
                borderRadius: '3px',
                overflowX: 'auto',
              }}
            >
              <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead>
                  <tr>
                    {['CVE', 'Severity', 'Package', 'Installed', 'Fixed', 'Title'].map((col) => (
                      <th
                        key={col}
                        className="px-3 py-1.5 text-left text-xs uppercase tracking-wider font-medium"
                        style={{
                          color: 'var(--text-muted)',
                          borderBottom: '1px solid var(--border-subtle)',
                          whiteSpace: 'nowrap',
                          fontSize: '12px',
                        }}
                      >
                        {col}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {sorted.map((vuln, idx) => (
                    <tr key={`${vuln.cve_id}-${idx}`}>
                      <td
                        className="px-3 py-1.5 font-mono text-xs"
                        style={{
                          color: 'var(--accent)',
                          borderBottom: '1px solid var(--border-subtle)',
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {isCveId(vuln.cve_id) ? (
                          <a
                            href={`https://nvd.nist.gov/vuln/detail/${vuln.cve_id}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="flex items-center gap-1"
                            style={{ color: 'var(--accent)', textDecoration: 'none' }}
                          >
                            {vuln.cve_id}
                            <ExternalLink size={10} />
                          </a>
                        ) : (
                          <span>{vuln.cve_id}</span>
                        )}
                      </td>
                      <td
                        className="px-3 py-1.5"
                        style={{ borderBottom: '1px solid var(--border-subtle)', whiteSpace: 'nowrap' }}
                      >
                        <span
                          className="font-mono text-xs px-1.5 py-0.5 uppercase"
                          style={{
                            background: severityBg(vuln.severity),
                            border: `1px solid ${severityBorder(vuln.severity)}`,
                            color: severityColor(vuln.severity),
                            borderRadius: '3px',
                            fontSize: '12px',
                          }}
                        >
                          {vuln.severity}
                        </span>
                      </td>
                      <td
                        className="px-3 py-1.5 font-mono text-xs"
                        style={{
                          color: 'var(--text-primary)',
                          borderBottom: '1px solid var(--border-subtle)',
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {vuln.package}
                      </td>
                      <td
                        className="px-3 py-1.5 font-mono text-xs"
                        style={{
                          color: 'var(--text-secondary)',
                          borderBottom: '1px solid var(--border-subtle)',
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {vuln.installed_version}
                      </td>
                      <td
                        className="px-3 py-1.5 font-mono text-xs"
                        style={{
                          color: vuln.fixed_version ? 'var(--status-ok)' : 'var(--text-muted)',
                          borderBottom: '1px solid var(--border-subtle)',
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {vuln.fixed_version || 'no fix'}
                      </td>
                      <td
                        className="px-3 py-1.5 text-xs"
                        style={{
                          color: 'var(--text-secondary)',
                          borderBottom: '1px solid var(--border-subtle)',
                          maxWidth: '280px',
                        }}
                      >
                        <span className="truncate block" title={vuln.title}>
                          {vuln.title}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </div>
  )
}

interface ScanRowProps {
  scan: ImageScan
  isExpanded: boolean
  onToggle: () => void
}

function ScanRow({ scan, isExpanded, onToggle }: ScanRowProps) {
  const total = scan.critical + scan.high + scan.medium + scan.low + scan.unknown
  return (
    <>
      <tr
        style={{ cursor: 'pointer' }}
        onClick={onToggle}
      >
        <td
          className="px-3 py-2 font-mono text-xs"
          style={{ color: 'var(--text-primary)', borderBottom: isExpanded ? 'none' : '1px solid var(--border-subtle)' }}
        >
          <div className="flex items-center gap-1.5">
            {isExpanded ? (
              <ChevronDown size={11} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
            ) : (
              <ChevronRight size={11} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
            )}
            <span className="truncate" title={scan.image} style={{ maxWidth: '280px', display: 'block' }}>
              {scan.image}
            </span>
          </div>
        </td>
        <td
          className="px-3 py-2 text-xs"
          style={{ color: 'var(--text-muted)', borderBottom: isExpanded ? 'none' : '1px solid var(--border-subtle)', whiteSpace: 'nowrap' }}
        >
          {formatDate(scan.scanned_at)}
        </td>
        <td
          className="px-3 py-2"
          style={{ borderBottom: isExpanded ? 'none' : '1px solid var(--border-subtle)' }}
        >
          <div className="flex items-center gap-1 flex-wrap">
            <SeverityBadge count={scan.critical} severity="critical" />
            <SeverityBadge count={scan.high} severity="high" />
            <SeverityBadge count={scan.medium} severity="medium" />
            <SeverityBadge count={scan.low} severity="low" />
            {total === 0 && (
              <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
                clean
              </span>
            )}
          </div>
        </td>
      </tr>
      {isExpanded && (
        <tr>
          <td colSpan={3} style={{ padding: 0, borderBottom: '1px solid var(--border-subtle)' }}>
            <CVEDetailPanel digest={scan.image_digest} />
          </td>
        </tr>
      )}
    </>
  )
}

interface ScanTriggerProps {
  scannerAvailable: boolean
}

function ScanTrigger({ scannerAvailable }: ScanTriggerProps) {
  const { data: tree } = useTree()
  const { mutate: scanContainer, isPending, error, isSuccess, reset } = useScanContainer()
  const [selectedId, setSelectedId] = useState('')

  const containers = (tree?.nodes ?? []).flatMap((node) =>
    node.containers.map((c) => ({ ...c, nodeName: node.name })),
  )

  function errorMessage(): string | null {
    if (!error) return null
    if (error instanceof ApiError) {
      if (error.status === 409) return 'Scanner unavailable — Trivy is not configured on this host.'
      if (error.status === 502) return 'Scan failed — Trivy returned an error.'
      if (error.status === 404) return 'Container not found.'
    }
    return 'Scan failed.'
  }

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        padding: '12px 16px',
      }}
    >
      <div className="flex items-center gap-2 mb-3">
        <Bug size={12} style={{ color: 'var(--text-muted)' }} />
        <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
          Scan a Container Image
        </span>
      </div>

      <div className="flex items-center gap-2 flex-wrap">
        <select
          value={selectedId}
          onChange={(e) => { setSelectedId(e.target.value); reset() }}
          disabled={!scannerAvailable || isPending}
          className="font-mono text-xs px-2 py-1.5"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: scannerAvailable ? 'pointer' : 'not-allowed',
            opacity: scannerAvailable ? 1 : 0.5,
            minWidth: '220px',
          }}
        >
          <option value="">Select container…</option>
          {containers.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name} ({c.nodeName})
            </option>
          ))}
        </select>

        <button
          type="button"
          disabled={!scannerAvailable || isPending || !selectedId}
          onClick={() => { if (selectedId) scanContainer(selectedId) }}
          className="flex items-center gap-1.5 text-xs px-3 py-1.5"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: (!scannerAvailable || isPending || !selectedId) ? 'not-allowed' : 'pointer',
            opacity: (!scannerAvailable || !selectedId) ? 0.5 : 1,
          }}
        >
          {isPending ? (
            <>
              <Loader size={11} className="animate-spin" />
              Scanning…
            </>
          ) : (
            <>
              <RefreshCw size={11} />
              Scan
            </>
          )}
        </button>

        {isSuccess && !isPending && (
          <span className="text-xs" style={{ color: '#40c878' }}>
            Scan queued — results will appear above shortly.
          </span>
        )}
      </div>

      {errorMessage() && (
        <div
          className="mt-2 text-xs px-2 py-1.5"
          style={{
            background: 'rgba(232,64,64,0.07)',
            border: '1px solid rgba(232,64,64,0.2)',
            color: 'var(--status-error)',
            borderRadius: '3px',
          }}
        >
          {errorMessage()}
        </div>
      )}
    </div>
  )
}

// ---- Bulk scan trigger ----

interface BulkScanTriggerProps {
  scannerAvailable: boolean
}

function BulkScanTrigger({ scannerAvailable }: BulkScanTriggerProps) {
  const { data: tree } = useTree()
  const { mutate: bulkScan, isPending, data: results, reset } = useBulkScan()
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())

  const containers = (tree?.nodes ?? []).flatMap((node) =>
    node.containers.map((c) => ({ ...c, nodeName: node.name })),
  )

  function toggleContainer(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) { next.delete(id) } else { next.add(id) }
      return next
    })
    reset()
  }

  function selectAll() {
    setSelectedIds(new Set(containers.map((c) => c.id)))
    reset()
  }

  function clearAll() {
    setSelectedIds(new Set())
    reset()
  }

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        padding: '12px 16px',
      }}
    >
      <div className="flex items-center gap-2 mb-3">
        <Bug size={12} style={{ color: 'var(--text-muted)' }} />
        <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
          Scan Multiple Containers
        </span>
      </div>

      <div className="flex items-center gap-2 mb-2 flex-wrap">
        <button
          type="button"
          onClick={selectAll}
          className="text-xs px-2 py-1"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: 'pointer',
          }}
        >
          All
        </button>
        <button
          type="button"
          onClick={clearAll}
          className="text-xs px-2 py-1"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: 'pointer',
          }}
        >
          None
        </button>
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          {selectedIds.size} of {containers.length} selected
        </span>
      </div>

      <div
        className="mb-3"
        style={{
          maxHeight: '120px',
          overflowY: 'auto',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
        }}
      >
        {containers.map((c) => (
          <label
            key={c.id}
            className="flex items-center gap-2 px-3 py-1 cursor-pointer hover:bg-[var(--bg-elevated)]"
          >
            <input
              type="checkbox"
              checked={selectedIds.has(c.id)}
              onChange={() => toggleContainer(c.id)}
              disabled={!scannerAvailable || isPending}
              className="accent-[var(--accent)]"
            />
            <span className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
              {c.name}
            </span>
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              ({c.nodeName})
            </span>
          </label>
        ))}
      </div>

      <div className="flex items-center gap-2">
        <button
          type="button"
          disabled={!scannerAvailable || isPending || selectedIds.size === 0}
          onClick={() => bulkScan(Array.from(selectedIds))}
          className="flex items-center gap-1.5 text-xs px-3 py-1.5"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: (!scannerAvailable || isPending || selectedIds.size === 0) ? 'not-allowed' : 'pointer',
            opacity: (!scannerAvailable || selectedIds.size === 0) ? 0.5 : 1,
          }}
        >
          {isPending ? (
            <><Loader size={11} className="animate-spin" /> Scanning…</>
          ) : (
            <><RefreshCw size={11} /> Scan Selected</>
          )}
        </button>
      </div>

      {results && !isPending && (
        <div className="mt-3 text-xs" style={{ color: 'var(--text-muted)' }}>
          {results.results.filter((r) => !r.error).length} succeeded,{' '}
          {results.results.filter((r) => !!r.error).length} failed
          {results.results.some((r) => !!r.error) && (
            <ul className="mt-1">
              {results.results.filter((r) => !!r.error).map((r) => (
                <li key={r.container_id} style={{ color: 'var(--status-error)' }}>
                  {r.image}: {r.error}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  )
}

// ---- CVE schedules panel ----

const INTERVAL_OPTIONS = [
  { label: 'Every hour', seconds: 3600 },
  { label: 'Every 6 hours', seconds: 21600 },
  { label: 'Every 12 hours', seconds: 43200 },
  { label: 'Every day', seconds: 86400 },
  { label: 'Every 3 days', seconds: 259200 },
  { label: 'Every week', seconds: 604800 },
]

interface SchedulesPanelProps {
  isAdmin: boolean
}

function SchedulesPanel({ isAdmin }: SchedulesPanelProps) {
  const { data: tree } = useTree()
  const { data, isLoading } = useCVESchedules(isAdmin)
  const { mutate: createSchedule, isPending: isCreating } = useCreateCveSchedule()
  const { mutate: toggleSchedule } = useToggleCveSchedule()
  const { mutate: deleteSchedule } = useDeleteCveSchedule()

  const [targetType, setTargetType] = useState<'node' | 'container'>('node')
  const [targetId, setTargetId] = useState('')
  const [label, setLabel] = useState('')
  const [intervalSeconds, setIntervalSeconds] = useState(86400)

  const nodes = tree?.nodes ?? []
  const containers = nodes.flatMap((n) => n.containers.map((c) => ({ ...c, nodeName: n.name })))

  const targets = targetType === 'node'
    ? nodes.map((n) => ({ id: n.id, label: n.name }))
    : containers.map((c) => ({ id: c.id, label: `${c.name} (${c.nodeName})` }))

  function handleCreate() {
    if (!targetId) return
    createSchedule(
      { target_type: targetType, target_id: targetId, label, interval_seconds: intervalSeconds },
      { onSuccess: () => { setTargetId(''); setLabel('') } },
    )
  }

  function humanInterval(s: number): string {
    if (s < 3600) return `${s}s`
    if (s < 86400) return `${Math.round(s / 3600)}h`
    return `${Math.round(s / 86400)}d`
  }

  const schedules: CveSchedule[] = data?.schedules ?? []

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        padding: '12px 16px',
      }}
    >
      <div className="flex items-center gap-2 mb-4">
        <Clock size={12} style={{ color: 'var(--text-muted)' }} />
        <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
          Scheduled Scans
        </span>
      </div>

      {/* Create form */}
      <div className="flex items-center gap-2 flex-wrap mb-4">
        <select
          value={targetType}
          onChange={(e) => { setTargetType(e.target.value as 'node' | 'container'); setTargetId('') }}
          className="font-mono text-xs px-2 py-1"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
          }}
        >
          <option value="node">Node (all containers)</option>
          <option value="container">Container</option>
        </select>

        <select
          value={targetId}
          onChange={(e) => setTargetId(e.target.value)}
          className="font-mono text-xs px-2 py-1"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            minWidth: '160px',
          }}
        >
          <option value="">Select target…</option>
          {targets.map((t) => (
            <option key={t.id} value={t.id}>{t.label}</option>
          ))}
        </select>

        <input
          type="text"
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          placeholder="Label (optional)"
          className="font-mono text-xs px-2 py-1"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            minWidth: '120px',
          }}
        />

        <select
          value={intervalSeconds}
          onChange={(e) => setIntervalSeconds(Number(e.target.value))}
          className="font-mono text-xs px-2 py-1"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
          }}
        >
          {INTERVAL_OPTIONS.map((o) => (
            <option key={o.seconds} value={o.seconds}>{o.label}</option>
          ))}
        </select>

        <button
          type="button"
          disabled={!targetId || isCreating}
          onClick={handleCreate}
          className="flex items-center gap-1 text-xs px-3 py-1"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: (!targetId || isCreating) ? 'not-allowed' : 'pointer',
            opacity: !targetId ? 0.5 : 1,
          }}
        >
          {isCreating ? <Loader size={10} className="animate-spin" /> : <Plus size={10} />}
          Add
        </button>
      </div>

      {/* Schedules list */}
      {isLoading ? (
        <div className="flex items-center gap-2 py-2">
          <Loader size={11} className="animate-spin" style={{ color: 'var(--accent)' }} />
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading schedules…</span>
        </div>
      ) : schedules.length === 0 ? (
        <div className="text-xs py-2" style={{ color: 'var(--text-muted)' }}>
          No schedules configured.
        </div>
      ) : (
        <div style={{ border: '1px solid var(--border-subtle)', borderRadius: '3px', overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                {['Label', 'Target', 'Interval', 'Last run', 'Enabled', ''].map((col) => (
                  <th
                    key={col}
                    className="px-3 py-1.5 text-left text-xs uppercase tracking-wider font-medium"
                    style={{
                      color: 'var(--text-muted)',
                      borderBottom: '1px solid var(--border-subtle)',
                      fontSize: '11px',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {col}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {schedules.map((sched) => (
                <tr key={sched.id}>
                  <td className="px-3 py-1.5 text-xs" style={{ color: 'var(--text-primary)', borderBottom: '1px solid var(--border-subtle)' }}>
                    {sched.label || <span style={{ color: 'var(--text-muted)' }}>—</span>}
                  </td>
                  <td className="px-3 py-1.5 font-mono text-xs" style={{ color: 'var(--text-secondary)', borderBottom: '1px solid var(--border-subtle)', whiteSpace: 'nowrap' }}>
                    {sched.target_type}: {sched.target_id.slice(0, 12)}…
                  </td>
                  <td className="px-3 py-1.5 text-xs" style={{ color: 'var(--text-secondary)', borderBottom: '1px solid var(--border-subtle)' }}>
                    {humanInterval(sched.interval_seconds)}
                  </td>
                  <td className="px-3 py-1.5 text-xs" style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-subtle)', whiteSpace: 'nowrap' }}>
                    {sched.last_run_at ? new Date(sched.last_run_at).toLocaleString() : 'Never'}
                  </td>
                  <td className="px-3 py-1.5" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                    <input
                      type="checkbox"
                      checked={sched.enabled}
                      onChange={(e) => toggleSchedule({ id: sched.id, enabled: e.target.checked })}
                      className="accent-[var(--accent)]"
                    />
                  </td>
                  <td className="px-3 py-1.5" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                    <button
                      type="button"
                      onClick={() => deleteSchedule(sched.id)}
                      className="p-0.5"
                      style={{ color: 'var(--text-muted)', cursor: 'pointer', background: 'none', border: 'none' }}
                      title="Delete schedule"
                    >
                      <Trash2 size={12} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// ---- Admin gate ----

function AdminRequired() {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-16" style={{ color: 'var(--text-muted)' }}>
      <ShieldAlert size={28} style={{ color: 'var(--text-muted)' }} />
      <span className="text-xs uppercase tracking-wider">Admin access required</span>
      <span className="text-xs">CVE scanning is only accessible to administrators.</span>
    </div>
  )
}

// ---- Main CVE page ----

export default function CVE() {
  const { data: me, isLoading: meLoading } = useMe()
  const isAdmin = me?.role === 'admin'

  const { data, isLoading, refetch, isRefetching } = useCVEScans(isAdmin)
  const { data: status } = useCVEStatus(isAdmin)
  const [expandedDigest, setExpandedDigest] = useState<string | null>(null)
  const [tableFilter, setTableFilter] = useState<SeverityFilter>('all')
  const [timeWindow, setTimeWindow] = useState<TimeWindow>('all')

  // Prefer the status endpoint's resolved trivy presence; fall back to the
  // scans payload's `available` until status loads.
  const scannerAvailable = status?.available ?? data?.available ?? true
  const rawScans = data?.scans ?? []
  const now = Date.now()
  const sorted = sortScans(rawScans).filter(
    (s) => passesTableFilter(s, tableFilter) && passesTimeFilter(s, timeWindow, now),
  )

  function toggleExpand(digest: string) {
    setExpandedDigest((prev) => (prev === digest ? null : digest))
  }

  return (
    <AppShell>
      {/*
        Page root is a flex column whose children stack vertically. It must NOT be
        height-locked: AppShell's <main> is the scroll container (overflow-auto), so
        this column has to grow to its full content height and let <main> scroll.
        Previously `h-full` pinned the column to the viewport height; because the
        stacked sections are shrinkable flex items, the tall lower sections (the
        "Image Scans" table) got compressed/clipped and were unreachable without
        zooming out. Dropping `h-full` (and the now-pointless `flex-1 min-h-0`,
        which only matter for a height-constrained column) lets the page scroll
        naturally and keeps Image Scans reachable.
      */}
      <div
        className="flex flex-col w-full p-6"
        style={{ maxWidth: '960px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-2">
            <Bug size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1
              className="text-sm font-medium uppercase tracking-wider"
              style={{ color: 'var(--text-primary)' }}
            >
              CVE Scan
            </h1>
          </div>
          {isAdmin && (
            <button
              type="button"
              onClick={() => void refetch()}
              disabled={isRefetching}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: isRefetching ? 'default' : 'pointer',
                opacity: isRefetching ? 0.6 : 1,
              }}
            >
              <RefreshCw size={11} className={isRefetching ? 'animate-spin' : ''} />
              {isRefetching ? 'Refreshing…' : 'Refresh'}
            </button>
          )}
        </div>

        {/* Loading */}
        {(meLoading || isLoading) && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading CVE data…
            </span>
          </div>
        )}

        {/* Admin gate */}
        {!meLoading && !isAdmin && <AdminRequired />}

        {/* Content */}
        {isAdmin && !meLoading && !isLoading && (
          <>
            {/* Scanner status banner */}
            {!scannerAvailable ? (
              <div
                className="flex items-start gap-2 px-3 py-2.5 text-xs mb-6"
                style={{
                  backgroundColor: 'rgba(74,82,104,0.12)',
                  border: '1px solid var(--border-default)',
                  borderRadius: '3px',
                  color: 'var(--text-secondary)',
                }}
              >
                <Bug size={12} style={{ color: 'var(--text-muted)', flexShrink: 0, marginTop: '1px' }} />
                <span>
                  Image scanning requires Trivy — set{' '}
                  <code className="font-mono" style={{ color: 'var(--text-primary)' }}>
                    TRIVY_PATH
                  </code>{' '}
                  or install{' '}
                  <code className="font-mono" style={{ color: 'var(--text-primary)' }}>
                    trivy
                  </code>{' '}
                  on the Stratum host. Cached scan results are shown below (read-only).
                </span>
              </div>
            ) : (
              <div
                className="flex items-start gap-2 px-3 py-2.5 text-xs mb-6"
                style={{
                  backgroundColor: 'rgba(64,200,120,0.08)',
                  border: '1px solid rgba(64,200,120,0.25)',
                  borderRadius: '3px',
                  color: 'var(--text-secondary)',
                }}
              >
                <ShieldCheck size={12} style={{ color: '#40c878', flexShrink: 0, marginTop: '1px' }} />
                <span>
                  Trivy ready
                  {status?.version ? (
                    <>
                      {' '}
                      <span style={{ color: 'var(--text-muted)' }}>v{status.version}</span>
                    </>
                  ) : null}
                  {' — '}
                  {typeof status?.db_age_days === 'number' ? (
                    status.db_age_days <= 0 ? (
                      <span style={{ color: 'var(--text-primary)' }}>vulnerability DB updated today</span>
                    ) : (
                      <span style={{ color: status.db_age_days > 7 ? 'var(--status-warn)' : 'var(--text-primary)' }}>
                        vulnerability DB {status.db_age_days} day{status.db_age_days === 1 ? '' : 's'} old
                      </span>
                    )
                  ) : (
                    <span style={{ color: 'var(--text-muted)' }}>
                      vulnerability DB downloads on first scan
                    </span>
                  )}
                  .
                </span>
              </div>
            )}

            {/* Single-container scan trigger */}
            <div className="mb-4">
              <ScanTrigger scannerAvailable={scannerAvailable} />
            </div>

            {/* Bulk scan trigger */}
            <div className="mb-6">
              <BulkScanTrigger scannerAvailable={scannerAvailable} />
            </div>

            {/* Scheduled scans */}
            <div className="mb-6">
              <SchedulesPanel isAdmin={isAdmin} />
            </div>

            {/* Scans table */}
            <div className="mb-3 flex items-center gap-3">
              <div
                className="flex items-center gap-2"
                style={{ borderBottom: '1px solid var(--border-subtle)', paddingBottom: '6px', flex: 1 }}
              >
                <span
                  className="text-xs font-medium uppercase tracking-wider"
                  style={{ color: 'var(--text-muted)' }}
                >
                  Image Scans
                </span>
                <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
                  ({rawScans.length})
                </span>
              </div>
              {/* Time-window filter — newest scans always sort first. */}
              <div className="flex items-center gap-1 mb-1">
                {TIME_WINDOWS.map((tw) => {
                  const active = timeWindow === tw.value
                  return (
                    <button
                      key={tw.value}
                      type="button"
                      onClick={() => setTimeWindow(tw.value)}
                      className="font-mono text-xs px-2 py-0.5"
                      style={{
                        background: active ? 'var(--accent-glow)' : 'var(--bg-elevated)',
                        border: `1px solid ${active ? 'var(--accent)' : 'var(--border-default)'}`,
                        color: active ? 'var(--accent)' : 'var(--text-secondary)',
                        borderRadius: '3px',
                        cursor: 'pointer',
                      }}
                    >
                      {tw.label}
                    </button>
                  )
                })}
              </div>
              <select
                value={tableFilter}
                onChange={(e) => setTableFilter(e.target.value as SeverityFilter)}
                className="font-mono text-xs px-2 py-0.5 mb-1"
                style={{
                  background: 'var(--bg-elevated)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-secondary)',
                  borderRadius: '3px',
                  cursor: 'pointer',
                }}
              >
                <option value="all">All images</option>
                <option value="critical">Has Critical</option>
                <option value="high">Has High</option>
                <option value="medium">Has Medium</option>
                <option value="low">Has Low</option>
              </select>
            </div>

            {rawScans.length === 0 ? (
              <div
                className="px-3 py-4 text-xs"
                style={{
                  color: 'var(--text-muted)',
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                }}
              >
                No scan results yet. Use the scan trigger above to scan a container image.
              </div>
            ) : sorted.length === 0 ? (
              <div
                className="px-3 py-4 text-xs"
                style={{
                  color: 'var(--text-muted)',
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                }}
              >
                No images match the current filter.
              </div>
            ) : (
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
                      {['Image', 'Scanned At', 'Vulnerabilities'].map((col) => (
                        <th
                          key={col}
                          className="px-3 py-2 text-left text-xs uppercase tracking-wider font-medium"
                          style={{
                            color: 'var(--text-muted)',
                            borderBottom: '1px solid var(--border-subtle)',
                            fontSize: '12px',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {col}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {sorted.map((scan) => (
                      <ScanRow
                        key={scan.image_digest}
                        scan={scan}
                        isExpanded={expandedDigest === scan.image_digest}
                        onToggle={() => toggleExpand(scan.image_digest)}
                      />
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}
      </div>
    </AppShell>
  )
}
