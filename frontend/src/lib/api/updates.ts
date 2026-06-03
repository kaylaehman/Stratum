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

/** An external management tool (Watchtower/Portainer) found on a host. */
export interface ManagementTool {
  name: string
  /** True when the tool updates containers on its own (Watchtower) — an active
   *  conflict with a Stratum-driven update, not just a second management UI. */
  auto_updates: boolean
}

/** A host that also runs a tool whose actions can conflict with Stratum updates. */
export interface OverlapNode {
  node_id: string
  node_name: string
  managers: ManagementTool[]
  auto_updates: boolean
}

/** Extended locally — do NOT edit the shared types/api.ts UpdatesResponse */
export interface UpdatesResponse extends _UpdatesResponse {
  summary?: UpdatesSummary
  overlaps?: OverlapNode[]
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
