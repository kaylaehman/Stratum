import type { ReactNode } from 'react'
import { useFeatureEnabled } from '../../lib/api/features'

interface FeatureGuardProps {
  /** The feature-flag key, e.g. 'feature.uptime_monitoring'. */
  flag: string
  /** Human-readable feature name shown in the disabled notice. */
  name: string
  children: ReactNode
}

/**
 * Renders children when the feature flag is enabled; otherwise shows a clean
 * "disabled" notice instead of the page. Used to guard the routes of Beta
 * subsystems that have a toggle in Settings → Features, so a deep-link to a
 * disabled feature degrades gracefully rather than rendering a live panel.
 *
 * Defaults to enabled while the flag list loads (see useFeatureEnabled), so the
 * page never flickers off before the first /api/features response.
 */
export function FeatureGuard({ flag, name, children }: FeatureGuardProps) {
  const enabled = useFeatureEnabled(flag)
  if (enabled) return <>{children}</>

  return (
    <div className="flex flex-1 items-center justify-center p-10">
      <div
        className="max-w-md text-center px-6 py-5"
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
        }}
      >
        <p className="text-sm font-medium mb-1" style={{ color: 'var(--text-primary)' }}>
          {name} is disabled
        </p>
        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
          An admin can enable it under <span className="font-mono">Settings → Features</span>{' '}
          (<span className="font-mono">{flag}</span>).
        </p>
      </div>
    </div>
  )
}
