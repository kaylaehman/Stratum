import { useQuery } from '@tanstack/react-query'
import { apiGet } from '../api'
import type { DepGraph } from '../../types/api'

export function depGraphKey(nodeId: string) {
  return ['depgraph', nodeId] as const
}

export function useNodeDepGraph(nodeId: string | null) {
  return useQuery({
    queryKey: depGraphKey(nodeId ?? ''),
    queryFn: () => apiGet<DepGraph>(`/api/nodes/${nodeId}/depgraph`),
    enabled: !!nodeId,
    staleTime: 30_000,
    retry: false,
  })
}
