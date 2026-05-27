import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiDelete } from '../api'
import type { FileWatchesResponse, AddWatchRequest, ScanResponse, FileEventsResponse } from '../../types/api'

export function watchesKey(nodeId: string) {
  return ['filewatch', nodeId] as const
}

export function eventsKey(nodeId: string) {
  return ['fileevents', nodeId] as const
}

export function useWatches(nodeId: string) {
  return useQuery({
    queryKey: watchesKey(nodeId),
    queryFn: () => apiGet<FileWatchesResponse>(`/api/nodes/${nodeId}/watches`),
    enabled: Boolean(nodeId),
    retry: false,
  })
}

export function useAddWatch(nodeId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: AddWatchRequest) =>
      apiPost<void>(`/api/nodes/${nodeId}/watches`, req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: watchesKey(nodeId) })
    },
  })
}

export function useDeleteWatch(nodeId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (watchId: string) =>
      apiDelete<void>(`/api/nodes/${nodeId}/watches/${watchId}`),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: watchesKey(nodeId) })
    },
  })
}

export function useScanWatches(nodeId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => apiPost<ScanResponse>(`/api/nodes/${nodeId}/watches/scan`, {}),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: eventsKey(nodeId) })
    },
  })
}

export function useFileEvents(nodeId: string) {
  return useQuery({
    queryKey: eventsKey(nodeId),
    queryFn: () => apiGet<FileEventsResponse>(`/api/fileevents?node=${nodeId}&limit=200`),
    enabled: Boolean(nodeId),
    retry: false,
  })
}
