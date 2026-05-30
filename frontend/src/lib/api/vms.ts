import { useMutation, useQueryClient } from '@tanstack/react-query'
import { apiPost } from '../api'
import { TREE_KEY } from './tree'
import type { VMPowerActionResponse } from '../../types/api'

export type VMAction = 'start' | 'stop' | 'shutdown' | 'reboot'

interface VMPowerVars {
  nodeId: string
  vmid: number
  action: VMAction
}

export function useVMPowerAction() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, vmid, action }: VMPowerVars) =>
      apiPost<VMPowerActionResponse>(
        `/api/nodes/${encodeURIComponent(nodeId)}/vms/${vmid}/${action}`,
        null,
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: TREE_KEY })
    },
  })
}
