import { useState } from 'react'
import { Copy, Check, AlertTriangle } from 'lucide-react'
import type { SuggestedFix } from '../../types/api'

interface CopyButtonProps {
  text: string
}

function CopyButton({ text }: CopyButtonProps) {
  const [copied, setCopied] = useState(false)

  function handleCopy() {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1800)
    })
  }

  return (
    <button
      type="button"
      onClick={handleCopy}
      title="Copy to clipboard"
      className="flex items-center gap-1 px-2 py-1 text-xs"
      style={{
        background: copied ? 'rgba(34,201,122,0.12)' : 'var(--bg-overlay)',
        border: `1px solid ${copied ? 'var(--status-ok)' : 'var(--border-strong)'}`,
        color: copied ? 'var(--status-ok)' : 'var(--text-secondary)',
        borderRadius: '3px',
        cursor: 'pointer',
        flexShrink: 0,
        transition: 'color 0.15s, border-color 0.15s',
      }}
    >
      {copied ? <Check size={11} /> : <Copy size={11} />}
      <span>{copied ? 'Copied' : 'Copy'}</span>
    </button>
  )
}

interface FixItemProps {
  fix: SuggestedFix
  index: number
}

function FixItem({ fix, index }: FixItemProps) {
  return (
    <div
      className="flex flex-col gap-2 p-3"
      style={{
        backgroundColor: 'var(--bg-elevated)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
      }}
    >
      <p
        className="text-xs uppercase tracking-wider font-medium"
        style={{ color: 'var(--text-muted)' }}
      >
        Fix {index + 1}
      </p>

      {/* Command block with copy */}
      <div
        className="flex items-start gap-2"
        style={{
          backgroundColor: 'var(--bg-base)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          padding: '8px 10px',
        }}
      >
        <code
          className="font-mono text-xs flex-1 break-all whitespace-pre-wrap"
          style={{ color: 'var(--accent)' }}
        >
          {fix.command}
        </code>
        <CopyButton text={fix.command} />
      </div>

      {/* Rationale */}
      <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
        {fix.rationale}
      </p>

      {/* Warning (amber) */}
      {fix.warning && (
        <div
          className="flex items-start gap-2 px-2 py-2"
          style={{
            backgroundColor: 'rgba(240,160,32,0.08)',
            border: '1px solid rgba(240,160,32,0.3)',
            borderRadius: '3px',
          }}
        >
          <AlertTriangle
            size={12}
            style={{ color: 'var(--status-warn)', marginTop: '1px', flexShrink: 0 }}
          />
          <span className="text-xs" style={{ color: 'var(--status-warn)' }}>
            {fix.warning}
          </span>
        </div>
      )}

      {/* Context note */}
      <p
        className="text-xs"
        style={{
          color: 'var(--text-muted)',
          borderLeft: '2px solid var(--border-strong)',
          paddingLeft: '8px',
        }}
      >
        Run this command on the host, not inside the container.
      </p>
    </div>
  )
}

interface Props {
  fixes: SuggestedFix[]
  accessGranted: boolean
}

export function SuggestedFixList({ fixes, accessGranted }: Props) {
  if (fixes.length === 0) {
    return (
      <p
        className="text-xs px-3 py-2"
        style={{
          color: accessGranted ? 'var(--status-ok)' : 'var(--text-muted)',
          borderLeft: `2px solid ${accessGranted ? 'var(--status-ok)' : 'var(--border-strong)'}`,
          paddingLeft: '8px',
        }}
      >
        {accessGranted
          ? 'No fix needed — access is already granted.'
          : 'No automated fix available for this configuration. Review the steps above for details.'}
      </p>
    )
  }

  return (
    <div className="flex flex-col gap-2">
      <p
        className="text-xs font-medium uppercase tracking-wider"
        style={{ color: 'var(--text-muted)' }}
      >
        Suggested fixes
      </p>
      {fixes.map((fix, i) => (
        <FixItem key={i} fix={fix} index={i} />
      ))}
    </div>
  )
}
