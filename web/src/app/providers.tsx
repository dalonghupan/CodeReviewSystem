"use client"

import { type ReactNode } from "react"
import { AuthProvider } from "@/store/auth-context"
import { QueryProvider } from "@/store/query-provider"

export function Providers({ children }: { children: ReactNode }) {
  return (
    <QueryProvider>
      <AuthProvider>{children}</AuthProvider>
    </QueryProvider>
  )
}
