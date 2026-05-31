import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost } from '../api'
import type { UpdatesResponse as _UpdatesResponse } from '../../types/api'

// ---- Summary types (backend /api/updates summary field) ----

export interface UnknownBreakdownItem {
  category: string
  count: number
  example_reason: string
}

export interface UpdatesSummary {
  total: number
  up_to_date: number
  update_available: number
  unknown: number
  dominant_unknown_category: string
  dominant_unknown_count: number
  unknown_breakdown: UnknownBreakdownItem[]
}

/** Extended locally — do NOT edit the shared types/api.ts UpdatesResponse */
export interface UpdatesResponse extends _UpdatesResponse {
  summary?: UpdatesSummary
}

// ---- Query keys ----

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
