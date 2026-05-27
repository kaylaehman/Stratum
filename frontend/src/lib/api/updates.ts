import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost } from '../api'
import type { UpdatesResponse } from '../../types/api'

export function updatesKey() {
  return ['updates'] as const
}

export function useUpdates() {
  return useQuery({
    queryKey: updatesKey(),
    queryFn: () => apiGet<UpdatesResponse>('/api/updates'),
    staleTime: 60_000,
  })
}

export function useRescanUpdates() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => apiPost<void>('/api/updates/rescan', null),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: updatesKey() })
    },
  })
}
