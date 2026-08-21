"use client"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export default function SettingsPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">个人设置</h1>
      <Card>
        <CardHeader><CardTitle>通知偏好</CardTitle></CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">个人设置页面 — 开发中</p>
        </CardContent>
      </Card>
    </div>
  )
}
