import { useQuery } from '@tanstack/react-query'
import { apiGet } from '../api'
import type { HealthReport } from '../../types/api'

export function containerHealthKey(containerId: string) {
  return ['container', containerId, 'health'] as const
}

export function useContainerHealth(containerId: string) {
  return useQuery({
    queryKey: containerHealthKey(containerId),
    queryFn: () => apiGet<HealthReport>(`/api/containers/${containerId}/health`),
    enabled: !!containerId,
    staleTime: 15_000,
    refetchInterval: 15_000,
  })
}
