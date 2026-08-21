// 缺陷相关类型定义（对齐 proto/crsystem/v1/quality.proto 的 DefectInfo，snake_case）

/** protojson 枚举字符串 */
export type DefectLevel =
  | "DEFECT_LEVEL_UNSPECIFIED"
  | "DEFECT_LEVEL_FATAL"
  | "DEFECT_LEVEL_CRITICAL"
  | "DEFECT_LEVEL_MAJOR"
  | "DEFECT_LEVEL_MINOR"

export interface Defect {
  defect_id: string
  review_id: string
  comment_id: string
  review_title: string
  file_path: string
  line_num: number
  defect_level: DefectLevel
  content: string
  module_name: string
  creator_uid: string
  creator_name: string
  is_fixed: boolean
  fixed_by_uid?: string
  fix_time?: string
  stat_month: string
  created_at: string
}

export interface ListDefectsReq {
  tenant_id: string
  review_id?: string
  /** 缺陷等级筛选（枚举数值 1~4） */
  defect_level?: number
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
