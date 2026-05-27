import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiPut, apiDelete } from '../api'
import type {
  UsersListResponse,
  SessionsListResponse,
  CreateUserRequest,
  UpdateRoleRequest,
  User,
} from '../../types/api'

export const USERS_KEY = ['users'] as const
export const SESSIONS_KEY = ['sessions'] as const

// ── Users ────────────────────────────────────────────────────────────────────

export function useUsers() {
  return useQuery({
    queryKey: USERS_KEY,
    queryFn: () => apiGet<UsersListResponse>('/api/users'),
    staleTime: 30_000,
  })
}

export function useCreateUser() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateUserRequest) => apiPost<User>('/api/users', req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: USERS_KEY })
    },
  })
}

export function useUpdateUserRole() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, role }: { id: string; role: UpdateRoleRequest['role'] }) =>
      apiPut<User>(`/api/users/${encodeURIComponent(id)}/role`, { role } satisfies UpdateRoleRequest),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: USERS_KEY })
    },
  })
}

export function useDeleteUser() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => apiDelete<void>(`/api/users/${encodeURIComponent(id)}`),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: USERS_KEY })
    },
  })
}

// ── Sessions ─────────────────────────────────────────────────────────────────

export function useSessions() {
  return useQuery({
    queryKey: SESSIONS_KEY,
    queryFn: () => apiGet<SessionsListResponse>('/api/sessions'),
    staleTime: 30_000,
  })
}

export function useRevokeSession() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => apiDelete<void>(`/api/sessions/${encodeURIComponent(id)}`),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: SESSIONS_KEY })
    },
  })
}
