"use client"

import { useState, useRef, useEffect } from "react"
import { Bell, CheckCheck, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { useUnreadCount, useNotifications, useMarkAsRead, useBatchMarkAsRead, useSSEStatus } from "@/api/hooks/use-notifications"
import type { NotificationInfo, SSEEventType } from "@/api/types/notification"
import { cn } from "@/lib/utils"

// ==================== 事件类型映射 ====================

const EVENT_TYPE_MAP: Record<SSEEventType, { label: string; color: "default" | "secondary" | "outline" | "destructive" }> = {
  review_assigned: { label: "指派评审", color: "default" },
  review_rejected: { label: "评审驳回", color: "destructive" },
  review_resubmitted: { label: "复审提交", color: "secondary" },
  review_completed: { label: "评审通过", color: "default" },
  review_timeout: { label: "评审超时", color: "destructive" },
  new_comment: { label: "新评论", color: "secondary" },
  connection_established: { label: "连接已建立", color: "outline" },
}

// ==================== 通知项组件 ====================

function NotificationItem({
  notification,
  onMarkRead,
}: {
  notification: NotificationInfo
  onMarkRead: (id: string) => void
}) {
  const config = EVENT_TYPE_MAP[notification.event_type] || {
    label: notification.event_type,
    color: "outline" as const,
  }

  return (
    <div
      className={cn(
        "flex flex-col gap-1 rounded-lg border p-3 text-sm transition-colors hover:bg-accent",
        !notification.is_read && "border-l-2 border-l-primary bg-primary/5",
      )}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Badge variant={config.color} className="text-[10px] px-1.5 py-0">
            {config.label}
          </Badge>
          {!notification.is_read && (
            <span className="h-2 w-2 rounded-full bg-primary" />
          )}
        </div>
        <span className="text-[10px] text-muted-foreground">
          {new Date(notification.created_at).toLocaleString("zh-CN")}
        </span>
      </div>
      <p className="font-medium text-foreground">{notification.title}</p>
      <p className="line-clamp-2 text-muted-foreground">{notification.content}</p>
      {!notification.is_read && (
        <Button
          variant="ghost"
          size="sm"
          className="mt-1 h-6 w-fit px-2 text-xs text-muted-foreground"
          onClick={() => onMarkRead(notification.notification_id)}
        >
          标记已读
        </Button>
      )}
    </div>
  )
}

// ==================== 通知铃铛组件 ====================

export function NotificationBell() {
  const [open, setOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const { data: unreadData, isLoading: countLoading } = useUnreadCount()
  const { data: notifData, isLoading: notifLoading } = useNotifications({ page_size: 10 })
  const markAsRead = useMarkAsRead()
  const batchMarkAsRead = useBatchMarkAsRead()
  const sseStatus = useSSEStatus()

  const unreadTotal = unreadData?.total ?? 0
  const notifications = notifData?.items ?? []

  // 点击外部关闭
  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    if (open) document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [open])

  // SSE 新消息时展开通知列表
  useEffect(() => {
    if (unreadTotal > 0 && !open) {
      // 不自动展开，只更新 Badge 数量
    }
  }, [unreadTotal, open])

  const handleMarkAllRead = () => {
    batchMarkAsRead.mutate([])
  }

  const handleMarkRead = (id: string) => {
    markAsRead.mutate(id)
  }

  return (
    <div ref={dropdownRef} className="relative">
      <Button
        variant="ghost"
        size="icon"
        className="relative"
        onClick={() => setOpen(!open)}
        aria-label="通知"
      >
        {sseStatus === "reconnecting" || sseStatus === "connecting" ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : (
          <Bell className="h-5 w-5" />
        )}
        {unreadTotal > 0 && (
          <Badge
            variant="destructive"
            className="absolute -right-1 -top-1 flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px]"
          >
            {unreadTotal > 99 ? "99+" : unreadTotal}
          </Badge>
        )}
      </Button>

      {open && (
        <div className="absolute right-0 top-full z-50 mt-2 w-96 rounded-lg border bg-popover shadow-lg">
          {/* 头部 */}
          <div className="flex items-center justify-between border-b px-4 py-3">
            <div className="flex items-center gap-2">
              <h3 className="font-semibold">通知</h3>
              {unreadTotal > 0 && (
                <span className="text-xs text-muted-foreground">
                  {unreadTotal} 条未读
                </span>
              )}
            </div>
            {unreadTotal > 0 && (
              <Button
                variant="ghost"
                size="sm"
                className="h-7 gap-1 text-xs text-muted-foreground"
                onClick={handleMarkAllRead}
                disabled={batchMarkAsRead.isPending}
              >
                <CheckCheck className="h-3.5 w-3.5" />
                全部已读
              </Button>
            )}
          </div>

          {/* 通知列表 */}
          <ScrollArea className="max-h-96">
            {notifLoading || countLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
              </div>
            ) : notifications.length === 0 ? (
              <div className="flex flex-col items-center gap-2 py-8 text-center text-sm text-muted-foreground">
                <Bell className="h-8 w-8 opacity-30" />
                <p>暂无通知</p>
              </div>
            ) : (
              <div className="flex flex-col gap-2 p-3">
                {notifications.map((n) => (
                  <NotificationItem
                    key={n.notification_id}
                    notification={n}
                    onMarkRead={handleMarkRead}
                  />
                ))}
              </div>
            )}
          </ScrollArea>
        </div>
      )}
    </div>
  )
}
