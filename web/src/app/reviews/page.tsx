"use client"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export default function ReviewsPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">评审管理</h1>
      <Card>
        <CardHeader><CardTitle>创建评审</CardTitle></CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">评审管理页面 — 开发中</p>
        </CardContent>
      </Card>
    </div>
  )
}
