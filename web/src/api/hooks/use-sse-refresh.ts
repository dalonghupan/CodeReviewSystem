"use client"

import { useEffect } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { sseManager } from "@/api/services/notification"
import { useAuth } from "@/store/auth-context"
import { reviewKeys } from "@/api/hooks/use-reviews"
import { commentKeys } from "@/api/hooks/use-comments"
import type { SSEEvent } from "@/api/types/notification"

/**
 * SSE 事件驱动的自动刷新 Hook
 *
 * 当收到特定类型的 SSE 事件时，自动 invalidate 相关页面的 TanStack Query 缓存
 * 实现实时数据刷新，无需用户手动刷新页面
 */
export function useSSERefresh() {
  const { token } = useAuth()
  const qc = useQueryClient()

  useEffect(() => {
    if (!token) return

    // 事件类型 → 需要刷新的 Query Key 映射
    const eventToKeys: Record<string, (() => void)[]> = {
      // 评审相关事件 → 刷新评审列表
      review_assigned: [() => qc.invalidateQueries({ queryKey: reviewKeys.all })],
      review_rejected: [() => qc.invalidateQueries({ queryKey: reviewKeys.all })],
      review_resubmitted: [() => qc.invalidateQueries({ queryKey: reviewKeys.all })],
      review_completed: [() => qc.invalidateQueries({ queryKey: reviewKeys.all })],

      // 新评论事件 → 刷新评论列表
      new_comment: [() => qc.invalidateQueries({ queryKey: commentKeys.all })],
    }

    const handler = (event: SSEEvent) => {
      const actions = eventToKeys[event.event]
      if (actions) {
        actions.forEach((action) => action())
      }
    }

    // 注册通配符监听
    const unsub = sseManager.on("*", handler)

    return () => {
      unsub()
    }
  }, [token, qc])
}
