// 评论相关类型定义

export interface Comment {
  comment_id: string
  review_id: string
  file_path: string
  line_num: number
  content: string
  comment_type: "defect" | "suggestion" | "question" | "praise"
  creator_uid: string
  creator_name?: string
  parent_id?: string
  is_resolved: boolean
  created_at: string
  updated_at: string
}

export interface ListCommentsReq {
  review_id?: string
  file_path?: string
  page?: number
  page_size?: number
}

export interface ListCommentsResp {
  items: Comment[]
  pagination: {
    total: number
    page: number
    page_size: number
    total_pages: number
  }
}

export interface CreateCommentReq {
  review_id: string
  file_path: string
  line_num: number
  content: string
  comment_type: "defect" | "suggestion" | "question" | "praise"
  parent_id?: string
}
