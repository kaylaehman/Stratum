import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiFetch, apiPut } from '../api'
import type {
  SecretsResponse,
  CreateSecretGroupRequest,
  SetSecretRequest,
  ImportSecretsRequest,
  ImportSecretsResponse,
  RevealResponse,
} from '../../types/api'

// ---- Local types for C5 features (expiry + plaintext scanner) ----
// These are NOT added to types/api.ts per swarm isolation rules.

export type ExpiryStatus = 'none' | 'ok' | 'warning' | 'expired'

export interface SecretExpiryMeta {
  id: string
  key: string
  group_id: string
  group_name: string
  expires_at: string | null
  rotated_at: string | null
  status: ExpiryStatus
}

export interface ExpiringSecretsResponse {
  secrets: SecretExpiryMeta[]
}

export interface SetExpiryRequest {
  expires_at?: string | null
  rotated_at?: string | null
}

export interface PlaintextFinding {
  path: string
  line: number
  key_name: string
  reason: string
}

export interface ScanResponse {
  findings: PlaintextFinding[]
}

// ---- Query keys ----

export function secretsKey() {
  return ['secrets'] as const
}

export function expiringSecretsKey() {
  return ['secrets', 'expiring'] as const
}

export function scanKey(nodeId: string) {
  return ['secrets', 'scan', nodeId] as const
}

// ---- Existing hooks ----

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

// ---- C5: Expiry hooks ----

/** Fetch all secrets that have expiry metadata — metadata only, no values. */
export function useExpiringSecrets() {
  return useQuery({
    queryKey: expiringSecretsKey(),
    queryFn: () => apiGet<ExpiringSecretsResponse>('/api/secrets/expiring'),
    staleTime: 60_000,
  })
}

interface SetExpiryVars {
  secretId: string
  body: SetExpiryRequest
}

/** Admin-only: set or clear expiry / rotated_at on a secret. Audited server-side. */
export function useSetExpiry() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ secretId, body }: SetExpiryVars) =>
      apiPut<void>(`/api/secrets/${secretId}/expiry`, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: secretsKey() })
      void queryClient.invalidateQueries({ queryKey: expiringSecretsKey() })
    },
  })
}

// ---- C5: Plaintext scanner hook ----

/**
 * On-demand scan for plaintext secrets on a node.
 * Returns findings with path, line, key_name, reason — never values.
 * Implemented as a mutation so the user explicitly triggers each scan.
 */
export function useScanNode() {
  return useMutation({
    mutationFn: (nodeId: string) =>
      apiGet<ScanResponse>(`/api/secrets/scan?node=${encodeURIComponent(nodeId)}`),
  })
}
