import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPut, apiPost } from '../api'
import type {
  AIConfig,
  SetAIConfigRequest,
  AIAskRequest,
  AIAskResponse,
  AIOAuthStartResponse,
} from '../../types/api'

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

// --- Claude OAuth ("sign in with Claude") ---

export function useAIOAuthStart() {
  return useMutation<AIOAuthStartResponse, unknown, void>({
    mutationFn: () => apiGet<AIOAuthStartResponse>('/api/ai/oauth/start'),
  })
}

export function useAIOAuthExchange() {
  const qc = useQueryClient()
  return useMutation<AIConfig, unknown, { code: string; verifier: string; state: string }>({
    mutationFn: (body) => apiPost<AIConfig>('/api/ai/oauth/exchange', body),
    onSuccess: (data) => qc.setQueryData(AI_CONFIG_KEY, data),
  })
}

export function useAIOAuthSetToken() {
  const qc = useQueryClient()
  return useMutation<AIConfig, unknown, { access_token: string; refresh_token?: string }>({
    mutationFn: (body) => apiPost<AIConfig>('/api/ai/oauth/token', body),
    onSuccess: (data) => qc.setQueryData(AI_CONFIG_KEY, data),
  })
}

export function useAIOAuthDisconnect() {
  const qc = useQueryClient()
  return useMutation<AIConfig, unknown, void>({
    mutationFn: () => apiPost<AIConfig>('/api/ai/oauth/disconnect', null),
    onSuccess: (data) => qc.setQueryData(AI_CONFIG_KEY, data),
  })
}
