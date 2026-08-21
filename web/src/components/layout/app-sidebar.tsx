"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { useAuth } from "@/store/auth-context"
import { Button } from "@/components/ui/button"
import {
  Code2,
  GitBranch,
  FileText,
  BarChart3,
  Bug,
  Settings,
  LogOut,
} from "lucide-react"
import { cn } from "@/lib/utils"

const NAV_ITEMS = [
  { href: "/dashboard", label: "数据大盘", icon: BarChart3 },
  { href: "/repos", label: "仓库管理", icon: GitBranch },
  { href: "/reviews", label: "评审管理", icon: FileText },
  { href: "/defects", label: "缺陷台账", icon: Bug },
  { href: "/reports", label: "质量报表", icon: BarChart3 },
  { href: "/settings", label: "个人设置", icon: Settings },
]

export function AppSidebar() {
  const pathname = usePathname()
  const { user, logout } = useAuth()

  return (
    <aside className="flex h-full w-60 flex-col border-r bg-background">
      {/* Logo */}
      <div className="flex items-center gap-2 border-b px-4 py-4">
        <Code2 className="h-6 w-6 text-primary" />
        <span className="font-semibold">CR-System</span>
      </div>

      {/* 导航 */}
      <nav className="flex-1 space-y-1 p-3">
        {NAV_ITEMS.map((item) => {
          const Icon = item.icon
          const active = pathname === item.href
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors",
                active
                  ? "bg-primary/10 text-primary font-medium"
                  : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
              )}
            >
              <Icon className="h-4 w-4" />
              {item.label}
            </Link>
          )
        })}
      </nav>

      {/* 用户信息 */}
      <div className="border-t p-3">
        <div className="mb-2 px-3 text-xs text-muted-foreground">
          {user?.username || "未登录"}
        </div>
        <Button
          variant="ghost"
          size="sm"
          className="w-full justify-start gap-2 text-muted-foreground"
          onClick={logout}
        >
          <LogOut className="h-4 w-4" />
          退出登录
        </Button>
      </div>
    </aside>
  )
}
