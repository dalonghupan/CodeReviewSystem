"use client"

import { useState } from "react"
import type { ColumnDef } from "@tanstack/react-table"
import { useRepos, useBindRepo, useDeleteRepo } from "@/api/hooks/use-repos"
import type { Repo } from "@/api/types/repo"
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

const columns: ColumnDef<Repo>[] = [
  {
    accessorKey: "name",
    header: "仓库名称",
    cell: ({ row }) => (
      <div className="flex items-center gap-2 font-medium">
        <GitBranch className="h-4 w-4 text-muted-foreground" />
        {row.getValue("name")}
      </div>
    ),
  },
  {
    accessorKey: "platform",
    header: "平台",
    cell: ({ row }) => {
      const platform = row.getValue("platform") as string
      const colors: Record<string, string> = {
        gitlab: "bg-orange-100 text-orange-800",
        gitee: "bg-blue-100 text-blue-800",
        github: "bg-gray-100 text-gray-800",
      }
      return (
        <Badge variant="outline" className={colors[platform] || ""}>
          {platform}
        </Badge>
      )
    },
  },
  {
    accessorKey: "default_branch",
    header: "默认分支",
  },
  {
    accessorKey: "is_active",
    header: "状态",
    cell: ({ row }) => {
      const active = row.getValue("is_active") as boolean
      return (
        <Badge variant={active ? "default" : "secondary"}>
          {active ? "活跃" : "已停用"}
        </Badge>
      )
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
  const [page, setPage] = useState(1)
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({ name: "", platform: "", repo_url: "", auth_id: "" })

  const { data, isLoading } = useRepos({ page, page_size: 10 })
  const bindRepo = useBindRepo()
  const _deleteRepo = useDeleteRepo()

  const handleBind = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await bindRepo.mutateAsync({
        tenant_id: "default",
        ...form,
      })
      setOpen(false)
      setForm({ name: "", platform: "", repo_url: "", auth_id: "" })
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
                <Label>仓库名称</Label>
                <Input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder="例如：my-project"
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
                    <SelectItem value="gitlab">GitLab</SelectItem>
                    <SelectItem value="gitee">Gitee</SelectItem>
                    <SelectItem value="github">GitHub</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>仓库 URL</Label>
                <Input
                  value={form.repo_url}
                  onChange={(e) => setForm({ ...form, repo_url: e.target.value })}
                  placeholder="https://gitlab.com/org/repo"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label>授权 ID</Label>
                <Input
                  value={form.auth_id}
                  onChange={(e) => setForm({ ...form, auth_id: e.target.value })}
                  placeholder="授权记录ID"
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
