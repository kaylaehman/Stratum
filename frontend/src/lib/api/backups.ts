import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost } from '../api'
import type {
  BackupsResponse,
  StartBackupRequest,
  StartBackupResponse,
  StartGuestBackupRequest,
  StartGuestBackupResponse,
} from '../../types/api'

export function backupsKey() {
  return ['backups'] as const
}

export function useBackups() {
  return useQuery({
    queryKey: backupsKey(),
    queryFn: () => apiGet<BackupsResponse>('/api/backups'),
    refetchInterval: 5000,
  })
}

interface StartBackupVars {
  nodeId: string
  volume: string
  destDir: string
}

export function useStartBackup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, volume, destDir }: StartBackupVars) =>
      apiPost<StartBackupResponse>(`/api/nodes/${nodeId}/backups`, {
        volume,
        dest_dir: destDir,
      } satisfies StartBackupRequest),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: backupsKey() })
    },
  })
}

interface StartGuestBackupVars {
  nodeId: string
  vmid: number
  storage: string
}

export function useStartGuestBackup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, vmid, storage }: StartGuestBackupVars) =>
      apiPost<StartGuestBackupResponse>(
        `/api/nodes/${nodeId}/vms/${vmid}/backup`,
        { storage } satisfies StartGuestBackupRequest,
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: backupsKey() })
    },
  })
}
