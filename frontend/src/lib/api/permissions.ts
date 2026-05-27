import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '../../store/auth'
import { apiGet } from '../api'
import type { ContainerInspect, UidAnalysis, FileVerdict } from '../../types/api'

// ---- Query keys ----

export function containerInspectKey(containerId: string) {
  return ['container', containerId, 'inspect'] as const
}

export function uidAnalysisKey(containerId: string) {
  return ['container', containerId, 'uid-analysis'] as const
}

export function fileVerdictKey(containerId: string, hostPath: string) {
  return ['container', containerId, 'file-uid', hostPath] as const
}

// ---- Hooks ----

export function useContainerInspect(containerId: string) {
  return useQuery({
    queryKey: containerInspectKey(containerId),
    queryFn: () => apiGet<ContainerInspect>(`/api/containers/${containerId}`),
    enabled: Boolean(containerId),
    staleTime: 15_000,
  })
}

export function useUidAnalysis(containerId: string) {
  return useQuery({
    queryKey: uidAnalysisKey(containerId),
    queryFn: () => apiGet<UidAnalysis>(`/api/containers/${containerId}/uid-analysis`),
    enabled: Boolean(containerId),
    staleTime: 30_000,
  })
}

export function useFileVerdict(containerId: string, hostPath: string) {
  return useQuery({
    queryKey: fileVerdictKey(containerId, hostPath),
    queryFn: () =>
      apiGet<FileVerdict>(
        `/api/containers/${containerId}/file-uid?host_path=${encodeURIComponent(hostPath)}`,
      ),
    enabled: Boolean(containerId && hostPath),
    staleTime: 15_000,
  })
}

// ---- Download helper ----

export function downloadAnalysis(
  containerId: string,
  format: 'csv' | 'json',
): void {
  const { accessToken } = useAuthStore.getState()
  const url = `/api/containers/${containerId}/uid-analysis.${format}`
  void (async () => {
    const headers: Record<string, string> = {}
    if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`
    const res = await fetch(url, { headers })
    if (!res.ok) return
    const blob = await res.blob()
    const objectUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = objectUrl
    a.download = `uid-analysis-${containerId.slice(0, 12)}.${format}`
    a.click()
    URL.revokeObjectURL(objectUrl)
  })()
}
