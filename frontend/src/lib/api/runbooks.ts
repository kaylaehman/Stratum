import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiPut, apiDelete } from '../api'
import type { Runbook, RunbooksListResponse, RunbookRequest } from '../../types/api'

// Re-export shared types so the page can import locally without touching types/api.ts.
export type { Runbook, RunbooksListResponse, RunbookRequest }

// Local types: runbook validation result (from POST /api/runbooks/{id}/validate).
export interface RunbookLintError {
  step_index: number
  step: string
  risk: string
  message: string
}

export interface RunbookValidationResult {
  valid: boolean
  errors: RunbookLintError[]
  warnings: RunbookLintError[]
  step_risks: string[]
}

export const RUNBOOKS_KEY = ['runbooks'] as const

export function useRunbooks() {
  return useQuery({
    queryKey: RUNBOOKS_KEY,
    queryFn: () => apiGet<RunbooksListResponse>('/api/runbooks'),
    staleTime: 30_000,
  })
}

export function useRunbook(id: string) {
  return useQuery({
    queryKey: [...RUNBOOKS_KEY, id],
    queryFn: () => apiGet<Runbook>(`/api/runbooks/${id}`),
    enabled: !!id,
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

export function useValidateRunbook() {
  return useMutation({
    mutationFn: (id: string) =>
      apiPost<RunbookValidationResult>(`/api/runbooks/${id}/validate`, {}),
  })
}
