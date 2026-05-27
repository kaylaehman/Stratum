import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPut } from '../api'
import type { FeaturesResponse, SetFeatureRequest } from '../../types/api'

export const FEATURES_KEY = ['features'] as const

export function useFeatures() {
  return useQuery({
    queryKey: FEATURES_KEY,
    queryFn: () => apiGet<FeaturesResponse>('/api/features'),
    staleTime: 60_000,
  })
}

/**
 * Returns whether a named feature flag is enabled.
 * Defaults to `true` while the list is loading so nothing flickers off
 * before the first response arrives.
 */
export function useFeatureEnabled(key: string): boolean {
  const { data, isLoading } = useFeatures()
  if (isLoading || !data) return true
  const flag = data.features.find((f) => f.key === key)
  return flag?.enabled ?? true
}

export function useSetFeature() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ key, enabled }: { key: string; enabled: boolean }) =>
      apiPut<FeaturesResponse>(`/api/features/${key}`, { enabled } satisfies SetFeatureRequest),
    onSuccess: (updated) => {
      queryClient.setQueryData(FEATURES_KEY, updated)
    },
    onError: () => {
      void queryClient.invalidateQueries({ queryKey: FEATURES_KEY })
    },
  })
}
