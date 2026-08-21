// 评论相关类型定义（对齐 proto/crsystem/v1/review.proto，snake_case）

/** protojson 枚举字符串：DEFECT_LEVEL_UNSPECIFIED / FATAL / CRITICAL / MAJOR / MINOR */
export type DefectLevel =
  | "DEFECT_LEVEL_UNSPECIFIED"
  | "DEFECT_LEVEL_FATAL"
  | "DEFECT_LEVEL_CRITICAL"
  | "DEFECT_LEVEL_MAJOR"
  | "DEFECT_LEVEL_MINOR"

export interface CommentInfo {
  comment_id: string
  review_id: string
  file_path: string
  line_num: number
  commit_version: string
  content: string
  defect_level: DefectLevel
  create_uid: string
  create_name: string
  create_avatar: string
  reply_parent_id: string
  replies: CommentInfo[]
  is_isolated: boolean
  created_at: string
  updated_at: string
}

export interface CommentGroup {
  file_path: string
  comments: CommentInfo[]
}

export interface GetCommentsReq {
  review_id: string
  file_path?: string
  commit_version?: string
  include_isolated?: boolean
}

export interface GetCommentsResp {
  file_groups: CommentGroup[]
  total_count: number
  defect_count: number
}

export interface AddCommentReq {
  review_id: string
  file_path: string
  line_num: number
  commit_version: string
  content: string
  /** 枚举数值：0=未标记 1=致命 2=严重 3=一般 4=建议 */
  defect_level?: number
}

export interface ReplyCommentReq {
  review_id: string
  comment_id: string
  content: string
}

export interface UpdateCommentReq {
  review_id: string
  comment_id: string
  content: string
  defect_level?: number
}
