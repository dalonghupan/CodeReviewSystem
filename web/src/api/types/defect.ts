// 缺陷相关类型定义

export type DefectLevel = "fatal" | "critical" | "major" | "minor"

export interface Defect {
  defect_id: string
  review_id: string
  comment_id: string
  file_path: string
  line_num: number
  defect_level: DefectLevel
  content: string
  module_name: string
  creator_uid: string
  is_fixed: boolean
  fixed_by_uid?: string
  fix_time?: string
  stat_month: string
  created_at: string
}

export interface ListDefectsReq {
  tenant_id?: string
  review_id?: string
  defect_level?: DefectLevel
  module_name?: string
  is_fixed?: boolean
  stat_month?: string
  page?: number
  page_size?: number
}

export interface ListDefectsResp {
  items: Defect[]
  pagination: {
    total: number
    page: number
    page_size: number
    total_pages: number
  }
}
