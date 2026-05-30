import { useMutation, useQueryClient } from '@tanstack/react-query'
import { apiPost } from '../api'
import { TREE_KEY } from './tree'
import type { NodePowerAction, NodePowerActionResponse } from '../../types/api'

interface NodePowerVars {
  nodeId: string
  action: NodePowerAction
}

export function useNodePowerAction() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, action }: NodePowerVars) =>
      apiPost<NodePowerActionResponse>(
        `/api/nodes/${encodeURIComponent(nodeId)}/power/${action}`,
        null,
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: TREE_KEY })
    },
  })
}
