import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiFetch } from '../api'
import type { SSHKeysResponse, DeleteSSHKeyRequest } from '../../types/api'

export function sshKeysKey(nodeId: string) {
  return ['sshkeys', nodeId] as const
}

export function useSSHKeys(nodeId: string | undefined) {
  return useQuery({
    queryKey: sshKeysKey(nodeId ?? ''),
    queryFn: () => apiGet<SSHKeysResponse>(`/api/nodes/${nodeId}/sshkeys`),
    enabled: Boolean(nodeId),
    retry: false,
  })
}

interface DeleteSSHKeyVars {
  nodeId: string
  request: DeleteSSHKeyRequest
}

export function useDeleteSSHKey() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ nodeId, request }: DeleteSSHKeyVars) =>
      apiFetch<void>(`/api/nodes/${nodeId}/sshkeys/delete`, {
        method: 'POST',
        body: JSON.stringify(request),
      }),
    onSuccess: (_data, { nodeId }) => {
      void queryClient.invalidateQueries({ queryKey: sshKeysKey(nodeId) })
    },
  })
}
