"use client"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export default function ReportsPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">质量报表</h1>
      <Card>
        <CardHeader><CardTitle>报表列表</CardTitle></CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">质量报表页面 — 开发中</p>
        </CardContent>
      </Card>
    </div>
  )
}
