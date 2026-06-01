import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiPut, apiDelete } from '../api'

// ---- Types (local — do NOT edit types/api.ts) ----

export type AlertSeverity = 'info' | 'warning' | 'critical'
export type DeliveryStatus = 'delivered' | 'suppressed_dedup' | 'suppressed_quiet'

export interface QuietHours {
  start_min: number   // minutes from midnight, e.g. 1320 = 22:00
  end_min: number     // minutes from midnight, e.g. 480 = 08:00
  tz: string          // IANA tz, e.g. "America/New_York"
  allow_critical: boolean
}

export interface PolicyEscalation {
  after_sec: number
  channels: string[]  // webhook IDs
}

export interface PolicyMatch {
  sources: string[]   // trigger source keys; empty = all
  key_glob: string    // glob pattern; empty = all
}

export interface AlertPolicy {
  id: string
  name: string
  enabled: boolean
  min_severity: AlertSeverity
  channels: string[]        // webhook IDs
  match: PolicyMatch
  quiet_hours: QuietHours | null
  dedup_window_sec: number
  escalate: PolicyEscalation | null
}

export interface AlertPoliciesResponse {
  policies: AlertPolicy[]
}

export interface AlertPolicyRequest {
  name: string
  enabled: boolean
  min_severity: AlertSeverity
  channels: string[]
  match: PolicyMatch
  quiet_hours: QuietHours | null
  dedup_window_sec: number
  escalate: PolicyEscalation | null
}

export interface AlertDelivery {
  policy_id: string
  alert_key: string
  severity: AlertSeverity
  channel: string
  status: DeliveryStatus
  created_at: string
}

export interface AlertDeliveriesResponse {
  deliveries: AlertDelivery[]
}

// ---- Query keys ----

export const alertPoliciesKey = () => ['alert-policies'] as const
export const alertDeliveriesKey = (limit: number) => ['alert-deliveries', limit] as const

// ---- Hooks ----

export function useAlertPolicies() {
  return useQuery({
    queryKey: alertPoliciesKey(),
    queryFn: () => apiGet<AlertPoliciesResponse>('/api/alert-policies'),
    staleTime: 30_000,
  })
}

export function useAlertDeliveries(limit = 50) {
  return useQuery({
    queryKey: alertDeliveriesKey(limit),
    queryFn: () => apiGet<AlertDeliveriesResponse>(`/api/alert-deliveries?limit=${limit}`),
    staleTime: 15_000,
    refetchInterval: 30_000,
  })
}

export function useCreateAlertPolicy() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: AlertPolicyRequest) => apiPost<AlertPolicy>('/api/alert-policies', body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: alertPoliciesKey() })
    },
  })
}

interface UpdatePolicyVars {
  id: string
  body: AlertPolicyRequest
}

export function useUpdateAlertPolicy() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: UpdatePolicyVars) =>
      apiPut<AlertPolicy>(`/api/alert-policies/${id}`, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: alertPoliciesKey() })
    },
  })
}

export function useDeleteAlertPolicy() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => apiDelete<void>(`/api/alert-policies/${id}`),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: alertPoliciesKey() })
    },
  })
}
