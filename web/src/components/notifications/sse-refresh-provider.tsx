"use client"

import { type ReactNode } from "react"
import { useSSERefresh } from "@/api/hooks/use-sse-refresh"

/**
 * SSE 刷新 Provider
 *
 * 在页面内启用 SSE 驱动的自动数据刷新。
 * 当收到评审状态变更、新评论等事件时，自动刷新相关页面的列表数据。
 */
export function SSERefreshProvider({ children }: { children: ReactNode }) {
  useSSERefresh()

  return <>{children}</>
}
