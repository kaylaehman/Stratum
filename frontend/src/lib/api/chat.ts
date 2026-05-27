import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPut } from '../api'
import type { ChatConfig, SetChatConfigRequest } from '../../types/api'

export const CHAT_CONFIG_KEY = ['chat', 'config'] as const

export function useChatConfig() {
  return useQuery({
    queryKey: CHAT_CONFIG_KEY,
    queryFn: () => apiGet<ChatConfig>('/api/chat/config'),
    staleTime: 30_000,
  })
}

export function useSetChatConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: SetChatConfigRequest) =>
      apiPut<ChatConfig>('/api/chat/config', req),
    onSuccess: (updated) => {
      queryClient.setQueryData(CHAT_CONFIG_KEY, updated)
    },
    onError: () => {
      void queryClient.invalidateQueries({ queryKey: CHAT_CONFIG_KEY })
    },
  })
}
