import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../api'

// ---- Local types ----

export interface ConfigVersion {
  id: string
  node_id: string
  path: string
  hash: string
  author: string
  created_at: string
}

export interface DriftResult {
  has_snapshot: boolean
  is_drifted: boolean
  snapshot_content: string
  current_content: string
  snapshot_hash: string
  current_hash: string
}

// ---- Query keys ----

export function configVersionsKey(nodeId: string, path: string) {
  return ['configversions', nodeId, path] as const
}

export function configDriftKey(nodeId: string, path: string) {
  return ['configversions', 'drift', nodeId, path] as const
}

// ---- Hooks ----

export function useConfigVersions(nodeId: string, path: string, enabled = true) {
  return useQuery({
    queryKey: configVersionsKey(nodeId, path),
    queryFn: () =>
      apiFetch<ConfigVersion[]>(
        `/api/nodes/${encodeURIComponent(nodeId)}/configversions?path=${encodeURIComponent(path)}`,
      ),
    enabled: enabled && Boolean(nodeId && path),
    staleTime: 10_000,
  })
}

export function useConfigDrift(nodeId: string, path: string, enabled = true) {
  return useQuery({
    queryKey: configDriftKey(nodeId, path),
    queryFn: () =>
      apiFetch<DriftResult>(
        `/api/nodes/${encodeURIComponent(nodeId)}/configversions/drift?path=${encodeURIComponent(path)}`,
      ),
    enabled: enabled && Boolean(nodeId && path),
    staleTime: 15_000,
    refetchOnWindowFocus: true,
  })
}

export function useSnapshotConfig(nodeId: string, path: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () =>
      apiFetch<void>(`/api/nodes/${encodeURIComponent(nodeId)}/configversions/snapshot`, {
        method: 'POST',
        body: JSON.stringify({ path }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: configVersionsKey(nodeId, path) })
      void qc.invalidateQueries({ queryKey: configDriftKey(nodeId, path) })
    },
  })
}

export function useRevertConfig(nodeId: string, path: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (version_id: string) =>
      apiFetch<void>(`/api/nodes/${encodeURIComponent(nodeId)}/configversions/revert`, {
        method: 'POST',
        body: JSON.stringify({ version_id }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: configVersionsKey(nodeId, path) })
      void qc.invalidateQueries({ queryKey: configDriftKey(nodeId, path) })
    },
  })
}
