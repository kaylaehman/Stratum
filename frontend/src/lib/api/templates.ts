import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost, apiFetch } from '../api'
import type {
  TemplatesResponse,
  TemplateWithVersions,
  TemplateCreateRequest,
  TemplateRenderRequest,
  TemplateRenderResponse,
  TemplateDeployRequest,
  TemplateDeployResponse,
  Template,
} from '../../types/api'

export function templatesKey() {
  return ['templates'] as const
}

export function templateKey(id: string) {
  return ['templates', id] as const
}

export function useTemplates() {
  return useQuery({
    queryKey: templatesKey(),
    queryFn: () => apiGet<TemplatesResponse>('/api/templates'),
    staleTime: 30_000,
  })
}

export function useTemplate(id: string) {
  return useQuery({
    queryKey: templateKey(id),
    queryFn: () => apiGet<TemplateWithVersions>(`/api/templates/${id}`),
    enabled: !!id,
    staleTime: 30_000,
  })
}

export function useCreateTemplate() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: TemplateCreateRequest) =>
      apiPost<Template>('/api/templates', req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: templatesKey() })
    },
  })
}

export function useUpdateTemplate() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, req }: { id: string; req: TemplateCreateRequest }) =>
      apiFetch<Template>(`/api/templates/${id}`, {
        method: 'PUT',
        body: JSON.stringify(req),
      }),
    onSuccess: (_data, { id }) => {
      void queryClient.invalidateQueries({ queryKey: templatesKey() })
      void queryClient.invalidateQueries({ queryKey: templateKey(id) })
    },
  })
}

export function useDeleteTemplate() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/templates/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: templatesKey() })
    },
  })
}

export function useRenderTemplate() {
  return useMutation({
    mutationFn: ({ id, req }: { id: string; req: TemplateRenderRequest }) =>
      apiPost<TemplateRenderResponse>(`/api/templates/${id}/render`, req),
  })
}

export function useDeployTemplate() {
  return useMutation({
    mutationFn: ({ id, req }: { id: string; req: TemplateDeployRequest }) =>
      apiPost<TemplateDeployResponse>(`/api/templates/${id}/deploy`, req),
  })
}
