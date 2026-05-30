import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiPut, apiDelete } from '../api'
import type {
  SkillsResponse,
  SkillDetail,
  SkillRaw,
  SkillGenerateRequest,
  SkillGenerateResult,
} from '../../types/api'

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

/** Load the raw YAML source for a skill (custom: verbatim; builtin: re-marshalled). */
export function fetchSkillRaw(id: string): Promise<SkillRaw> {
  return apiGet<SkillRaw>(`/api/skills/${encodeURIComponent(id)}/raw`)
}

export function useCreateSkill() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (yaml: string) => apiPost<SkillDetail>('/api/skills', { yaml }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: skillsKey() })
    },
  })
}

export function useUpdateSkill() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, yaml }: { id: string; yaml: string }) =>
      apiPut<SkillDetail>(`/api/skills/${encodeURIComponent(id)}`, { yaml }),
    onSuccess: (_data, { id }) => {
      void queryClient.invalidateQueries({ queryKey: skillsKey() })
      void queryClient.invalidateQueries({ queryKey: skillKey(id) })
    },
  })
}

export function useDeleteSkill() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => apiDelete<void>(`/api/skills/${encodeURIComponent(id)}`),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: skillsKey() })
    },
  })
}

export function useGenerateSkill() {
  return useMutation({
    mutationFn: (req: SkillGenerateRequest) =>
      apiPost<SkillGenerateResult>('/api/skills/generate', req),
  })
}
