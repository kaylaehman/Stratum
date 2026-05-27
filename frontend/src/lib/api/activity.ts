import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { apiGet } from '../api'
import { useAuthStore } from '../../store/auth'
import type {
  ActivityListResponse,
  ActivityActionsResponse,
  ActivityFilters,
} from '../../types/api'

export function buildActivityQuery(filters: ActivityFilters): string {
  const params = new URLSearchParams()
  if (filters.user) params.set('user', filters.user)
  if (filters.action) params.set('action', filters.action)
  if (filters.action_prefix) params.set('action_prefix', filters.action_prefix)
  if (filters.target_type) params.set('target_type', filters.target_type)
  if (filters.result) params.set('result', filters.result)
  if (filters.from) params.set('from', filters.from)
  if (filters.to) params.set('to', filters.to)
  if (filters.q) params.set('q', filters.q)
  return params.toString()
}

export function activityKey(filters: ActivityFilters) {
  return ['activity', filters] as const
}

export function activityActionsKey() {
  return ['activity', 'actions'] as const
}

export function useActivity(filters: ActivityFilters) {
  return useInfiniteQuery({
    queryKey: activityKey(filters),
    queryFn: ({ pageParam }) => {
      const qs = buildActivityQuery(filters)
      const cursor = pageParam ? `cursor=${encodeURIComponent(pageParam)}&` : ''
      return apiGet<ActivityListResponse>(`/api/activity?${cursor}${qs}`)
    },
    getNextPageParam: (lastPage) => lastPage.next_cursor || undefined,
    initialPageParam: '',
    staleTime: 15_000,
  })
}

export function useActivityActions() {
  return useQuery({
    queryKey: activityActionsKey(),
    queryFn: () => apiGet<ActivityActionsResponse>('/api/activity/actions'),
    staleTime: 60_000,
  })
}

export async function exportActivityCsv(filters: ActivityFilters): Promise<void> {
  const qs = buildActivityQuery(filters)
  const url = `/api/activity/export.csv${qs ? `?${qs}` : ''}`
  const token = useAuthStore.getState().accessToken

  const headers: Record<string, string> = {}
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch(url, { method: 'GET', headers })
  if (!res.ok) {
    throw new Error(`Export failed: ${res.status}`)
  }

  const blob = await res.blob()
  const objectUrl = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = objectUrl
  anchor.download = 'activity.csv'
  document.body.appendChild(anchor)
  anchor.click()
  document.body.removeChild(anchor)
  URL.revokeObjectURL(objectUrl)
}
