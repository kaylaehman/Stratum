import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPut, apiPost } from '../api'

// ---- Types (local — do NOT edit types/api.ts) ----

export type AutomationCategory = 'self_heal' | 'update' | 'security' | 'maintenance'
export type AutomationStatus = '' | 'ok' | 'error' | 'skipped' | 'running'

export interface AutomationView {
  key: string
  label: string
  description: string
  category: AutomationCategory
  enabled: boolean
  interval_seconds: number
  config: Record<string, unknown>
  last_run: string | null
  last_status: AutomationStatus
  last_detail: string
}

export interface AutomationsResponse {
  automations: AutomationView[]
}

export interface UpdateAutomationBody {
  enabled?: boolean
  interval_seconds?: number
  config?: Record<string, unknown>
}

// ---- Query keys ----

export function automationsKey() {
  return ['automations'] as const
}

// ---- Hooks ----

export function useAutomations() {
  return useQuery({
    queryKey: automationsKey(),
    queryFn: () => apiGet<AutomationsResponse>('/api/automations'),
    staleTime: 30_000,
  })
}

export function useUpdateAutomation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ key, body }: { key: string; body: UpdateAutomationBody }) =>
      apiPut<AutomationView>(`/api/automations/${key}`, body),
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: automationsKey() })
    },
  })
}

export function useRunAutomation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (key: string) =>
      apiPost<void>(`/api/automations/${key}/run`, null),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: automationsKey() })
    },
  })
}
