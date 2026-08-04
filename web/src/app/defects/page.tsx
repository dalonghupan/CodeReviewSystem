"use client"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export default function DefectsPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">缺陷台账</h1>
      <Card>
        <CardHeader><CardTitle>缺陷列表</CardTitle></CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">缺陷台账页面 — 开发中</p>
        </CardContent>
      </Card>
    </div>
  )
}
