import { useMutation, useQueryClient } from '@tanstack/react-query'
import { apiPost } from '../api'
import { TREE_KEY } from './tree'

// ── Local types ────────────────────────────────────────────────────────────────

export type OrchTargetKind = 'stack' | 'node'
export type OrchAction = 'start' | 'stop' | 'restart'

export interface OrchStep {
  kind: string
  id: string
  name: string
  action: string
  order: number
}

export interface OrchPlan {
  target: string
  action: OrchAction
  steps: OrchStep[]
  /** Dependency cycles detected (array of cycle paths). */
  cycles: string[][]
}

export interface OrchStepResult {
  step: OrchStep
  ok: boolean
  error?: string
  duration_ms: number
}

export interface OrchExecuteResult {
  results: OrchStepResult[]
}

export interface OrchPlanRequest {
  target_kind: OrchTargetKind
  target_id: string
  project?: string
  action: OrchAction
}

// ── Hooks ──────────────────────────────────────────────────────────────────────

/** Dry-run: get the ordered plan without executing. */
export function useOrchPlan() {
  return useMutation({
    mutationFn: (req: OrchPlanRequest) =>
      apiPost<OrchPlan>('/api/orchestration/plan', req),
  })
}

/** Execute the ordered orchestration (step-up handled by apiFetch for stop). */
export function useOrchExecute() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: OrchPlanRequest) =>
      apiPost<OrchExecuteResult>('/api/orchestration/execute', req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: TREE_KEY })
    },
  })
}

export interface DrainPlanRequest {
  dry_run: true
}

export interface DrainExecuteRequest {
  dry_run?: false
}

/** Dry-run drain: returns a Plan describing what will stop. */
export function useDrainPlan() {
  return useMutation({
    mutationFn: (nodeId: string) =>
      apiPost<OrchPlan>(
        `/api/nodes/${encodeURIComponent(nodeId)}/drain`,
        { dry_run: true } satisfies DrainPlanRequest,
      ),
  })
}

/** Execute drain (admin + step-up, handled by apiFetch 428 interceptor). */
export function useDrainExecute() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (nodeId: string) =>
      apiPost<OrchExecuteResult>(
        `/api/nodes/${encodeURIComponent(nodeId)}/drain`,
        {} satisfies DrainExecuteRequest,
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: TREE_KEY })
    },
  })
}
