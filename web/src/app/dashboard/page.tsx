"use client"

import { useAuth } from "@/store/auth-context"
import { useReviews } from "@/api/hooks/use-reviews"
import { useDefects } from "@/api/hooks/use-defects"
import { useRepos } from "@/api/hooks/use-repos"
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
  GitBranch,
  Clock,
  Bug,
} from "lucide-react"

export default function DashboardPage() {
  const { user } = useAuth()
  const tenantId = user?.tenant_id ?? ""
  const { data: reviewsData } = useReviews({ tenant_id: tenantId, page: 1, page_size: 1 })
  const { data: defectsData } = useDefects({ tenant_id: tenantId, page: 1, page_size: 1 })
  const { data: reposData } = useRepos({ tenant_id: tenantId, page: 1, page_size: 1 })

  const totalReviews = reviewsData?.pagination?.total ?? 0
  const totalDefects = defectsData?.pagination?.total ?? 0
  const totalRepos = reposData?.pagination?.total ?? 0

  const STATS = [
    {
      label: "绑定仓库",
      value: totalRepos.toString(),
      icon: GitBranch,
      color: "text-blue-600",
    },
    {
      label: "进行中评审",
      value: totalReviews.toString(),
      icon: FileText,
      color: "text-blue-600",
    },
    {
      label: "待处理缺陷",
      value: totalDefects.toString(),
      icon: AlertTriangle,
      color: "text-yellow-600",
    },
    {
      label: "评审耗时(平均)",
      value: "—",
      icon: Clock,
      color: "text-purple-600",
    },
  ]

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

      {/* 近期评审 */}
      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader className="flex flex-row items-center gap-2">
            <FileText className="h-5 w-5 text-muted-foreground" />
            <CardTitle className="text-base">近期评审</CardTitle>
          </CardHeader>
          <CardContent>
            {reviewsData?.items && reviewsData.items.length > 0 ? (
              <ul className="space-y-2">
                {reviewsData.items.slice(0, 5).map((review) => (
                  <li key={review.review_id} className="flex items-center justify-between text-sm">
                    <span className="truncate max-w-[200px]">{review.title}</span>
                    <span className="text-muted-foreground text-xs">
                      {new Date(review.created_at).toLocaleDateString("zh-CN")}
                    </span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground">暂无评审记录</p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center gap-2">
            <Bug className="h-5 w-5 text-muted-foreground" />
            <CardTitle className="text-base">近期缺陷</CardTitle>
          </CardHeader>
          <CardContent>
            {defectsData?.items && defectsData.items.length > 0 ? (
              <ul className="space-y-2">
                {defectsData.items.slice(0, 5).map((defect) => (
                  <li key={defect.defect_id} className="flex items-center justify-between text-sm">
                    <span className="truncate max-w-[200px] font-mono text-xs">
                      {defect.file_path}:L{defect.line_num}
                    </span>
                    <span className="text-muted-foreground text-xs">
                      {new Date(defect.created_at).toLocaleDateString("zh-CN")}
                    </span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground">暂无缺陷记录</p>
            )}
          </CardContent>
        </Card>
      </div>

      {/* 缺陷级别分布（占位） */}
      <Card>
        <CardHeader className="flex flex-row items-center gap-2">
          <CheckCircle2 className="h-5 w-5 text-muted-foreground" />
          <CardTitle className="text-base">质量概览</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            连接后端 API 后将展示完整的质量指标趋势图
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
