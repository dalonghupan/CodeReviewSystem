"use client"

import { forwardRef, type HTMLAttributes } from "react"
import { cn } from "@/lib/utils"

/**
 * 简单的 ScrollArea 组件
 * 为通知列表等场景提供可滚动容器
 */
const ScrollArea = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, children, ...props }, ref) => {
    return (
      <div
        ref={ref}
        className={cn("overflow-auto", className)}
        {...props}
      >
        {children}
      </div>
    )
  },
)
ScrollArea.displayName = "ScrollArea"

export { ScrollArea }
