"use client"

import { useState } from "react"
import type { ColumnDef } from "@tanstack/react-table"
import { useDefects, useUpdateDefectStatus } from "@/api/hooks/use-defects"
import { useAuth } from "@/store/auth-context"
import { toDataTablePagination } from "@/lib/utils"
import type { Defect, DefectLevel } from "@/api/types/defect"
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Bug, CheckCircle2 } from "lucide-react"

const LEVEL_MAP: Record<DefectLevel, { label: string; color: string }> = {
  DEFECT_LEVEL_UNSPECIFIED: { label: "未标记", color: "" },
  DEFECT_LEVEL_FATAL: { label: "致命", color: "bg-red-100 text-red-800 border-red-200" },
  DEFECT_LEVEL_CRITICAL: { label: "严重", color: "bg-orange-100 text-orange-800 border-orange-200" },
  DEFECT_LEVEL_MAJOR: { label: "主要", color: "bg-yellow-100 text-yellow-800 border-yellow-200" },
  DEFECT_LEVEL_MINOR: { label: "次要", color: "bg-blue-100 text-blue-800 border-blue-200" },
}

// 级别筛选下拉值（数值与 DefectLevel 枚举一致）
const LEVEL_OPTIONS = [
  { value: "1", label: "致命" },
  { value: "2", label: "严重" },
  { value: "3", label: "主要" },
  { value: "4", label: "次要" },
]

const columns: ColumnDef<Defect>[] = [
  {
    accessorKey: "file_path",
    header: "文件",
    cell: ({ row }) => (
      <div className="flex items-center gap-2">
        <Bug className="h-4 w-4 text-muted-foreground" />
        <span className="font-mono text-sm truncate max-w-[200px]">
          {row.getValue("file_path")}
        </span>
      </div>
    ),
  },
  {
    accessorKey: "line_num",
    header: "行号",
    cell: ({ row }) => (
      <span className="font-mono text-sm">L{row.getValue<number>("line_num")}</span>
    ),
  },
  {
    accessorKey: "defect_level",
    header: "级别",
    cell: ({ row }) => {
      const level = row.getValue("defect_level") as DefectLevel
      const config = LEVEL_MAP[level] || { label: level, color: "" }
      return <Badge variant="outline" className={config.color}>{config.label}</Badge>
    },
  },
  {
    accessorKey: "content",
    header: "缺陷描述",
    cell: ({ row }) => {
      const content = row.getValue("content") as string
      return <span className="text-sm truncate max-w-[300px]">{content || "-"}</span>
    },
  },
  {
    accessorKey: "module_name",
    header: "模块",
  },
  {
    accessorKey: "is_fixed",
    header: "状态",
    cell: ({ row }) => {
      const fixed = row.getValue("is_fixed") as boolean
      return fixed
        ? <Badge variant="default" className="bg-green-600"><CheckCircle2 className="mr-1 h-3 w-3" />已修复</Badge>
        : <Badge variant="secondary">待修复</Badge>
    },
  },
  {
    accessorKey: "created_at",
    header: "发现时间",
    cell: ({ row }) => {
      const date = row.getValue("created_at") as string
      return date ? new Date(date).toLocaleDateString("zh-CN") : "-"
    },
  },
]

export default function DefectsPage() {
  const { user } = useAuth()
  const tenantId = user?.tenant_id ?? ""
  const [page, setPage] = useState(1)
  const [levelFilter, setLevelFilter] = useState("")

  const { data, isLoading } = useDefects({
    tenant_id: tenantId,
    page,
    page_size: 10,
    defect_level: levelFilter ? Number(levelFilter) : undefined,
  })
  const _updateStatus = useUpdateDefectStatus()

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">缺陷台账</h1>
        <div className="flex items-center gap-2">
          <Select value={levelFilter} onValueChange={(v) => { setLevelFilter(v ?? ""); setPage(1) }}>
            <SelectTrigger className="w-32">
              <SelectValue placeholder="全部级别" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value=" ">全部级别</SelectItem>
              {LEVEL_OPTIONS.map((l) => (
                <SelectItem key={l.value} value={l.value}>
                  {l.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>缺陷列表</CardTitle>
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
