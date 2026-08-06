// 通知/消息相关类型定义

// ==================== SSE 事件类型 ====================

/** SSE 事件类型枚举 */
export type SSEEventType =
  | "review_assigned"       // 被指派为评审人
  | "review_rejected"       // 评审被驳回
  | "review_resubmitted"    // 复审提交
  | "review_completed"      // 评审通过/完成
  | "review_timeout"        // 评审超时
  | "new_comment"           // 新评论
  | "connection_established" // SSE 连接建立（系统事件）

/** SSE 事件载荷 */
export interface SSEEvent {
  event: SSEEventType
  data: string              // JSON 字符串，需 parse
  id?: string               // 事件 ID（用于断线重连）
}

/** 通知事件数据（data parse 后的结构） */
export interface SSEEventData {
  notification_id: string
  user_id: string
  event_type: SSEEventType
  title: string
  content: string
  related_id: string        // 关联业务 ID（如 review_id）
  created_at: string
}

/** SSE 连接状态 */
export type SSEConnectionStatus =
  | "disconnected"  // 未连接/已断开
  | "connecting"    // 连接中
  | "connected"     // 已连接
  | "reconnecting"  // 重连中
  | "error"         // 连接出错

// ==================== 站内信类型 ====================

export interface NotificationInfo {
  notification_id: string
  user_id: string
  event_type: SSEEventType
  title: string
  content: string
  related_id: string
  is_read: boolean
  channel: "site" | "wechat" | "email"
  created_at: string
  read_at?: string
}

export interface ListNotificationsReq {
  unread_only?: boolean
  event_type?: string
  page?: number
  page_size?: number
}

export interface ListNotificationsResp {
  items: NotificationInfo[]
  pagination: {
    total: number
    page: number
    page_size: number
    total_pages: number
  }
  unread_total: number
}

export interface UnreadCountResp {
  total: number
  by_type: Record<string, number>
}
