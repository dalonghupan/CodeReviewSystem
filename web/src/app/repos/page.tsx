"use client"

import { useState } from "react"
import type { ColumnDef } from "@tanstack/react-table"
import { useRepos, useBindRepo, useDeleteRepo } from "@/api/hooks/use-repos"
import type { Repo } from "@/api/types/repo"
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Plus, Trash2, GitBranch } from "lucide-react"
import { toDataTablePagination } from "@/lib/utils"

// 后端 protojson 枚举值 → 展示名
const PLATFORM_LABELS: Record<string, string> = {
  GIT_PLATFORM_GITLAB: "GitLab",
  GIT_PLATFORM_GITEE: "Gitee",
  GIT_PLATFORM_GITHUB: "GitHub",
}

const PLATFORM_COLORS: Record<string, string> = {
  GIT_PLATFORM_GITLAB: "bg-orange-100 text-orange-800",
  GIT_PLATFORM_GITEE: "bg-blue-100 text-blue-800",
  GIT_PLATFORM_GITHUB: "bg-gray-100 text-gray-800",
}

// 平台下拉值（数值与 GitPlatform 枚举一致）
const PLATFORM_OPTIONS = [
  { value: "1", label: "GitLab" },
  { value: "2", label: "Gitee" },
  { value: "3", label: "GitHub" },
]

const columns: ColumnDef<Repo>[] = [
  {
    accessorKey: "full_name",
    header: "仓库名称",
    cell: ({ row }) => (
      <div className="flex items-center gap-2 font-medium">
        <GitBranch className="h-4 w-4 text-muted-foreground" />
        {row.getValue("full_name")}
      </div>
    ),
  },
  {
    accessorKey: "platform",
    header: "平台",
    cell: ({ row }) => {
      const platform = row.getValue("platform") as string
      return (
        <Badge variant="outline" className={PLATFORM_COLORS[platform] || ""}>
          {PLATFORM_LABELS[platform] || platform}
        </Badge>
      )
    },
  },
  {
    accessorKey: "default_branch",
    header: "默认分支",
  },
  {
    accessorKey: "sync_status",
    header: "同步状态",
    cell: ({ row }) => {
      const status = row.getValue("sync_status") as string
      return <Badge variant="secondary">{status || "-"}</Badge>
    },
  },
  {
    accessorKey: "created_at",
    header: "创建时间",
    cell: ({ row }) => {
      const date = row.getValue("created_at") as string
      return date ? new Date(date).toLocaleDateString("zh-CN") : "-"
    },
  },
  {
    id: "actions",
    header: "操作",
    cell: ({ row }) => {
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const repo = row.original
      return (
        <Button variant="ghost" size="sm" className="text-destructive">
          <Trash2 className="h-4 w-4" />
        </Button>
      )
    },
  },
]

export default function ReposPage() {
  const { user } = useAuth()
  const tenantId = user?.tenant_id ?? ""
  const [page, setPage] = useState(1)
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({ full_name: "", platform: "", platform_repo_id: "", auth_id: "" })

  const { data, isLoading } = useRepos({ tenant_id: tenantId, page, page_size: 10 })
  const bindRepo = useBindRepo()
  const _deleteRepo = useDeleteRepo()

  const handleBind = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await bindRepo.mutateAsync({
        tenant_id: tenantId,
        full_name: form.full_name,
        platform: Number(form.platform),
        platform_repo_id: form.platform_repo_id,
        auth_id: form.auth_id,
      })
      setOpen(false)
      setForm({ full_name: "", platform: "", platform_repo_id: "", auth_id: "" })
    } catch {
      // error handled by interceptor
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">仓库管理</h1>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger className="inline-flex items-center justify-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
            <Plus className="h-4 w-4" />
            绑定仓库
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>绑定 Git 仓库</DialogTitle>
            </DialogHeader>
            <form onSubmit={handleBind} className="space-y-4">
              <div className="space-y-2">
                <Label>仓库全名</Label>
                <Input
                  value={form.full_name}
                  onChange={(e) => setForm({ ...form, full_name: e.target.value })}
                  placeholder="例如：org/my-project"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label>平台</Label>
                <Select
                  value={form.platform}
                  onValueChange={(v) => setForm({ ...form, platform: v ?? "" })}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="选择平台" />
                  </SelectTrigger>
                  <SelectContent>
                    {PLATFORM_OPTIONS.map((p) => (
                      <SelectItem key={p.value} value={p.value}>
                        {p.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>平台仓库 ID</Label>
                <Input
                  value={form.platform_repo_id}
                  onChange={(e) => setForm({ ...form, platform_repo_id: e.target.value })}
                  placeholder="GitLab/Gitee/GitHub 平台侧的仓库ID"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label>授权 ID</Label>
                <Input
                  value={form.auth_id}
                  onChange={(e) => setForm({ ...form, auth_id: e.target.value })}
                  placeholder="授权记录ID"
                  required
                />
              </div>
              <Button type="submit" className="w-full" disabled={bindRepo.isPending}>
                {bindRepo.isPending ? "绑定中..." : "确认绑定"}
              </Button>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>已绑定仓库</CardTitle>
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
