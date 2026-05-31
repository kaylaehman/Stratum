import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiFetch } from '../api'

export interface UptimeMonitor {
  id: string
  name: string
  type: 'http' | 'tcp' | 'icmp'
  target: string
  interval_seconds: number
  timeout_ms: number
  expected: string
  enabled: boolean
  node_id?: string
  created_at: string
  updated_at: string
  // Stats (inline in list/get responses)
  monitor_id?: string
  current_status?: 'up' | 'down' | 'degraded'
  uptime_24h?: number
  uptime_7d?: number
  uptime_30d?: number
  avg_response_ms?: number
}

export interface UptimeResult {
  id: string
  monitor_id: string
  checked_at: string
  status: 'up' | 'down' | 'degraded'
  response_time_ms: number
  error?: string
}

export interface UptimeMonitorRequest {
  name: string
  type: 'http' | 'tcp' | 'icmp'
  target: string
  interval_seconds: number
  timeout_ms: number
  expected: string
  enabled: boolean
  node_id?: string
}

export interface UptimeListResponse {
  monitors: UptimeMonitor[]
}

export interface UptimeHistoryResponse {
  results: UptimeResult[]
}

export type UptimeHistoryRange = '24h' | '7d' | '30d'

// --- Query keys ---

export function uptimeMonitorsKey() {
  return ['uptime', 'monitors'] as const
}

export function uptimeMonitorKey(id: string) {
  return ['uptime', 'monitors', id] as const
}

export function uptimeHistoryKey(id: string, range: UptimeHistoryRange) {
  return ['uptime', 'history', id, range] as const
}

// --- Hooks ---

export function useUptimeMonitors() {
  return useQuery({
    queryKey: uptimeMonitorsKey(),
    queryFn: () => apiGet<UptimeListResponse>('/api/uptime/monitors'),
    staleTime: 30_000,
    refetchInterval: 30_000,
  })
}

export function useUptimeMonitor(id: string) {
  return useQuery({
    queryKey: uptimeMonitorKey(id),
    queryFn: () => apiGet<UptimeMonitor>(`/api/uptime/monitors/${id}`),
    staleTime: 30_000,
    refetchInterval: 30_000,
  })
}

export function useUptimeHistory(id: string, range: UptimeHistoryRange) {
  return useQuery({
    queryKey: uptimeHistoryKey(id, range),
    queryFn: () =>
      apiGet<UptimeHistoryResponse>(`/api/uptime/monitors/${id}/history?range=${range}`),
    staleTime: 15_000,
    refetchInterval: 30_000,
    enabled: !!id,
  })
}

export function useCreateUptimeMonitor() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: UptimeMonitorRequest) => apiPost('/api/uptime/monitors', body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: uptimeMonitorsKey() })
    },
  })
}

interface UpdateUptimeVars {
  id: string
  body: UptimeMonitorRequest
}

export function useUpdateUptimeMonitor() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: UpdateUptimeVars) =>
      apiFetch<void>(`/api/uptime/monitors/${id}`, {
        method: 'PUT',
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: uptimeMonitorsKey() })
    },
  })
}

export function useDeleteUptimeMonitor() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/uptime/monitors/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: uptimeMonitorsKey() })
    },
  })
}
