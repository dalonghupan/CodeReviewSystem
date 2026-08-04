"use client"

import { useAuth } from "@/store/auth-context"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  FileText,
  CheckCircle2,
  AlertTriangle,
  Clock,
} from "lucide-react"

const STATS = [
  { label: "进行中评审", value: "—", icon: FileText, color: "text-blue-600" },
  { label: "已完成评审", value: "—", icon: CheckCircle2, color: "text-green-600" },
  { label: "待处理缺陷", value: "—", icon: AlertTriangle, color: "text-yellow-600" },
  { label: "评审耗时(平均)", value: "—", icon: Clock, color: "text-purple-600" },
]

export default function DashboardPage() {
  const { user } = useAuth()

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">数据大盘</h1>
        <p className="text-muted-foreground">
          欢迎回来，{user?.username || "用户"}
        </p>
      </div>

      {/* 统计卡片 */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {STATS.map((stat) => {
          const Icon = stat.icon
          return (
            <Card key={stat.label}>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium">
                  {stat.label}
                </CardTitle>
                <Icon className={`h-4 w-4 ${stat.color}`} />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{stat.value}</div>
              </CardContent>
            </Card>
          )
        })}
      </div>

      {/* 占位：后续接入真实数据 */}
      <Card>
        <CardHeader>
          <CardTitle>近期动态</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            暂无数据，等待后端 API 对接后展示
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
