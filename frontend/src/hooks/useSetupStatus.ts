import { useQuery } from '@tanstack/react-query'
import { apiGet } from '../lib/api'
import type { SetupStatusResponse } from '../types/api'

export function useSetupStatus() {
  return useQuery({
    queryKey: ['setup-status'],
    queryFn: () => apiGet<SetupStatusResponse>('/api/setup/status'),
    retry: false,
    staleTime: Infinity,
  })
}
