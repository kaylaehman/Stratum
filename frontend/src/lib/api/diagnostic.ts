import { useMutation } from '@tanstack/react-query'
import { apiPost } from '../api'
import type { DiagnosticResult } from '../../types/api'

export interface RunDiagnosticRequest {
  host_path: string
}

export async function runDiagnostic(
  containerId: string,
  hostPath: string,
): Promise<DiagnosticResult> {
  return apiPost<DiagnosticResult>(
    `/api/containers/${containerId}/diagnostic`,
    { host_path: hostPath } satisfies RunDiagnosticRequest,
  )
}

export function useDiagnostic(containerId: string) {
  return useMutation({
    mutationFn: (hostPath: string) => runDiagnostic(containerId, hostPath),
  })
}
