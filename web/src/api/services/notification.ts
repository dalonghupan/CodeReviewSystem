import api from "@/api/client"
import { getToken } from "@/lib/utils"
import type {
  SSEEvent,
  SSEEventData,
  SSEConnectionStatus,
  SSEEventType,
  ListNotificationsReq,
  ListNotificationsResp,
  UnreadCountResp,
  NotificationInfo,
} from "@/api/types/notification"

// ==================== SSE 连接管理 ====================

const SSE_URL = `${process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8000"}/api/v1/sse/events`

type SSECallback = (event: SSEEvent) => void
type StatusCallback = (status: SSEConnectionStatus) => void

class SSEManager {
  private eventSource: EventSource | null = null
  private listeners = new Map<SSEEventType | "*", Set<SSECallback>>()
  private statusListeners = new Set<StatusCallback>()
  private _status: SSEConnectionStatus = "disconnected"
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private maxRetries = 10
  private retryCount = 0
  private baseDelay = 2000
  private lastEventId: string | null = null

  get status() {
    return this._status
  }

  private setStatus(s: SSEConnectionStatus) {
    this._status = s
    this.statusListeners.forEach((cb) => cb(s))
  }

  /** 订阅指定类型的事件（"*" 表示所有事件） */
  on(eventType: SSEEventType | "*", callback: SSECallback) {
    if (!this.listeners.has(eventType)) {
      this.listeners.set(eventType, new Set())
    }
    this.listeners.get(eventType)!.add(callback)
    return () => this.listeners.get(eventType)?.delete(callback)
  }

  /** 订阅连接状态变化 */
  onStatus(callback: StatusCallback) {
    this.statusListeners.add(callback)
    return () => this.statusListeners.delete(callback)
  }

  /** 建立 SSE 连接 */
  connect() {
    if (this.eventSource && this._status === "connected") return

    this.setStatus("connecting")
    this.cleanup()

    const token = getToken()
    const url = token ? `${SSE_URL}?token=${encodeURIComponent(token)}` : SSE_URL

    this.eventSource = new EventSource(url, { withCredentials: true })

    // 如果后端发送了 lastEventId，自动记录
    this.eventSource.addEventListener("open", () => {
      this.setStatus("connected")
      this.retryCount = 0
    })

    // 通用事件处理 — 所有事件名都从后端发来
    this.eventSource.onmessage = (event) => {
      try {
        const sseEvent = this.parseEvent(event)
        if (sseEvent) {
          this.dispatch(sseEvent)
        }
      } catch {
        // 解析失败跳过
      }
    }

    // 当后端发出命名事件（event: XXX data: {...}）时
    // 我们需要通过 addEventListener 捕获每个命名事件
    // 由于事先不知道所有事件名，用一个通用 fallback 不可行
    // 这里采用 onmessage + 自定义 event.data 中包含 event type 字段
    // 兼容两种格式

    this.eventSource.onerror = () => {
      this.handleDisconnect()
    }
  }

  /** 断开 SSE 连接 */
  disconnect() {
    this.cleanup()
    this.setStatus("disconnected")
    this.retryCount = 0
    this.lastEventId = null
  }

  /** 解析 SSE 事件 */
  private parseEvent(event: MessageEvent): SSEEvent | null {
    try {
      const data = JSON.parse(event.data) as SSEEventData
      return {
        event: data.event_type,
        data: event.data,
        id: event.lastEventId || undefined,
      }
    } catch {
      // 如果 data 是纯文本（非 JSON），返回通用事件
      return {
        event: "review_assigned" as SSEEventType,
        data: event.data,
        id: event.lastEventId || undefined,
      }
    }
  }

  /** 分发事件到订阅者 */
  private dispatch(event: SSEEvent) {
    // 记录 lastEventId 用于重连
    if (event.id) this.lastEventId = event.id

    // 通知特定事件类型的订阅者
    const typeListeners = this.listeners.get(event.event)
    if (typeListeners) {
      typeListeners.forEach((cb) => {
        try {
          cb(event)
        } catch {
          // 单回调异常不阻塞其他回调
        }
      })
    }

    // 通知通配符订阅者
    const wildcardListeners = this.listeners.get("*")
    if (wildcardListeners) {
      wildcardListeners.forEach((cb) => {
        try {
          cb(event)
        } catch {
          // 单回调异常不阻塞其他回调
        }
      })
    }
  }

  /** 断线处理 + 自动重连 */
  private handleDisconnect() {
    this.cleanup()
    this.setStatus("reconnecting")

    if (this.retryCount >= this.maxRetries) {
      this.setStatus("error")
      return
    }

    this.retryCount++
    const delay = Math.min(this.baseDelay * Math.pow(1.5, this.retryCount - 1), 30000)

    this.reconnectTimer = setTimeout(() => {
      this.connect()
    }, delay)
  }

  /** 清理 EventSource 和定时器 */
  private cleanup() {
    if (this.eventSource) {
      this.eventSource.close()
      this.eventSource = null
    }
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }
}

/** 全局单例 SSE Manager */
export const sseManager = new SSEManager()

// ==================== 站内信 REST API ====================

/** 获取通知列表 */
export async function listNotifications(params?: ListNotificationsReq): Promise<ListNotificationsResp> {
  const res = await api.get<ListNotificationsResp>("/api/v1/notifications", { params })
  return res.data
}

/** 获取未读数量 */
export async function getUnreadCount(): Promise<UnreadCountResp> {
  const res = await api.get<UnreadCountResp>("/api/v1/notifications/unread-count")
  return res.data
}

/** 标记单条已读 */
export async function markAsRead(notificationId: string): Promise<void> {
  await api.post("/api/v1/notifications/read", { notification_id: notificationId })
}

/** 批量标记已读 */
export async function batchMarkAsRead(notificationIds?: string[]): Promise<void> {
  await api.post("/api/v1/notifications/batch-read", { notification_ids: notificationIds || [] })
}
