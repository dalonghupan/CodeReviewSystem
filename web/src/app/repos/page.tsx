"use client"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export default function ReposPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">仓库管理</h1>
      <Card>
        <CardHeader><CardTitle>绑定仓库</CardTitle></CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">仓库管理页面 — 开发中</p>
        </CardContent>
      </Card>
    </div>
  )
}
