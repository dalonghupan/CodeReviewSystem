"use client"

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { listDefects, getDefect, updateDefectStatus } from "@/api/services/defect"
import type { ListDefectsReq } from "@/api/types/defect"

export const defectKeys = {
  all: ["defects"] as const,
  list: (params?: ListDefectsReq) => ["defects", "list", params] as const,
  detail: (id: string) => ["defects", id] as const,
}

/** 缺陷列表查询 */
export function useDefects(params: ListDefectsReq) {
  return useQuery({
    queryKey: defectKeys.list(params),
    queryFn: () => listDefects(params),
    enabled: !!params.tenant_id,
  })
}

/** 缺陷详情查询 */
export function useDefect(defectId: string) {
  return useQuery({
    queryKey: defectKeys.detail(defectId),
    queryFn: () => getDefect(defectId),
    enabled: !!defectId,
  })
}

/** 更新缺陷状态 */
export function useUpdateDefectStatus() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ defectId, isFixed, operatorUid }: { defectId: string; isFixed: boolean; operatorUid: string }) =>
      updateDefectStatus(defectId, isFixed, operatorUid),
    onSuccess: () => qc.invalidateQueries({ queryKey: defectKeys.all }),
  })
}
