import { useMutation, useQueryClient } from '@tanstack/react-query'
import { apiPost } from '../api'
import { TREE_KEY } from './tree'
import type { BulkRequest, BulkResponse } from '../../types/api'

export function useBulkContainers() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: BulkRequest) =>
      apiPost<BulkResponse>('/api/containers/bulk', req),
    onSuccess: (_data, variables) => {
      if (!variables.dry_run) {
        void queryClient.invalidateQueries({ queryKey: TREE_KEY })
      }
    },
  })
}
