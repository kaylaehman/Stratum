import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPut } from '../api'
import type { DnsStatus, SetDnsConfigRequest } from '../../types/api'

export function dnsKey(nodeId: string) {
  return ['dns', nodeId] as const
}

export function useNodeDns(nodeId: string | undefined) {
  return useQuery({
    queryKey: dnsKey(nodeId ?? ''),
    queryFn: () => apiGet<DnsStatus>(`/api/nodes/${nodeId}/dns`),
    enabled: Boolean(nodeId),
    retry: false,
  })
}

interface SetDnsConfigVars {
  nodeId: string
  request: SetDnsConfigRequest
}

export function useSetDnsConfig(nodeId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId: id, request }: SetDnsConfigVars) =>
      apiPut<DnsStatus>(`/api/nodes/${id}/dns/config`, request),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: dnsKey(nodeId) })
    },
  })
}
