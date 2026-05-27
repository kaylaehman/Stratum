import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiFetch } from '../api'
import type { WebhooksResponse, WebhookRequest } from '../../types/api'

export function webhooksKey() {
  return ['webhooks'] as const
}

export function useWebhooks() {
  return useQuery({
    queryKey: webhooksKey(),
    queryFn: () => apiGet<WebhooksResponse>('/api/webhooks'),
    staleTime: 30_000,
  })
}

export function useCreateWebhook() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: WebhookRequest) => apiPost('/api/webhooks', body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: webhooksKey() })
    },
  })
}

interface UpdateWebhookVars {
  id: string
  body: WebhookRequest
}

export function useUpdateWebhook() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: UpdateWebhookVars) =>
      apiFetch<void>(`/api/webhooks/${id}`, {
        method: 'PUT',
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: webhooksKey() })
    },
  })
}

export function useDeleteWebhook() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/webhooks/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: webhooksKey() })
    },
  })
}

export function useTestWebhook() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/webhooks/${id}/test`, { method: 'POST' }),
  })
}
