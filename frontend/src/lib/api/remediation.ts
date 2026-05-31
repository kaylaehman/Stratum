import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost } from '../api'
import type {
  RemediationProposal,
  ProposalsResponse,
  GenerateProposalRequest,
} from '../../types/api'

export const PROPOSALS_KEY = ['remediation'] as const

export function proposalsKey(nodeId?: string) {
  return nodeId ? [...PROPOSALS_KEY, nodeId] : PROPOSALS_KEY
}

export function useProposals(nodeId?: string) {
  const params = nodeId ? `?node_id=${encodeURIComponent(nodeId)}` : ''
  return useQuery({
    queryKey: proposalsKey(nodeId),
    queryFn: () => apiGet<ProposalsResponse>(`/api/remediation${params}`),
    staleTime: 15_000,
  })
}

export function useProposal(id: string) {
  return useQuery({
    queryKey: [...PROPOSALS_KEY, 'item', id],
    queryFn: () => apiGet<RemediationProposal>(`/api/remediation/${id}`),
    enabled: !!id,
    staleTime: 10_000,
  })
}

export function useGenerateProposal() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: GenerateProposalRequest) =>
      apiPost<RemediationProposal>('/api/remediation', req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: PROPOSALS_KEY })
    },
  })
}

export function useApproveProposal() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      apiPost<RemediationProposal>(`/api/remediation/${id}/approve`, {}),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: PROPOSALS_KEY })
    },
  })
}

export function useRejectProposal() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      apiPost<RemediationProposal>(`/api/remediation/${id}/reject`, {}),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: PROPOSALS_KEY })
    },
  })
}

export function useExecuteProposal() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      apiPost<RemediationProposal>(`/api/remediation/${id}/execute`, {}),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: PROPOSALS_KEY })
    },
  })
}
