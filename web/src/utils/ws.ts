// WebSocket 封装：认证 + 多频道订阅 + 自动重连
import { useAuthStore } from '@/stores/auth'

export type WSChannel =
  | 'linkdetect'
  | 'monitor'
  | 'traffic'
  | 'dbmonitor'
  | 'containermon'
  | 'dashboard'

export interface WSMessage {
  channel: string
  event: string
  data: Record<string, unknown>
}

type Handler = (msg: WSMessage) => void

export class WSClient {
  private ws: WebSocket | null = null
  private channels: WSChannel[] = []
  private handlers = new Map<string, Set<Handler>>()
  private retry = 0
  private timer: number | null = null
  private closed = false

  constructor(channels: WSChannel[] = []) {
    this.channels = channels
  }

  connect(): void {
    const auth = useAuthStore()
    if (!auth.token || this.closed) return
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const ch = encodeURIComponent(this.channels.join(','))
    this.ws = new WebSocket(`${proto}://${location.host}/ws?token=${encodeURIComponent(auth.token)}&channels=${ch}`)
    this.ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data as string) as WSMessage
        this.dispatch(msg)
      } catch {
        /* 忽略非JSON消息 */
      }
    }
    this.ws.onclose = () => {
      if (this.closed) return
      const delay = Math.min(1000 * 2 ** this.retry, 15000)
      this.retry++
      this.timer = window.setTimeout(() => this.connect(), delay)
    }
    this.ws.onerror = () => {
      this.ws?.close()
    }
  }

  on(channel: WSChannel, event: string, handler: Handler): void {
    const key = `${channel}:${event}`
    if (!this.handlers.has(key)) this.handlers.set(key, new Set())
    this.handlers.get(key)!.add(handler)
  }

  off(channel: WSChannel, event: string, handler: Handler): void {
    this.handlers.get(`${channel}:${event}`)?.delete(handler)
  }

  private dispatch(msg: WSMessage): void {
    const set = this.handlers.get(`${msg.channel}:${msg.event}`)
    set?.forEach((h) => h(msg))
  }

  close(): void {
    this.closed = true
    if (this.timer) window.clearTimeout(this.timer)
    this.ws?.close()
    this.ws = null
  }
}
