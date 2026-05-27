import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiPut, apiDelete } from '../api'
import type {
  Memory,
  MemoryListResponse,
  MemoryScope,
  CreateMemoryRequest,
  UpdateMemoryRequest,
} from '../../types/api'

export function memoryKey(scope: MemoryScope, scopeId?: string) {
  return ['memory', scope, scopeId ?? null] as const
}

export function useMemory(scope: MemoryScope, scopeId?: string) {
  const params = new URLSearchParams({ scope })
  if (scopeId) params.set('scope_id', scopeId)
  return useQuery({
    queryKey: memoryKey(scope, scopeId),
    queryFn: () => apiGet<MemoryListResponse>(`/api/memory?${params.toString()}`),
    staleTime: 30_000,
  })
}

export function useCreateMemory(scope: MemoryScope, scopeId?: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: CreateMemoryRequest) => apiPost<Memory>('/api/memory', body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: memoryKey(scope, scopeId) })
    },
  })
}

export function useUpdateMemory(scope: MemoryScope, scopeId?: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: UpdateMemoryRequest }) =>
      apiPut<Memory>(`/api/memory/${id}`, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: memoryKey(scope, scopeId) })
    },
  })
}

export function useDeleteMemory(scope: MemoryScope, scopeId?: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => apiDelete<void>(`/api/memory/${id}`),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: memoryKey(scope, scopeId) })
    },
  })
}
