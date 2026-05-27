import { useQuery } from '@tanstack/react-query'
import { apiGet } from '../api'
import { useAuthStore } from '../../store/auth'
import type { MetricsResponse } from '../../types/api'

export type MetricsRange = '1h' | '6h' | '24h' | '7d'

export function metricsKey(containerId: string, range: MetricsRange) {
  return ['metrics', containerId, range] as const
}

export function useContainerMetrics(containerId: string | null, range: MetricsRange) {
  return useQuery({
    queryKey: metricsKey(containerId ?? '', range),
    queryFn: () =>
      apiGet<MetricsResponse>(`/api/containers/${containerId}/metrics?range=${range}`),
    enabled: !!containerId,
    staleTime: 15_000,
    refetchInterval: 15_000,
  })
}

export async function exportMetricsCsv(containerId: string, range: MetricsRange): Promise<void> {
  const url = `/api/containers/${containerId}/metrics.csv?range=${range}`
  const token = useAuthStore.getState().accessToken

  const headers: Record<string, string> = {}
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch(url, { method: 'GET', headers })
  if (!res.ok) {
    throw new Error(`Export failed: ${res.status}`)
  }

  const blob = await res.blob()
  const objectUrl = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = objectUrl
  anchor.download = `metrics-${containerId}-${range}.csv`
  document.body.appendChild(anchor)
  anchor.click()
  document.body.removeChild(anchor)
  URL.revokeObjectURL(objectUrl)
}
