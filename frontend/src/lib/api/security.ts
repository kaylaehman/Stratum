import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost } from '../api'
import type {
  AcknowledgeRequest,
  PortsResponse,
  PostureResult,
  PrivilegedResponse,
  SecurityBadgesResponse,
} from '../../types/api'

export function privilegedKey() {
  return ['security', 'privileged'] as const
}

export function portsKey() {
  return ['security', 'ports'] as const
}

export function securityBadgesKey() {
  return ['security', 'badges'] as const
}

export function usePrivileged(enabled = true) {
  return useQuery({
    queryKey: privilegedKey(),
    queryFn: () => apiGet<PrivilegedResponse>('/api/security/privileged'),
    enabled,
    staleTime: 30_000,
  })
}

export function usePorts(enabled = true) {
  return useQuery({
    queryKey: portsKey(),
    queryFn: () => apiGet<PortsResponse>('/api/security/ports'),
    enabled,
    staleTime: 30_000,
  })
}

export function useSecurityBadges(enabled = true) {
  return useQuery({
    queryKey: securityBadgesKey(),
    queryFn: () => apiGet<SecurityBadgesResponse>('/api/containers/security-badges'),
    enabled,
    staleTime: 30_000,
  })
}

export function useAcknowledgeFlag() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: AcknowledgeRequest) =>
      apiPost<unknown>('/api/security/acknowledge', req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: privilegedKey() })
      void queryClient.invalidateQueries({ queryKey: securityBadgesKey() })
    },
  })
}

export function postureKey(nodeId: string) {
  return ['security', 'posture', nodeId] as const
}

export function usePosture(nodeId: string, enabled = true) {
  return useQuery({
    queryKey: postureKey(nodeId),
    queryFn: () => apiGet<PostureResult>(`/api/nodes/${encodeURIComponent(nodeId)}/posture`),
    enabled: enabled && nodeId !== '',
    staleTime: 60_000,
  })
}

export function useRescan() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (nodeId: string | undefined = undefined) => {
      const url = nodeId
        ? `/api/security/rescan?node=${encodeURIComponent(nodeId)}`
        : '/api/security/rescan'
      return apiPost<unknown>(url, null)
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: privilegedKey() })
      void queryClient.invalidateQueries({ queryKey: portsKey() })
      void queryClient.invalidateQueries({ queryKey: securityBadgesKey() })
    },
  })
}
