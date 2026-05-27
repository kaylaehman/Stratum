import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost } from '../api'
import type { BackupsResponse, StartBackupRequest, StartBackupResponse } from '../../types/api'

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
