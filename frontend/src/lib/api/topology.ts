import { useQuery } from '@tanstack/react-query'
import { apiGet } from '../api'
import type { TopologyResponse } from '../../types/api'

export function topologyKey(nodeId: string) {
  return ['topology', nodeId] as const
}

export function useNodeTopology(nodeId: string | null) {
  return useQuery({
    queryKey: topologyKey(nodeId ?? ''),
    queryFn: () => apiGet<TopologyResponse>(`/api/nodes/${nodeId}/topology`),
    enabled: !!nodeId,
    staleTime: 30_000,
    retry: false,
  })
}
