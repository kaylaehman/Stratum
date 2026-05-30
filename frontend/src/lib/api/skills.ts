import { useQuery } from '@tanstack/react-query'
import { apiGet } from '../api'
import type { SkillsResponse, SkillDetail } from '../../types/api'

export function skillsKey() {
  return ['skills'] as const
}

export function skillKey(id: string) {
  return ['skills', id] as const
}

export function useSkills() {
  return useQuery({
    queryKey: skillsKey(),
    queryFn: () => apiGet<SkillsResponse>('/api/skills'),
    staleTime: 60_000,
  })
}

export function useSkill(id: string, enabled: boolean) {
  return useQuery({
    queryKey: skillKey(id),
    queryFn: () => apiGet<SkillDetail>(`/api/skills/${encodeURIComponent(id)}`),
    enabled: enabled && id.length > 0,
    staleTime: 120_000,
  })
}
