import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  children: ReactNode
  /** Optional label so nested boundaries can identify which subtree failed. */
  label?: string
}

interface State {
  error: Error | null
}

/**
 * ErrorBoundary stops a render-time throw from unmounting the whole React tree
 * to a blank page. Without one, any uncaught error in a child (e.g. a component
 * dereferencing an unexpected-null API field) blanks the entire app. Here it
 * renders a recoverable fallback that also surfaces the error text, which is the
 * fastest way to diagnose the underlying throw.
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // Keep a console breadcrumb; a future iteration can POST this to the backend.
    console.error('ErrorBoundary caught', this.props.label ?? '', error, info.componentStack)
  }

  handleReset = () => this.setState({ error: null })

  render() {
    const { error } = this.state
    if (!error) return this.props.children

    return (
      <div
        role="alert"
        style={{
          minHeight: '100vh',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: '12px',
          padding: '24px',
          backgroundColor: 'var(--bg-base, #0d0f12)',
          color: 'var(--text-primary, #e6e8eb)',
          fontFamily: 'var(--font-mono, monospace)',
        }}
      >
        <div style={{ fontSize: '13px', fontWeight: 600 }}>Something broke while rendering this view.</div>
        <pre
          style={{
            maxWidth: '640px',
            maxHeight: '40vh',
            overflow: 'auto',
            fontSize: '11px',
            color: 'var(--status-error, #d6455d)',
            border: '1px solid var(--border-subtle, #2a2f36)',
            borderRadius: '4px',
            padding: '12px',
            whiteSpace: 'pre-wrap',
          }}
        >
          {error.message}
        </pre>
        <div style={{ display: 'flex', gap: '8px' }}>
          <button
            type="button"
            onClick={this.handleReset}
            style={{
              fontSize: '12px',
              padding: '6px 14px',
              cursor: 'pointer',
              backgroundColor: 'transparent',
              border: '1px solid var(--border-default, #3a4048)',
              borderRadius: '3px',
              color: 'var(--text-primary, #e6e8eb)',
            }}
          >
            Try again
          </button>
          <button
            type="button"
            onClick={() => window.location.reload()}
            style={{
              fontSize: '12px',
              padding: '6px 14px',
              cursor: 'pointer',
              backgroundColor: 'transparent',
              border: '1px solid var(--border-default, #3a4048)',
              borderRadius: '3px',
              color: 'var(--text-primary, #e6e8eb)',
            }}
          >
            Reload
          </button>
        </div>
      </div>
    )
  }
}
