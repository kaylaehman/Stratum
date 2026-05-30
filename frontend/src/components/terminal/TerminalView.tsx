import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '../../store/auth'

export type TerminalStatus = 'connecting' | 'connected' | 'closed' | 'error'

// Read a theme CSS variable off :root, falling back when unset.
function cssVar(name: string, fallback: string): string {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return v || fallback
}

interface TerminalViewProps {
  nodeId: string
  /** Bumping this value forces a fresh connection (reconnect button). */
  reconnectKey: number
  onStatusChange?: (s: TerminalStatus) => void
}

/**
 * An xterm.js terminal bridged to the node's SSH PTY over a WebSocket.
 * Keystrokes are sent as binary frames; resize events as JSON text frames;
 * PTY output arrives as binary frames. One connection per (nodeId, reconnectKey).
 */
export function TerminalView({ nodeId, reconnectKey, onStatusChange }: TerminalViewProps) {
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    onStatusChange?.('connecting')

    const term = new Terminal({
      cursorBlink: true,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: 13,
      theme: {
        background: cssVar('--bg-base', '#0b0c0f'),
        foreground: cssVar('--text-primary', '#e6e6e6'),
        cursor: cssVar('--accent', '#6ea8fe'),
      },
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(container)
    try {
      fit.fit()
    } catch {
      // container not laid out yet — the ResizeObserver below will fit shortly.
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${window.location.host}/api/nodes/${nodeId}/terminal`
    const token = useAuthStore.getState().accessToken
    const ws = token ? new WebSocket(url, ['bearer', token]) : new WebSocket(url)
    ws.binaryType = 'arraybuffer'

    let errored = false

    function sendResize() {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
      }
    }

    ws.onopen = () => {
      onStatusChange?.('connected')
      sendResize()
      term.focus()
    }
    ws.onmessage = (ev) => {
      if (ev.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(ev.data))
      } else if (typeof ev.data === 'string') {
        term.write(ev.data)
      }
    }
    ws.onerror = () => {
      errored = true
      onStatusChange?.('error')
    }
    ws.onclose = () => {
      onStatusChange?.(errored ? 'error' : 'closed')
      term.write('\r\n\x1b[2m[disconnected]\x1b[0m\r\n')
    }

    // Forward keystrokes as UTF-8 binary frames.
    const dataDisp = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(new TextEncoder().encode(data))
      }
    })
    // fit() changes term.cols/rows -> onResize fires -> tell the PTY.
    const resizeDisp = term.onResize(() => sendResize())

    function sendText(text: string) {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(new TextEncoder().encode(text))
      }
    }

    // Clipboard copy/paste via custom key handler.
    // Returning false swallows the event (it does not reach the shell).
    term.attachCustomKeyEventHandler((e) => {
      if (e.type !== 'keydown') return true

      // Copy: Ctrl+Shift+C or Ctrl+Insert — only when there is a selection.
      const isCopy =
        (e.ctrlKey && e.shiftKey && e.key === 'C') ||
        (e.ctrlKey && e.key === 'Insert')
      if (isCopy) {
        const sel = term.getSelection()
        if (sel) {
          navigator.clipboard.writeText(sel).catch(() => {})
          return false
        }
        return true // no selection — let it propagate
      }

      // Paste: Ctrl+Shift+V or Shift+Insert.
      const isPaste =
        (e.ctrlKey && e.shiftKey && e.key === 'V') ||
        (e.shiftKey && e.key === 'Insert')
      if (isPaste) {
        navigator.clipboard.readText().then(sendText).catch(() => {})
        return false
      }

      return true
    })

    // Right-click to paste.
    function onContextMenu(ev: MouseEvent) {
      ev.preventDefault()
      navigator.clipboard.readText().then(sendText).catch(() => {})
    }
    container.addEventListener('contextmenu', onContextMenu)

    const observer = new ResizeObserver(() => {
      try {
        fit.fit()
      } catch {
        // ignore transient 0-size layout during unmount
      }
    })
    observer.observe(container)

    return () => {
      container.removeEventListener('contextmenu', onContextMenu)
      observer.disconnect()
      dataDisp.dispose()
      resizeDisp.dispose()
      ws.onclose = null // avoid status callback after unmount
      ws.close()
      term.dispose()
    }
  }, [nodeId, reconnectKey, onStatusChange])

  return (
    <div style={{ position: 'relative', flex: 1, minHeight: 0 }}>
      <div ref={containerRef} style={{ position: 'absolute', inset: 0, padding: '8px' }} />
    </div>
  )
}
