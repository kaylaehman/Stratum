type Callback = (data: unknown) => void

interface Subscription {
  topic: string
  cb: Callback
}

class WebSocketManager {
  private socket: WebSocket | null = null
  private subscriptions: Subscription[] = []
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null

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
      // Simple topic match: if parsed is an object with a 'topic' field
      if (
        typeof parsed === 'object' &&
        parsed !== null &&
        'topic' in parsed &&
        (parsed as Record<string, unknown>)['topic'] === sub.topic
      ) {
        sub.cb(parsed)
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
