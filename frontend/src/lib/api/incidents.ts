import { useQuery } from '@tanstack/react-query'
import { apiGet } from '../api'

export type IncidentSource = 'activity' | 'container' | 'metric' | 'file_event'
export type IncidentSeverity = 'info' | 'warning' | 'critical'

export interface IncidentEntry {
  timestamp: string
  source: IncidentSource
  severity: IncidentSeverity
  node_id?: string
  target_id?: string
  target_type?: string
  summary: string
  deep_link?: string
}

export interface IncidentTimelineResponse {
  entries: IncidentEntry[]
  from: string
  to: string
}

export interface IncidentFilters {
  from?: string
  to?: string
  node_id?: string
}

function buildIncidentQuery(filters: IncidentFilters): string {
  const params = new URLSearchParams()
  if (filters.from) params.set('from', filters.from)
  if (filters.to) params.set('to', filters.to)
  if (filters.node_id) params.set('node_id', filters.node_id)
  return params.toString()
}

export function incidentTimelineKey(filters: IncidentFilters) {
  return ['incidents', 'timeline', filters] as const
}

export function useIncidentTimeline(filters: IncidentFilters) {
  const qs = buildIncidentQuery(filters)
  return useQuery({
    queryKey: incidentTimelineKey(filters),
    queryFn: () =>
      apiGet<IncidentTimelineResponse>(`/api/incidents/timeline${qs ? `?${qs}` : ''}`),
    staleTime: 30_000,
    refetchInterval: 60_000,
  })
}
