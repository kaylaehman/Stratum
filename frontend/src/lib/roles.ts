import { useAuthStore } from '../store/auth'
import type { UserRole } from '../types/api'

/** Numeric rank: higher = more privileged. */
export const roleRank: Record<UserRole, number> = {
  viewer: 0,
  operator: 1,
  admin: 2,
}

/**
 * Returns true when `userRole` meets or exceeds `min`.
 * Unknown roles (empty string / undefined) rank as viewer (0).
 */
export function hasRole(userRole: string | undefined | null, min: UserRole): boolean {
  const rank = roleRank[(userRole as UserRole) ?? 'viewer'] ?? 0
  return rank >= roleRank[min]
}

export interface CanResult {
  /** True for operator-or-higher (start/stop/restart containers, bulk start/stop/restart). */
  isOperator: boolean
  /** True for admin only (secrets, scripts, webhooks, user management, destructive ops). */
  isAdmin: boolean
}

/** Hook returning role-derived capability flags for the currently signed-in user. */
export function useCan(): CanResult {
  const user = useAuthStore((s) => s.user)
  const role = user?.role
  return {
    isOperator: hasRole(role, 'operator'),
    isAdmin: hasRole(role, 'admin'),
  }
}
