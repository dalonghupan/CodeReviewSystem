import api from "@/api/client"
import type { Defect, ListDefectsReq, ListDefectsResp } from "@/api/types/defect"

/** 获取缺陷列表 */
export async function listDefects(params?: ListDefectsReq): Promise<ListDefectsResp> {
  const res = await api.get<ListDefectsResp>("/api/v1/defects", { params })
  return res.data
}

/** 获取缺陷详情 */
export async function getDefect(defectId: string): Promise<Defect> {
  const res = await api.get<Defect>(`/api/v1/defects/${defectId}`)
  return res.data
}

/** 更新缺陷修复状态 */
export async function updateDefectStatus(defectId: string, isFixed: boolean, operatorUid: string): Promise<void> {
  await api.put(`/api/v1/defects/${defectId}/status`, { is_fixed: isFixed, operator_uid: operatorUid })
}
