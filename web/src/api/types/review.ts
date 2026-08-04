// 评审单相关类型定义

export type ReviewStatus =
  | "pending"       // 待评审
  | "in_review"     // 评审中
  | "rejected"      // 驳回待修改
  | "resubmitted"   // 复审提交
  | "approved"      // 复审通过
  | "archived"      // 归档

export interface Review {
  review_id: string
  tenant_id: string
  repo_id: string
  mr_id: string
  title: string
  status: ReviewStatus
  creator_uid: string
  reviewer_uids: string[]
  source_branch: string
  target_branch: string
  commit_hash: string
  sonar_passed: boolean
  defect_count: number
  created_at: string
  updated_at: string
  archived_at?: string
}

export interface ListReviewsReq {
  tenant_id?: string
  status?: ReviewStatus
  repo_id?: string
  creator_uid?: string
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
