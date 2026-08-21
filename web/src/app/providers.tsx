"use client"

import { type ReactNode } from "react"
import { AuthProvider } from "@/store/auth-context"
import { QueryProvider } from "@/store/query-provider"
import { SSEProvider } from "@/store/sse-provider"

export function Providers({ children }: { children: ReactNode }) {
  return (
    <QueryProvider>
      <AuthProvider>
        <SSEProvider>{children}</SSEProvider>
      </AuthProvider>
    </QueryProvider>
  )
}
