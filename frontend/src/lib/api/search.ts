import { useQuery } from '@tanstack/react-query'
import { apiGet } from '../api'
import type { SearchResponse } from '../../types/api'

export function searchKey(query: string) {
  return ['search', query] as const
}

export function useSearch(query: string) {
  return useQuery({
    queryKey: searchKey(query),
    queryFn: () => apiGet<SearchResponse>(`/api/search?q=${encodeURIComponent(query)}`),
    enabled: query.trim().length > 0,
    staleTime: 10_000,
    placeholderData: (prev) => prev,
  })
}
