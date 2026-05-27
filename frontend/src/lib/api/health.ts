import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPut } from '../api'
import { TREE_KEY } from './tree'
import type { HealthReport, SetHealthcheckRequest, SetHealthcheckResponse } from '../../types/api'

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

export function useSetHealthcheck(containerId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: SetHealthcheckRequest) =>
      apiPut<SetHealthcheckResponse>(
        `/api/containers/${encodeURIComponent(containerId)}/healthcheck`,
        req,
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: TREE_KEY })
      void queryClient.invalidateQueries({ queryKey: containerHealthKey(containerId) })
    },
  })
}
