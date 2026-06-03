import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiPut } from '../api'
import type {
  AddProxyRoutePlan,
  AddProxyRouteRequest,
  CloudflareDiscovery,
  ContainerProxyStatus,
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

export function containerProxyKey(containerId: string) {
  return ['container-proxy', containerId] as const
}

/** The reverse-proxy state for one container: serving hostnames, add targets,
 *  and suggested target URLs. Admin-only; 403 for non-admins (retry disabled). */
export function useContainerProxy(containerId: string | undefined) {
  return useQuery({
    queryKey: containerProxyKey(containerId ?? ''),
    queryFn: () => apiGet<ContainerProxyStatus>(`/api/containers/${containerId}/proxy`),
    enabled: Boolean(containerId),
    retry: false,
  })
}

/** Add (or, with dry_run, preview) a reverse-proxy route to a container. A
 *  dry-run performs no mutation and is not audited; a real add is audited and
 *  invalidates the container's proxy state so the new route appears. */
export function useAddContainerProxy(containerId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (request: AddProxyRouteRequest) =>
      apiPost<AddProxyRoutePlan>(`/api/containers/${containerId}/proxy`, request),
    onSuccess: (plan) => {
      if (plan.applied) {
        void queryClient.invalidateQueries({ queryKey: containerProxyKey(containerId) })
      }
    },
  })
}
