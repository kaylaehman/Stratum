type Callback = (data: unknown) => void

interface Subscription {
  topic: string
  cb: Callback
}

type OpenCallback = () => void

class WebSocketManager {
  private socket: WebSocket | null = null
  private subscriptions: Subscription[] = []
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private openCallbacks: OpenCallback[] = []

  /** Register a callback to fire every time the socket (re)opens. */
  onOpen(cb: OpenCallback): () => void {
    this.openCallbacks.push(cb)
    return () => {
      this.openCallbacks = this.openCallbacks.filter((c) => c !== cb)
    }
  }

  // token reserved for future use (e.g. query-param auth or subprotocol header)
  connect(token?: string): void {
    void token
    if (this.socket && this.socket.readyState === WebSocket.OPEN) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${window.location.host}/api/ws`

    try {
      this.socket = new WebSocket(url)

      this.socket.onopen = () => {
        if (this.reconnectTimer) {
          clearTimeout(this.reconnectTimer)
          this.reconnectTimer = null
        }
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

  private dispatch(raw: string): void {
    let parsed: unknown
    try {
      parsed = JSON.parse(raw)
    } catch {
      parsed = raw
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
