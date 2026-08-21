import { AppSidebar } from "@/components/layout/app-sidebar"
import { NotificationBell } from "@/components/notifications/notification-bell"
import { Separator } from "@/components/ui/separator"
import { SSERefreshProvider } from "@/components/notifications/sse-refresh-provider"

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <div className="flex h-screen overflow-hidden">
      <AppSidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* 顶部栏 */}
        <header className="flex h-14 items-center justify-end gap-4 border-b bg-background px-6">
          <NotificationBell />
        </header>
        <Separator />
        {/* 主内容区 */}
        <main className="flex-1 overflow-auto p-6">
          <SSERefreshProvider>{children}</SSERefreshProvider>
        </main>
      </div>
    </div>
  )
}
