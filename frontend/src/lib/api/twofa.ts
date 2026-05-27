import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost } from '../api'
import type { TwoFAStatus, TwoFASetupResponse, TwoFACodeRequest } from '../../types/api'

export const TWOFA_STATUS_KEY = ['me', '2fa'] as const

export function useTwoFAStatus() {
  return useQuery({
    queryKey: TWOFA_STATUS_KEY,
    queryFn: () => apiGet<TwoFAStatus>('/api/me/2fa'),
    staleTime: 30_000,
  })
}

export function useTwoFASetup() {
  return useMutation({
    mutationFn: () => apiPost<TwoFASetupResponse>('/api/me/2fa/setup', {}),
  })
}

export function useEnableTwoFA() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: TwoFACodeRequest) =>
      apiPost<void>('/api/me/2fa/enable', req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: TWOFA_STATUS_KEY })
    },
  })
}

export function useDisableTwoFA() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: TwoFACodeRequest) =>
      apiPost<void>('/api/me/2fa/disable', req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: TWOFA_STATUS_KEY })
    },
  })
}
