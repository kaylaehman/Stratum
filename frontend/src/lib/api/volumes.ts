import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiFetch, apiPost } from '../api'
import type { VolumesResponse } from '../../types/api'

export function volumesKey() {
  return ['volumes'] as const
}

export function useVolumes() {
  return useQuery({
    queryKey: volumesKey(),
    queryFn: () => apiGet<VolumesResponse>('/api/volumes'),
    staleTime: 30_000,
  })
}

interface RemoveVolumeVars {
  nodeId: string
  name: string
}

export function useRemoveVolume() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, name }: RemoveVolumeVars) => {
      const encodedName = encodeURIComponent(name)
      return apiFetch<void>(`/api/nodes/${nodeId}/volumes/${encodedName}`, { method: 'DELETE' })
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: volumesKey() })
    },
  })
}

// ---- Prune unused volumes ----

export interface PruneUnusedResult {
  node_id: string
  name: string
  ok: boolean
  error?: string
}

export interface PruneUnusedResponse {
  results: PruneUnusedResult[]
  removed_count: number
  failed_count: number
}

interface PruneUnusedVars {
  /** Omit to prune across all docker-capable nodes. */
  nodeId?: string
}

export function usePruneUnusedVolumes() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId }: PruneUnusedVars) => {
      const body: { node_id?: string } = nodeId ? { node_id: nodeId } : {}
      return apiPost<PruneUnusedResponse>('/api/volumes/prune-unused', body)
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: volumesKey() })
    },
  })
}
