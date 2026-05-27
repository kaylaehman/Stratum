import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiFetch } from '../api'
import type { ScheduleResponse, SetCronRequest } from '../../types/api'

export function scheduleKey(nodeId: string) {
  return ['schedule', nodeId] as const
}

export function useSchedule(nodeId: string | undefined) {
  return useQuery({
    queryKey: scheduleKey(nodeId ?? ''),
    queryFn: () => apiGet<ScheduleResponse>(`/api/nodes/${nodeId}/schedule`),
    enabled: Boolean(nodeId),
    retry: false,
  })
}

interface SetCronVars {
  nodeId: string
  request: SetCronRequest
}

export function useSetCron() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, request }: SetCronVars) =>
      apiFetch<void>(`/api/nodes/${nodeId}/cron`, {
        method: 'PUT',
        body: JSON.stringify(request),
      }),
    onSuccess: (_data, { nodeId }) => {
      void queryClient.invalidateQueries({ queryKey: scheduleKey(nodeId) })
    },
  })
}
