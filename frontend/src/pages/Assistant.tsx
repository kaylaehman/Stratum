import { Bot, Settings } from 'lucide-react'
import { Link } from 'react-router-dom'
import { AppShell } from '../components/layout/AppShell'
import { AIConversation } from '../components/ai/AIConversation'
import { useAIConfig } from '../lib/api/ai'
import type { AIConfig } from '../types/api'

// Friendly "provider · model" label for the active AI provider.
function providerLabel(cfg: AIConfig): string {
  switch (cfg.provider) {
    case 'ollama':
      return `Ollama · ${cfg.ollama_model || 'local model'}`
    case 'claude':
      return `Claude API · ${cfg.claude_model}`
    case 'claude-oauth':
      return `Claude (signed in) · ${cfg.claude_model}`
    case 'openai':
      return `OpenAI · ${cfg.openai_model}`
    case 'gemini':
      return `Gemini · ${cfg.gemini_model}`
    default:
      return 'No provider configured'
  }
}

export default function Assistant() {
  const { data: cfg } = useAIConfig()

  return (
    <AppShell>
      <div className="flex flex-col flex-1 min-h-0 h-full w-full">
        {/* Page header */}
        <div className="flex items-center justify-between gap-4 mb-3 shrink-0 flex-wrap">
          <div className="flex items-center gap-2">
            <Bot size={16} style={{ color: 'var(--accent)' }} />
            <h1 className="text-sm font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
              AI Assistant
            </h1>
          </div>
          <div className="flex items-center gap-3">
            {cfg && (
              <span
                className="font-mono text-xs px-2 py-0.5"
                style={{
                  color: cfg.configured ? 'var(--text-secondary)' : 'var(--status-warn)',
                  background: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                }}
                title="Active AI provider"
              >
                {providerLabel(cfg)}
              </span>
            )}
            <Link
              to="/settings"
              title="AI settings"
              className="flex items-center gap-1 text-xs"
              style={{ color: 'var(--text-muted)', textDecoration: 'none' }}
            >
              <Settings size={13} />
              Settings
            </Link>
          </div>
        </div>

        {/* Chat surface fills the remaining height; messages scroll, input pinned. */}
        <div
          className="flex flex-col flex-1 min-h-0"
          style={{
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            overflow: 'hidden',
          }}
        >
          <AIConversation variant="page" autoFocus />
        </div>
      </div>
    </AppShell>
  )
}
