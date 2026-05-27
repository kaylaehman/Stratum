import { useQuery } from '@tanstack/react-query'
import { apiGet } from '../lib/api'
import type { User } from '../types/api'

export function useMe() {
  return useQuery({
    queryKey: ['me'],
    queryFn: () => apiGet<User>('/api/me'),
  })
}
