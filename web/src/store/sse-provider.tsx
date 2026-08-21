"use client"

import { useEffect, type ReactNode } from "react"
import { sseManager } from "@/api/services/notification"
import { useAuth } from "@/store/auth-context"

/**
 * SSE Provider
 *
 * 管理 SSE 连接的生命周期：
 * - 用户登录后自动建立连接
 * - 用户登出后断开连接
 * - 组件卸载时断开连接
 */
export function SSEProvider({ children }: { children: ReactNode }) {
  const { token } = useAuth()

  useEffect(() => {
    if (token) {
      // 登录后建立 SSE 连接
      sseManager.connect()
    } else {
      // 登出后断开 SSE 连接
      sseManager.disconnect()
    }

    return () => {
      // 组件卸载时断开连接（应用关闭）
      sseManager.disconnect()
    }
  }, [token])

  return <>{children}</>
}
