import { useQuery } from '@tanstack/react-query'
import { apiGet } from '../api'

// ---- Local types (not exported to types/api.ts per swarm boundary) ----

export type ForecastMetric = 'cpu' | 'memory' | 'disk_write' | 'disk_read' | 'net_rx' | 'net_tx'

export type ForecastTrend = 'rising' | 'falling' | 'stable'

export interface ForecastProjection {
  metric: ForecastMetric
  /** Current observed value (units depend on metric: % for cpu, bytes for memory/disk/net) */
  current: number
  /** Threshold value the projection targets (same units as current) */
  threshold: number
  /** Seconds until threshold is reached; -1 means no ETA (trend not converging) */
  eta_seconds: number
  /** Rate of change per sample interval */
  slope: number
  trend: ForecastTrend
}

export interface ContainerForecast {
  container_id: string
  projections: ForecastProjection[]
}

export interface NodeForecastResponse {
  forecast: ContainerForecast[]
}

// ---- React Query hook ----

export function forecastKey(nodeId: string) {
  return ['forecast', nodeId] as const
}

export function useNodeForecast(nodeId: string | null) {
  return useQuery({
    queryKey: forecastKey(nodeId ?? ''),
    queryFn: () => apiGet<NodeForecastResponse>(`/api/nodes/${nodeId}/forecast`),
    enabled: !!nodeId,
    staleTime: 60_000,
    refetchInterval: 60_000,
  })
}

// ---- Formatting helpers ----

/** Convert eta_seconds into a human-readable duration string. */
export function fmtEta(etaSeconds: number): string {
  if (etaSeconds < 0) return ''
  if (etaSeconds < 3600) {
    const mins = Math.ceil(etaSeconds / 60)
    return mins <= 1 ? 'less than a minute' : `~${mins} minutes`
  }
  if (etaSeconds < 86400) {
    const hrs = Math.round(etaSeconds / 3600)
    return `~${hrs} hour${hrs !== 1 ? 's' : ''}`
  }
  const days = Math.round(etaSeconds / 86400)
  return `~${days} day${days !== 1 ? 's' : ''}`
}

/** Human-readable metric label. */
export function fmtMetricLabel(metric: ForecastMetric): string {
  switch (metric) {
    case 'cpu': return 'CPU'
    case 'memory': return 'Memory'
    case 'disk_write': return 'Disk Write'
    case 'disk_read': return 'Disk Read'
    case 'net_rx': return 'Network RX'
    case 'net_tx': return 'Network TX'
    default: return metric
  }
}

/** Urgency bucket based on eta_seconds. */
export function etaUrgency(etaSeconds: number): 'critical' | 'warning' | 'stable' {
  if (etaSeconds < 0) return 'stable'
  if (etaSeconds < 86400) return 'critical'       // < 1 day
  if (etaSeconds < 7 * 86400) return 'warning'    // < 7 days
  return 'stable'
}
