"use client"

import { useEffect, useCallback, useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  sseManager,
  listNotifications,
  getUnreadCount,
  markAsRead,
  batchMarkAsRead,
} from "@/api/services/notification"
import { useAuth } from "@/store/auth-context"
import type {
  SSEEvent,
  SSEEventType,
  SSEConnectionStatus,
  ListNotificationsReq,
} from "@/api/types/notification"

// ==================== Query Keys ====================

export const notificationKeys = {
  all: ["notifications"] as const,
  list: (params?: ListNotificationsReq) => ["notifications", "list", params] as const,
  unreadCount: ["notifications", "unread-count"] as const,
}

// ==================== SSE 订阅 Hook ====================

/** 订阅 SSE 事件，组件挂载时自动建立连接 */
export function useSSE(
  eventType: SSEEventType | "*" = "*",
  callback?: (event: SSEEvent) => void,
) {
  const { token } = useAuth()

  useEffect(() => {
    if (!token) return

    // 建立 SSE 连接
    sseManager.connect()

    // 注册事件回调
    const unsub = callback
      ? sseManager.on(eventType, callback)
      : undefined

    return () => {
      unsub?.()
      // 不在这里断开连接（组件卸载时保持连接，由 Provider 管理生命周期）
    }
  }, [token, eventType, callback])
}

/** 获取 SSE 连接状态 */
export function useSSEStatus() {
  const [status, setStatus] = useState<SSEConnectionStatus>(sseManager.status)
  const { token } = useAuth()

  useEffect(() => {
    if (!token) {
      setStatus("disconnected")
      return
    }
    const unsub = sseManager.onStatus(setStatus)
    return () => { unsub?.() }
  }, [token])

  return status
}

// ==================== 通知数据 Hooks ====================

/** 获取通知列表 */
export function useNotifications(params?: ListNotificationsReq) {
  const { token } = useAuth()
  return useQuery({
    queryKey: notificationKeys.list(params),
    queryFn: () => listNotifications(params),
    enabled: !!token,
  })
}

/** 获取未读通知数量（轮询 + SSE 推送更新） */
export function useUnreadCount() {
  const { token } = useAuth()
  const qc = useQueryClient()

  // 初始查询 + 30 秒轮询
  const query = useQuery({
    queryKey: notificationKeys.unreadCount,
    queryFn: () => getUnreadCount(),
    enabled: !!token,
    refetchInterval: 30_000,
  })

  // SSE 推送时刷新未读数
  useEffect(() => {
    if (!token) return
    const unsub = sseManager.on("*", () => {
      qc.invalidateQueries({ queryKey: notificationKeys.unreadCount })
      qc.invalidateQueries({ queryKey: notificationKeys.list() })
    })
    return () => { unsub?.() }
  }, [token, qc])

  return query
}

// ==================== 通知操作 Mutations ====================

/** 标记通知为已读 */
export function useMarkAsRead() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (notificationId: string) => markAsRead(notificationId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: notificationKeys.unreadCount })
      qc.invalidateQueries({ queryKey: notificationKeys.list() })
    },
  })
}

/** 批量标记已读 */
export function useBatchMarkAsRead() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (notificationIds: string[] = []) => batchMarkAsRead(notificationIds),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: notificationKeys.unreadCount })
      qc.invalidateQueries({ queryKey: notificationKeys.list() })
    },
  })
}
