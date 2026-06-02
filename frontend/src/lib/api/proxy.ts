import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiPut } from '../api'
import type {
  CloudflareDiscovery,
  DiscoverCloudflareRequest,
  ProxyStatus,
  SetProxyConfigRequest,
} from '../../types/api'

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

/** Probe a Cloudflare API token for its accounts + tunnels (cloudflare-api
 *  setup picker). Read-only; the token is used in-memory and not persisted by
 *  this call. Errors surface the Cloudflare message verbatim. */
export function useDiscoverCloudflare(nodeId: string) {
  return useMutation({
    mutationFn: (request: DiscoverCloudflareRequest) =>
      apiPost<CloudflareDiscovery>(`/api/nodes/${nodeId}/proxy/cloudflare/discover`, request),
  })
}
