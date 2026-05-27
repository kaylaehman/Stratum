import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiFetch } from '../api'
import type {
  NodeView,
  NodesListResponse,
  CreateNodeRequest,
  RenameNodeRequest,
  ProbePreviewRequest,
  PreviewResult,
} from '../../types/api'

const NODES_KEY = ['nodes'] as const

export function useNodes() {
  return useQuery({
    queryKey: NODES_KEY,
    queryFn: () => apiGet<NodesListResponse>('/api/nodes').then((r) => r.nodes),
  })
}

export function useNode(id: string) {
  return useQuery({
    queryKey: ['nodes', id],
    queryFn: () => apiGet<NodeView>(`/api/nodes/${id}`),
    enabled: Boolean(id),
  })
}

export function useCreateNode() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: CreateNodeRequest) =>
      apiPost<NodeView>('/api/nodes', body),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: NODES_KEY })
    },
  })
}

export function useRenameNode() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) =>
      apiFetch<NodeView>(`/api/nodes/${id}`, {
        method: 'PUT',
        body: JSON.stringify({ name } satisfies RenameNodeRequest),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: NODES_KEY })
    },
  })
}

export function useDeleteNode() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/nodes/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: NODES_KEY })
    },
  })
}

export function useReprobeNode() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      apiPost<NodeView>(`/api/nodes/${id}/probe`, null),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: NODES_KEY })
    },
  })
}

export function useProbePreview() {
  return useMutation({
    mutationFn: (body: ProbePreviewRequest) =>
      apiPost<PreviewResult>('/api/nodes/probe-preview', body),
  })
}
