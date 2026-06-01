import { useQuery } from '@tanstack/react-query'
import { apiGet } from '../api'

// ── Local types (do NOT edit types/api.ts) ────────────────────────────────────

export interface PlacementHeadroom {
  cpu_free_pct: number
  mem_free_bytes: number
  mem_total_bytes: number
  disk_free_bytes: number
}

export interface PlacementRecommendation {
  node_id: string
  node_name: string
  score: number
  reasons: string[]
  headroom: PlacementHeadroom
}

export interface PlacementRecommendResponse {
  recommendations: PlacementRecommendation[]
}

// ── Query key ─────────────────────────────────────────────────────────────────

export const PLACEMENT_KEY = ['placement', 'recommend'] as const

// ── Hook ──────────────────────────────────────────────────────────────────────

/** Fetch placement-ranked Docker-capable nodes, best-first. */
export function usePlacementRecommend(enabled = true) {
  return useQuery({
    queryKey: PLACEMENT_KEY,
    queryFn: () => apiGet<PlacementRecommendResponse>('/api/placement/recommend'),
    staleTime: 30_000,
    enabled,
  })
}
