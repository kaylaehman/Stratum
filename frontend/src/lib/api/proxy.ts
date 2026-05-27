import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPut } from '../api'
import type { ProxyStatus, SetProxyConfigRequest } from '../../types/api'

export function proxyKey(nodeId: string) {
  return ['proxy', nodeId] as const
}

export function useNodeProxy(nodeId: string | undefined) {
  return useQuery({
    queryKey: proxyKey(nodeId ?? ''),
    queryFn: () => apiGet<ProxyStatus>(`/api/nodes/${nodeId}/proxy`),
    enabled: Boolean(nodeId),
    retry: false,
  })
}

interface SetProxyConfigVars {
  nodeId: string
  request: SetProxyConfigRequest
}

export function useSetProxyConfig(nodeId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId: id, request }: SetProxyConfigVars) =>
      apiPut<ProxyStatus>(`/api/nodes/${id}/proxy/config`, request),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: proxyKey(nodeId) })
    },
  })
}
