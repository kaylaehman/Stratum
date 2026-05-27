import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiFetch } from '../api'
import type {
  ScriptsResponse,
  Script,
  ScriptCreateRequest,
  RunScriptRequest,
  RunScriptResponse,
} from '../../types/api'

export function scriptsKey() {
  return ['scripts'] as const
}

export function useScripts() {
  return useQuery({
    queryKey: scriptsKey(),
    queryFn: () => apiGet<ScriptsResponse>('/api/scripts'),
    staleTime: 30_000,
  })
}

export function useCreateScript() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: ScriptCreateRequest) =>
      apiPost<Script>('/api/scripts', req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: scriptsKey() })
    },
  })
}

export function useUpdateScript() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, req }: { id: string; req: ScriptCreateRequest }) =>
      apiFetch<Script>(`/api/scripts/${id}`, {
        method: 'PUT',
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: scriptsKey() })
    },
  })
}

export function useDeleteScript() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/scripts/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: scriptsKey() })
    },
  })
}

export function useRunScript() {
  return useMutation({
    mutationFn: ({ id, req }: { id: string; req: RunScriptRequest }) =>
      apiPost<RunScriptResponse>(`/api/scripts/${id}/run`, req),
  })
}
