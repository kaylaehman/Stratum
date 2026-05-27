import { useMutation, useQueryClient } from '@tanstack/react-query'
import { apiPost } from '../api'
import { TREE_KEY } from './tree'

export type ContainerAction = 'start' | 'stop' | 'restart'

interface LifecycleVars {
  containerId: string
  action: ContainerAction
}

export function useContainerLifecycle() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ containerId, action }: LifecycleVars) =>
      apiPost<void>(`/api/containers/${encodeURIComponent(containerId)}/${action}`, null),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: TREE_KEY })
    },
  })
}
