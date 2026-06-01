import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost } from '../api'
import type {
  BackupsResponse,
  StartBackupRequest,
  StartBackupResponse,
  StartGuestBackupRequest,
  StartGuestBackupResponse,
} from '../../types/api'

// ---- Local types (not in types/api.ts) ----

export interface RestoreDockerRequest {
  archive_path: string
  target_path: string
}

export interface RestoreDockerResponse {
  output: string
}

export interface RestoreGuestRequest {
  pve_node: string
  archive_path: string
  target_storage: string
  target_vmid?: number
}

export interface RestoreGuestResponse {
  output: string
}

export interface VerifyResult {
  passed: boolean
  file_count: number
  total_bytes: number
  archive_path: string
  checked_at: string
  error?: string
}

export interface VerifyResultsResponse {
  results: VerifyResult[]
}

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

// ---- Restore hooks ----

interface RestoreDockerVars {
  nodeId: string
  archivePath: string
  targetPath: string
}

/** POST /api/nodes/{id}/backups/restore — admin + step-up */
export function useRestoreDocker() {
  return useMutation({
    mutationFn: ({ nodeId, archivePath, targetPath }: RestoreDockerVars) =>
      apiPost<RestoreDockerResponse>(`/api/nodes/${nodeId}/backups/restore`, {
        archive_path: archivePath,
        target_path: targetPath,
      } satisfies RestoreDockerRequest),
  })
}

interface RestoreGuestVars {
  nodeId: string
  pveNode: string
  archivePath: string
  targetStorage: string
  targetVmid?: number
}

/** POST /api/nodes/{id}/backups/restore-guest — admin + step-up */
export function useRestoreGuest() {
  return useMutation({
    mutationFn: ({ nodeId, pveNode, archivePath, targetStorage, targetVmid }: RestoreGuestVars) =>
      apiPost<RestoreGuestResponse>(`/api/nodes/${nodeId}/backups/restore-guest`, {
        pve_node: pveNode,
        archive_path: archivePath,
        target_storage: targetStorage,
        ...(targetVmid !== undefined ? { target_vmid: targetVmid } : {}),
      } satisfies RestoreGuestRequest),
  })
}

// ---- Verify hooks ----

function verifyResultsKey(nodeId: string) {
  return ['backups', 'verify', nodeId] as const
}

/** POST /api/nodes/{id}/backups/verify — operator */
export function useVerifyBackup(nodeId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () =>
      apiPost<VerifyResult>(`/api/nodes/${nodeId}/backups/verify`, {}),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: verifyResultsKey(nodeId) })
    },
  })
}

/** GET /api/nodes/{id}/backups/verify — past verify results */
export function useVerifyResults(nodeId: string, enabled: boolean) {
  return useQuery({
    queryKey: verifyResultsKey(nodeId),
    queryFn: () => apiGet<VerifyResultsResponse>(`/api/nodes/${nodeId}/backups/verify`),
    enabled,
    staleTime: 30_000,
  })
}
