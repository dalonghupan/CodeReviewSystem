import api from "@/api/client"
import type { Defect, ListDefectsReq, ListDefectsResp } from "@/api/types/defect"

// 后端 HTTP 绑定见 proto/crsystem/v1/quality.proto
// 分页参数为嵌套 message Pagination，query 绑定须用点号路径 pagination.page

/** 获取缺陷列表 */
export async function listDefects(params: ListDefectsReq): Promise<ListDefectsResp> {
  const { page, page_size, ...rest } = params
  const res = await api.get<ListDefectsResp>("/api/v1/defects", {
    params: {
      ...rest,
      ...(page != null ? { "pagination.page": page } : {}),
      ...(page_size != null ? { "pagination.page_size": page_size } : {}),
    },
  })
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
