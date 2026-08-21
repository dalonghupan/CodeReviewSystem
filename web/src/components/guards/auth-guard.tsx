"use client"

import { useEffect, type ReactNode } from "react"
import { useRouter } from "next/navigation"
import { useAuth } from "@/store/auth-context"

interface AuthGuardProps {
  children: ReactNode
  /** 允许匿名访问的路由白名单 */
  publicRoutes?: string[]
}

/** 权限路由守卫：未登录重定向到 /login */
export function AuthGuard({ children, publicRoutes = [] }: AuthGuardProps) {
  const { isAuthenticated, isLoading } = useAuth()
  const router = useRouter()
  const pathname =
    typeof window !== "undefined" ? window.location.pathname : ""

  useEffect(() => {
    if (isLoading) return
    if (publicRoutes.includes(pathname)) return
    if (!isAuthenticated) {
      router.replace("/login")
    }
  }, [isAuthenticated, isLoading, pathname, publicRoutes, router])

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="text-muted-foreground">加载中...</div>
      </div>
    )
  }

  if (publicRoutes.includes(pathname)) {
    return <>{children}</>
  }

  if (!isAuthenticated) {
    return null
  }

  return <>{children}</>
}
