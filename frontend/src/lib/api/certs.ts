import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost } from '../api'
import type { CertsResponse, RescanCertsResponse } from '../../types/api'

export function certsKey() {
  return ['certs'] as const
}

export function useCerts() {
  return useQuery({
    queryKey: certsKey(),
    queryFn: () => apiGet<CertsResponse>('/api/certs'),
    staleTime: 60_000,
  })
}

export function useRescanCerts() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => apiPost<RescanCertsResponse>('/api/certs/rescan', null),
    onSuccess: (data) => {
      queryClient.setQueryData(certsKey(), data)
    },
  })
}
