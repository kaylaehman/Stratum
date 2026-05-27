import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiFetch } from '../api'
import type {
  SecretsResponse,
  CreateSecretGroupRequest,
  SetSecretRequest,
  ImportSecretsRequest,
  ImportSecretsResponse,
  RevealResponse,
} from '../../types/api'

export function secretsKey() {
  return ['secrets'] as const
}

export function useSecrets() {
  return useQuery({
    queryKey: secretsKey(),
    queryFn: () => apiGet<SecretsResponse>('/api/secrets'),
    staleTime: 30_000,
  })
}

export function useCreateGroup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: CreateSecretGroupRequest) =>
      apiPost<{ id: string; name: string; description: string; secrets: [] }>(
        '/api/secret-groups',
        body,
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: secretsKey() })
    },
  })
}

export function useDeleteGroup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (groupId: string) =>
      apiFetch<void>(`/api/secret-groups/${groupId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: secretsKey() })
    },
  })
}

interface SetSecretVars {
  groupId: string
  body: SetSecretRequest
}

export function useSetSecret() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ groupId, body }: SetSecretVars) =>
      apiPost<void>(`/api/secret-groups/${groupId}/secrets`, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: secretsKey() })
    },
  })
}

interface ImportSecretsVars {
  groupId: string
  body: ImportSecretsRequest
}

export function useImportSecrets() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ groupId, body }: ImportSecretsVars) =>
      apiPost<ImportSecretsResponse>(`/api/secret-groups/${groupId}/import`, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: secretsKey() })
    },
  })
}

export function useDeleteSecret() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (secretId: string) =>
      apiFetch<void>(`/api/secrets/${secretId}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: secretsKey() })
    },
  })
}

/**
 * Reveal is a one-shot mutation — NOT a query.
 * The caller consumes the returned value transiently in component state.
 * React Query never caches the plaintext value.
 */
export function useRevealSecret() {
  return useMutation({
    mutationFn: (secretId: string) =>
      apiPost<RevealResponse>(`/api/secrets/${secretId}/reveal`, {}),
  })
}
