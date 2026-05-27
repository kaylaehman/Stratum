import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost } from '../api'
import { TREE_KEY } from './tree'
import { updatesKey } from './updates'
import type { SnapshotsResponse, UpdateContainerResponse, RollbackResponse, SaveSnapshotResponse } from '../../types/api'

export function snapshotsKey(containerId: string) {
  return ['snapshots', containerId] as const
}

export function useSnapshots(containerId: string) {
  return useQuery({
    queryKey: snapshotsKey(containerId),
    queryFn: () => apiGet<SnapshotsResponse>(`/api/containers/${encodeURIComponent(containerId)}/snapshots`),
    enabled: !!containerId,
    staleTime: 30_000,
  })
}

export function useSaveSnapshot() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (containerId: string) =>
      apiPost<SaveSnapshotResponse>(`/api/containers/${encodeURIComponent(containerId)}/snapshot`, null),
    onSuccess: (_data, containerId) => {
      void queryClient.invalidateQueries({ queryKey: snapshotsKey(containerId) })
    },
  })
}

export function useUpdateContainer() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (containerId: string) =>
      apiPost<UpdateContainerResponse>(`/api/containers/${encodeURIComponent(containerId)}/update`, null),
    onSuccess: (_data, containerId) => {
      void queryClient.invalidateQueries({ queryKey: TREE_KEY })
      void queryClient.invalidateQueries({ queryKey: updatesKey() })
      void queryClient.invalidateQueries({ queryKey: snapshotsKey(containerId) })
    },
  })
}

export function useRollback() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ containerId, snapshotId }: { containerId: string; snapshotId: string }) =>
      apiPost<RollbackResponse>(
        `/api/containers/${encodeURIComponent(containerId)}/rollback/${encodeURIComponent(snapshotId)}`,
        null,
      ),
    onSuccess: (_data, { containerId }) => {
      void queryClient.invalidateQueries({ queryKey: TREE_KEY })
      void queryClient.invalidateQueries({ queryKey: snapshotsKey(containerId) })
    },
  })
}
