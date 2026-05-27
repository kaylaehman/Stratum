import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiFetch } from '../api'
import type { Bookmark, BookmarksResponse, CreateBookmarkRequest } from '../../types/api'

export function bookmarksKey() {
  return ['bookmarks'] as const
}

export function useBookmarks() {
  return useQuery({
    queryKey: bookmarksKey(),
    queryFn: () => apiGet<BookmarksResponse>('/api/bookmarks'),
    staleTime: 30_000,
  })
}

export function useAddBookmark() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateBookmarkRequest) =>
      apiPost<Bookmark>('/api/bookmarks', req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: bookmarksKey() })
    },
  })
}

export function useRemoveBookmark() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/bookmarks/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: bookmarksKey() })
    },
  })
}
