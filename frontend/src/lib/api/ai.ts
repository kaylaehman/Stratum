import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPut, apiPost } from '../api'
import type { AIConfig, SetAIConfigRequest, AIAskRequest, AIAskResponse } from '../../types/api'

const AI_CONFIG_KEY = ['ai', 'config'] as const

export function useAIConfig() {
  return useQuery<AIConfig>({
    queryKey: AI_CONFIG_KEY,
    queryFn: () => apiGet<AIConfig>('/api/ai/config'),
    retry: false,
  })
}

export function useSetAIConfig() {
  const qc = useQueryClient()
  return useMutation<AIConfig, unknown, SetAIConfigRequest>({
    mutationFn: (body) => apiPut<AIConfig>('/api/ai/config', body),
    onSuccess: (data) => {
      qc.setQueryData(AI_CONFIG_KEY, data)
    },
  })
}

export function useAsk() {
  return useMutation<AIAskResponse, unknown, AIAskRequest>({
    mutationFn: (body) => apiPost<AIAskResponse>('/api/ai/ask', body),
  })
}
