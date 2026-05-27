import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiFetch } from '../api'
import { TREE_KEY } from './tree'
import type { WOLConfig, SetWOLRequest } from '../../types/api'

export function wolConfigKey(nodeId: string) {
  return ['wol', nodeId] as const
}

export function useWOLConfig(nodeId: string) {
  return useQuery({
    queryKey: wolConfigKey(nodeId),
    queryFn: () => apiGet<WOLConfig>(`/api/nodes/${nodeId}/wol`),
    retry: false,
    staleTime: 30_000,
  })
}

interface SetWOLVars {
  nodeId: string
  request: SetWOLRequest
}

export function useSetWOL() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, request }: SetWOLVars) =>
      apiFetch<void>(`/api/nodes/${nodeId}/wol`, {
        method: 'PUT',
        body: JSON.stringify(request),
      }),
    onSuccess: (_data, { nodeId }) => {
      void queryClient.invalidateQueries({ queryKey: wolConfigKey(nodeId) })
    },
  })
}

interface WakeNodeVars {
  nodeId: string
}

export function useWakeNode() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId }: WakeNodeVars) =>
      apiFetch<void>(`/api/nodes/${nodeId}/wake`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: TREE_KEY })
    },
  })
}
