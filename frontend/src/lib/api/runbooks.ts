import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiPut, apiDelete } from '../api'
import type { Runbook, RunbooksListResponse, RunbookRequest } from '../../types/api'

export const RUNBOOKS_KEY = ['runbooks'] as const

export function useRunbooks() {
  return useQuery({
    queryKey: RUNBOOKS_KEY,
    queryFn: () => apiGet<RunbooksListResponse>('/api/runbooks'),
    staleTime: 30_000,
  })
}

export function useCreateRunbook() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: RunbookRequest) => apiPost<Runbook>('/api/runbooks', body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: RUNBOOKS_KEY })
    },
  })
}

export function useUpdateRunbook() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: RunbookRequest }) =>
      apiPut<Runbook>(`/api/runbooks/${id}`, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: RUNBOOKS_KEY })
    },
  })
}

export function useDeleteRunbook() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => apiDelete<void>(`/api/runbooks/${id}`),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: RUNBOOKS_KEY })
    },
  })
}
