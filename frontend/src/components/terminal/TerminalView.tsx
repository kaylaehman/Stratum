import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '../../store/auth'
import { apiPost } from '../../lib/api'

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

    // ws is created only after a successful step-up preflight, so the effect goes
    // async here. `cancelled` guards every post-await action against an unmount /
    // nodeId change that happens while the preflight is in flight.
    let cancelled = false
    let retriedOnce = false
    let ws: WebSocket | null = null
    let errored = false

    function sendResize() {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
      }
    }

    // connect opens the PTY WebSocket and wires its handlers. Called only from
    // openTerminalSocket, i.e. after the step-up preflight has passed.
    function connect() {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const url = `${protocol}//${window.location.host}/api/nodes/${nodeId}/terminal`
      const token = useAuthStore.getState().accessToken
      const sock = token ? new WebSocket(url, ['bearer', token]) : new WebSocket(url)
      sock.binaryType = 'arraybuffer'
      ws = sock
      errored = false
      let opened = false

      sock.onopen = () => {
        opened = true
        onStatusChange?.('connected')
        sendResize()
        term.focus()
      }
      sock.onmessage = (ev) => {
        if (ev.data instanceof ArrayBuffer) {
          term.write(new Uint8Array(ev.data))
        } else if (typeof ev.data === 'string') {
          term.write(ev.data)
        }
      }
      sock.onerror = () => {
        errored = true
      }
      sock.onclose = () => {
        // Closed before it ever opened: the step-up grace window may have lapsed
        // between the preflight 200 and the WS handshake. Re-run the whole flow
        // (preflight included) exactly once — a bare reconnect can't surface a
        // fresh step-up challenge, since a WS handshake carries no response body.
        if (!opened && !retriedOnce && !cancelled) {
          retriedOnce = true
          void openTerminalSocket()
          return
        }
        onStatusChange?.(errored ? 'error' : 'closed')
        term.write('\r\n\x1b[2m[disconnected]\x1b[0m\r\n')
      }
    }

    // openTerminalSocket runs the step-up preflight, then connects. The preflight
    // goes through apiFetch, whose 428 handler shows the StepUp modal and retries —
    // the only way to challenge step-up before a WebSocket that can't carry a 428.
    async function openTerminalSocket() {
      try {
        await apiPost('/api/stepup/preflight', {})
      } catch {
        // 403 (not admin), a step-up the user dismissed, or a network error. The
        // apiFetch interceptor already surfaced the modal for a 428; here we just
        // report failure and do NOT open a shell.
        if (!cancelled) onStatusChange?.('error')
        return
      }
      if (cancelled) return
      connect()
    }

    // Forward keystrokes as UTF-8 binary frames (registered once; guards ws).
    const dataDisp = term.onData((data) => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(new TextEncoder().encode(data))
      }
    })
    // fit() changes term.cols/rows -> onResize fires -> tell the PTY.
    const resizeDisp = term.onResize(() => sendResize())

    function sendText(text: string) {
      if (ws && ws.readyState === WebSocket.OPEN) {
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

    // Kick off: step-up preflight, then connect. Everything above is registered
    // synchronously so it exists by the time the awaited preflight resolves.
    void openTerminalSocket()

    return () => {
      cancelled = true
      container.removeEventListener('contextmenu', onContextMenu)
      observer.disconnect()
      dataDisp.dispose()
      resizeDisp.dispose()
      if (ws) {
        ws.onclose = null // avoid status callback / retry after unmount
        ws.close()
      }
      term.dispose()
    }
  }, [nodeId, reconnectKey, onStatusChange])

  return (
    <div style={{ position: 'relative', flex: 1, minHeight: 0 }}>
      <div ref={containerRef} style={{ position: 'absolute', inset: 0, padding: '8px' }} />
    </div>
  )
}
