import { useQuery } from '@tanstack/react-query'
import { apiGet } from '../api'
import type { MountsResponse, ReverseMountsResponse, SharedMountsResponse } from '../../types/api'

export function containerMountsKey(containerId: string) {
  return ['container', containerId, 'mounts'] as const
}

export function reverseMountsKey(nodeId: string, hostPath: string) {
  return ['node', nodeId, 'mounts', 'reverse', hostPath] as const
}

export function sharedMountsKey(nodeId: string) {
  return ['node', nodeId, 'mounts', 'shared'] as const
}

export function useContainerMounts(containerId: string) {
  return useQuery({
    queryKey: containerMountsKey(containerId),
    queryFn: () => apiGet<MountsResponse>(`/api/containers/${containerId}/mounts`),
    enabled: Boolean(containerId),
    staleTime: 30_000,
  })
}

export function useReverseMounts(nodeId: string, hostPath: string) {
  return useQuery({
    queryKey: reverseMountsKey(nodeId, hostPath),
    queryFn: () =>
      apiGet<ReverseMountsResponse>(
        `/api/nodes/${nodeId}/mounts?host_path=${encodeURIComponent(hostPath)}`,
      ),
    enabled: Boolean(nodeId && hostPath),
    staleTime: 30_000,
  })
}

export function useSharedMounts(nodeId: string) {
  return useQuery({
    queryKey: sharedMountsKey(nodeId),
    queryFn: () => apiGet<SharedMountsResponse>(`/api/nodes/${nodeId}/mounts/shared`),
    enabled: Boolean(nodeId),
    staleTime: 30_000,
  })
}
