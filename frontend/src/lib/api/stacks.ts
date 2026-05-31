import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiFetch } from '../api'
import { TREE_KEY } from './tree'
import type {
  StackComposeResponse,
  StackEnvVarsResponse,
  StackDeployRequest,
  StackDeployResponse,
} from '../../types/api'

// ── Stack lifecycle ────────────────────────────────────────────────────────────

export type StackLifecycleAction = 'stop' | 'start' | 'restart'

export interface StackLifecycleRequest {
  action: StackLifecycleAction
}

export interface StackLifecycleResponse {
  action: StackLifecycleAction
  project: string
  output: string
}

interface StackLifecycleVars {
  nodeId: string
  project: string
  action: StackLifecycleAction
}

/** Start / stop / restart an entire compose stack (operator-gated on backend). */
export function useStackLifecycle() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, project, action }: StackLifecycleVars) =>
      apiPost<StackLifecycleResponse>(
        `/api/nodes/${encodeURIComponent(nodeId)}/stacks/${encodeURIComponent(project)}/lifecycle`,
        { action } satisfies StackLifecycleRequest,
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: TREE_KEY })
    },
  })
}

export function stackComposeKey(nodeId: string, project: string) {
  return ['stack-compose', nodeId, project] as const
}

export function stackEnvKey(nodeId: string, project: string) {
  return ['stack-env', nodeId, project] as const
}

/** Read the compose YAML + env-var metadata for a live stack. */
export function useStackCompose(nodeId: string, project: string, enabled = true) {
  return useQuery({
    queryKey: stackComposeKey(nodeId, project),
    queryFn: () =>
      apiGet<StackComposeResponse>(
        `/api/nodes/${encodeURIComponent(nodeId)}/stacks/${encodeURIComponent(project)}/compose`,
      ),
    enabled: enabled && !!nodeId && !!project,
    staleTime: 30_000,
    retry: false, // docker_not_available 409 should not retry
  })
}

/** List env vars (keys only, never values) for a (node, project). */
export function useStackEnvVars(nodeId: string, project: string, enabled = true) {
  return useQuery({
    queryKey: stackEnvKey(nodeId, project),
    queryFn: () =>
      apiGet<StackEnvVarsResponse>(
        `/api/nodes/${encodeURIComponent(nodeId)}/stacks/${encodeURIComponent(project)}/env`,
      ),
    enabled: enabled && !!nodeId && !!project,
    staleTime: 30_000,
  })
}

/** Redeploy a stack with updated compose YAML and env vars. Admin-only. */
export function useRedeployStack() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({
      nodeId,
      project,
      req,
    }: {
      nodeId: string
      project: string
      req: StackDeployRequest
    }) =>
      apiPost<StackDeployResponse>(
        `/api/nodes/${encodeURIComponent(nodeId)}/stacks/${encodeURIComponent(project)}/deploy`,
        req,
      ),
    onSuccess: (_data, { nodeId, project }) => {
      // Invalidate the tree so container statuses refresh.
      void queryClient.invalidateQueries({ queryKey: TREE_KEY })
      // Invalidate the compose/env caches for this project.
      void queryClient.invalidateQueries({ queryKey: stackComposeKey(nodeId, project) })
      void queryClient.invalidateQueries({ queryKey: stackEnvKey(nodeId, project) })
    },
  })
}

interface SetEnvVarVars {
  nodeId: string
  project: string
  key: string
  value?: string
  secretId?: string
}

/** Upsert one env var for a (node, project). */
export function useSetStackEnvVar() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, project, key, value, secretId }: SetEnvVarVars) =>
      apiFetch<void>(
        `/api/nodes/${encodeURIComponent(nodeId)}/stacks/${encodeURIComponent(project)}/env/${encodeURIComponent(key)}`,
        {
          method: 'PUT',
          body: JSON.stringify({ key, value: value ?? '', secret_id: secretId ?? '' }),
        },
      ),
    onSuccess: (_data, { nodeId, project }) => {
      void queryClient.invalidateQueries({ queryKey: stackEnvKey(nodeId, project) })
    },
  })
}

interface DeleteEnvVarVars {
  nodeId: string
  project: string
  key: string
}

/** Delete one env var for a (node, project). */
export function useDeleteStackEnvVar() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, project, key }: DeleteEnvVarVars) =>
      apiFetch<void>(
        `/api/nodes/${encodeURIComponent(nodeId)}/stacks/${encodeURIComponent(project)}/env/${encodeURIComponent(key)}`,
        { method: 'DELETE' },
      ),
    onSuccess: (_data, { nodeId, project }) => {
      void queryClient.invalidateQueries({ queryKey: stackEnvKey(nodeId, project) })
    },
  })
}
