import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiFetch } from '../api'
import type { VolumesResponse } from '../../types/api'

export function volumesKey() {
  return ['volumes'] as const
}

export function useVolumes() {
  return useQuery({
    queryKey: volumesKey(),
    queryFn: () => apiGet<VolumesResponse>('/api/volumes'),
    staleTime: 30_000,
  })
}

interface RemoveVolumeVars {
  nodeId: string
  name: string
}

export function useRemoveVolume() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, name }: RemoveVolumeVars) => {
      const encodedName = encodeURIComponent(name)
      return apiFetch<void>(`/api/nodes/${nodeId}/volumes/${encodedName}`, { method: 'DELETE' })
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: volumesKey() })
    },
  })
}
