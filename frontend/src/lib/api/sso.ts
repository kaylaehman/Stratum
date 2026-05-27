import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiDelete, apiGet, apiPut } from '../api'
import type { SSOConfig, SSOListResponse, SSOUpsertRequest } from '../../types/api'

export function ssoKey() {
  return ['sso'] as const
}

export function useSSOConfigs(enabled = true) {
  return useQuery({
    queryKey: ssoKey(),
    queryFn: () => apiGet<SSOListResponse>('/api/sso'),
    enabled,
    staleTime: 30_000,
  })
}

export function useUpsertSSO() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: SSOUpsertRequest) =>
      apiPut<SSOConfig>('/api/sso', req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ssoKey() })
    },
  })
}

export function useDeleteSSO() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      apiDelete<void>(`/api/sso/${encodeURIComponent(id)}`),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ssoKey() })
    },
  })
}
