import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiFetch } from '../api'
import type {
  CVEScansResponse,
  CVEDetailResponse,
  CVEStatusResponse,
  CVEBulkScanResponse,
  CVESchedulesResponse,
  CveSchedule,
  CreateCveScheduleRequest,
} from '../../types/api'

export function cveScansKey() {
  return ['cve', 'scans'] as const
}

export function cveStatusKey() {
  return ['cve', 'status'] as const
}

export function useCVEStatus(enabled = true) {
  return useQuery({
    queryKey: cveStatusKey(),
    queryFn: () => apiGet<CVEStatusResponse>('/api/security/cve/status'),
    staleTime: 60_000,
    enabled,
  })
}

export function cveDetailKey(digest: string) {
  return ['cve', 'detail', digest] as const
}

export function useCVEScans(enabled = true) {
  return useQuery({
    queryKey: cveScansKey(),
    queryFn: () => apiGet<CVEScansResponse>('/api/security/cve'),
    staleTime: 60_000,
    enabled,
  })
}

export function useCVEDetail(digest: string, enabled: boolean) {
  return useQuery({
    queryKey: cveDetailKey(digest),
    queryFn: () => apiGet<CVEDetailResponse>(`/api/security/cve/${encodeURIComponent(digest)}`),
    staleTime: 120_000,
    enabled: enabled && digest.length > 0,
  })
}

export function useScanContainer() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (containerId: string) =>
      apiFetch<void>(`/api/containers/${containerId}/cve-scan`, { method: 'POST' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: cveScansKey() })
    },
  })
}

export function useBulkScan() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (containerIds: string[]) =>
      apiFetch<CVEBulkScanResponse>('/api/security/cve/bulk-scan', {
        method: 'POST',
        body: JSON.stringify({ container_ids: containerIds }),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: cveScansKey() })
    },
  })
}

export function cveSchedulesKey() {
  return ['cve', 'schedules'] as const
}

export function useCVESchedules(enabled = true) {
  return useQuery({
    queryKey: cveSchedulesKey(),
    queryFn: () => apiGet<CVESchedulesResponse>('/api/security/cve/schedules'),
    staleTime: 30_000,
    enabled,
  })
}

export function useCreateCveSchedule() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateCveScheduleRequest) =>
      apiFetch<CveSchedule>('/api/security/cve/schedules', {
        method: 'POST',
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: cveSchedulesKey() })
    },
  })
}

export function useToggleCveSchedule() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      apiFetch<void>(`/api/security/cve/schedules/${id}`, {
        method: 'PUT',
        body: JSON.stringify({ enabled }),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: cveSchedulesKey() })
    },
  })
}

export function useDeleteCveSchedule() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/security/cve/schedules/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: cveSchedulesKey() })
    },
  })
}
