"use client"

import { useState } from "react"
import type { ColumnDef } from "@tanstack/react-table"
import { useReviews, useCreateReview } from "@/api/hooks/use-reviews"
import type { Review, ReviewStatus } from "@/api/types/review"
import { useAuth } from "@/store/auth-context"
import { DataTable } from "@/components/ui/data-table"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Plus, FileText } from "lucide-react"
import { toDataTablePagination } from "@/lib/utils"

const STATUS_MAP: Record<ReviewStatus, { label: string; variant: "default" | "secondary" | "outline" | "destructive" }> = {
  REVIEW_STATUS_UNSPECIFIED: { label: "未知", variant: "outline" },
  REVIEW_STATUS_PENDING: { label: "待评审", variant: "outline" },
  REVIEW_STATUS_IN_PROGRESS: { label: "评审中", variant: "default" },
  REVIEW_STATUS_REJECTED: { label: "驳回待修改", variant: "destructive" },
  REVIEW_STATUS_RESUBMITTED: { label: "复审提交", variant: "secondary" },
  REVIEW_STATUS_APPROVED: { label: "复审通过", variant: "default" },
  REVIEW_STATUS_ARCHIVED: { label: "已归档", variant: "secondary" },
}

const columns: ColumnDef<Review>[] = [
  {
    accessorKey: "title",
    header: "评审标题",
    cell: ({ row }) => (
      <div className="flex items-center gap-2 font-medium">
        <FileText className="h-4 w-4 text-muted-foreground" />
        {row.getValue("title")}
      </div>
    ),
  },
  {
    accessorKey: "status",
    header: "状态",
    cell: ({ row }) => {
      const status = row.getValue("status") as ReviewStatus
      const config = STATUS_MAP[status] || { label: status, variant: "outline" as const }
      return <Badge variant={config.variant}>{config.label}</Badge>
    },
  },
  {
    accessorKey: "creator_name",
    header: "创建人",
  },
  {
    accessorKey: "reviewer_names",
    header: "评审人",
    cell: ({ row }) => {
      const names = row.getValue("reviewer_names") as string[]
      return names?.length ? names.join("、") : "-"
    },
  },
  {
    accessorKey: "defect_count",
    header: "缺陷数",
  },
  {
    accessorKey: "created_at",
    header: "创建时间",
    cell: ({ row }) => {
      const date = row.getValue("created_at") as string
      return date ? new Date(date).toLocaleDateString("zh-CN") : "-"
    },
  },
]

export default function ReviewsPage() {
  const { user } = useAuth()
  const tenantId = user?.tenant_id ?? ""
  const [page, setPage] = useState(1)
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({ repo_id: "", mr_id: "", title: "", reviewer_uids: "" })

  const { data, isLoading } = useReviews({ tenant_id: tenantId, page, page_size: 10 })
  const createReview = useCreateReview()

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await createReview.mutateAsync({
        tenant_id: tenantId,
        ...form,
        reviewer_uids: form.reviewer_uids.split(",").map((s) => s.trim()).filter(Boolean),
      })
      setOpen(false)
      setForm({ repo_id: "", mr_id: "", title: "", reviewer_uids: "" })
    } catch {
      // handled by interceptor
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">评审管理</h1>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger className="inline-flex items-center justify-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
            <Plus className="h-4 w-4" />
            创建评审
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>创建代码评审</DialogTitle>
            </DialogHeader>
            <form onSubmit={handleCreate} className="space-y-4">
              <div className="space-y-2">
                <Label>评审标题</Label>
                <Input
                  value={form.title}
                  onChange={(e) => setForm({ ...form, title: e.target.value })}
                  placeholder="例如：feat: 用户登录模块代码审查"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label>仓库 ID</Label>
                <Input
                  value={form.repo_id}
                  onChange={(e) => setForm({ ...form, repo_id: e.target.value })}
                  placeholder="仓库ID"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label>MR 编号</Label>
                <Input
                  value={form.mr_id}
                  onChange={(e) => setForm({ ...form, mr_id: e.target.value })}
                  placeholder="MR 编号"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label>评审人 UID（逗号分隔）</Label>
                <Input
                  value={form.reviewer_uids}
                  onChange={(e) => setForm({ ...form, reviewer_uids: e.target.value })}
                  placeholder="uid1, uid2, uid3"
                />
              </div>
              <Button type="submit" className="w-full" disabled={createReview.isPending}>
                {createReview.isPending ? "创建中..." : "创建评审"}
              </Button>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>评审列表</CardTitle>
        </CardHeader>
        <CardContent>
          <DataTable
            columns={columns}
            data={data?.items || []}
            loading={isLoading}
            pagination={toDataTablePagination(data?.pagination)}
            onPageChange={setPage}
          />
        </CardContent>
      </Card>
    </div>
  )
}
