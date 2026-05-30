import { useAuthStore } from '../store/auth'

type Callback = (data: unknown) => void

interface Subscription {
  topic: string
  cb: Callback
}

type OpenCallback = () => void
type LogLineHandler = (line: import('../types/api').LogLine) => void

class WebSocketManager {
  private socket: WebSocket | null = null
  private subscriptions: Subscription[] = []
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private openCallbacks: OpenCallback[] = []
  private _clientId: string | null = null
  private logLineHandlers: LogLineHandler[] = []

  /** Register a callback to fire every time the socket (re)opens. */
  onOpen(cb: OpenCallback): () => void {
    this.openCallbacks.push(cb)
    return () => {
      this.openCallbacks = this.openCallbacks.filter((c) => c !== cb)
    }
  }

  connect(token?: string): void {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${window.location.host}/api/ws`

    // A browser cannot set the Authorization header on a WS handshake, so the
    // access token rides as the second WebSocket subprotocol; the backend auth
    // middleware reads it from Sec-WebSocket-Protocol. Falls back to the auth
    // store when no token is passed (e.g. the reconnect timer).
    const authToken = token ?? useAuthStore.getState().accessToken

    try {
      this.socket = authToken
        ? new WebSocket(url, ['bearer', authToken])
        : new WebSocket(url)

      this.socket.onopen = () => {
        if (this.reconnectTimer) {
          clearTimeout(this.reconnectTimer)
          this.reconnectTimer = null
        }
        // Reset client_id; server will send a fresh hello
        this._clientId = null
        for (const cb of this.openCallbacks) {
          cb()
        }
      }

      this.socket.onmessage = (event: MessageEvent) => {
        const raw = event.data as string
        this.dispatch(raw)
      }

      this.socket.onclose = () => {
        this.socket = null
        this.scheduleReconnect()
      }

      this.socket.onerror = () => {
        this.socket?.close()
      }
    } catch {
      this.scheduleReconnect()
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer) return
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.connect()
    }, 5000)
  }

  /** Returns the client_id received from the server hello, or null if not yet received. */
  clientId(): string | null {
    return this._clientId
  }

  /**
   * Register a handler for log-line messages (messages with stream + text fields).
   * Returns an unsubscribe function.
   */
  onLogLine(handler: LogLineHandler): () => void {
    this.logLineHandlers.push(handler)
    return () => {
      this.logLineHandlers = this.logLineHandlers.filter((h) => h !== handler)
    }
  }

  private dispatch(raw: string): void {
    let parsed: unknown
    try {
      parsed = JSON.parse(raw)
    } catch {
      parsed = raw
    }

    if (typeof parsed === 'object' && parsed !== null) {
      const obj = parsed as Record<string, unknown>

      // Hello message: capture client_id
      if ('client_id' in obj && typeof obj['client_id'] === 'string') {
        this._clientId = obj['client_id'] as string
        return
      }

      // Log line message: has stream + text fields
      if ('stream' in obj && 'text' in obj) {
        const line = parsed as import('../types/api').LogLine
        for (const handler of this.logLineHandlers) {
          handler(line)
        }
        return
      }
    }

    for (const sub of this.subscriptions) {
      if (typeof parsed === 'object' && parsed !== null) {
        const obj = parsed as Record<string, unknown>
        // Explicit topic field (generic messages)
        if ('topic' in obj && obj['topic'] === sub.topic) {
          sub.cb(parsed)
          continue
        }
        // CycleMessage: topic is "tree:<node_id>", matched by node_id field
        if ('node_id' in obj && sub.topic === `tree:${obj['node_id']}`) {
          sub.cb(parsed)
          continue
        }
      }
      // Raw string subscriptions (e.g. 'pong')
      if (typeof parsed === 'string' && sub.topic === parsed) {
        sub.cb(parsed)
      }
    }
  }

  subscribe(topic: string, cb: Callback): () => void {
    const sub: Subscription = { topic, cb }
    this.subscriptions.push(sub)
    return () => {
      this.subscriptions = this.subscriptions.filter((s) => s !== sub)
    }
  }

  send(data: string): void {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(data)
    }
  }

  close(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    this.socket?.close()
    this.socket = null
  }
}

export const wsManager = new WebSocketManager()
