// 评审单相关类型定义（对齐 proto/crsystem/v1/review.proto，snake_case）

/** protojson 枚举字符串（LLD §3.3 状态机） */
export type ReviewStatus =
  | "REVIEW_STATUS_UNSPECIFIED"
  | "REVIEW_STATUS_PENDING"      // 待评审
  | "REVIEW_STATUS_IN_PROGRESS"  // 评审中
  | "REVIEW_STATUS_REJECTED"     // 驳回待修改
  | "REVIEW_STATUS_RESUBMITTED"  // 复审提交
  | "REVIEW_STATUS_APPROVED"     // 复审通过
  | "REVIEW_STATUS_ARCHIVED"     // 归档

export type ReviewPriority =
  | "REVIEW_PRIORITY_UNSPECIFIED"
  | "REVIEW_PRIORITY_LOW"
  | "REVIEW_PRIORITY_NORMAL"
  | "REVIEW_PRIORITY_HIGH"
  | "REVIEW_PRIORITY_URGENT"

/** 列表项（ListReviews 返回 ReviewSummary） */
export interface Review {
  review_id: string
  tenant_id: string
  repo_id: string
  mr_id: string
  title: string
  creator_uid: string
  creator_name: string
  creator_avatar: string
  reviewer_names: string[]
  status: ReviewStatus
  priority: ReviewPriority
  comment_count: number
  defect_count: number
  deadline?: string
  created_at: string
  updated_at: string
}

export interface ListReviewsReq {
  tenant_id: string
  repo_id?: string
  /** 状态筛选（枚举数值 1~6） */
  status?: number
  creator_uid?: string
  reviewer_uid?: string
  keyword?: string
  page?: number
  page_size?: number
}

export interface ListReviewsResp {
  items: Review[]
  pagination: {
    total: number
    page: number
    page_size: number
    total_pages: number
  }
}
