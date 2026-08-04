import api from "@/api/client"
import type { Comment, ListCommentsReq, ListCommentsResp, CreateCommentReq } from "@/api/types/comment"

/** 获取评论列表 */
export async function listComments(params?: ListCommentsReq): Promise<ListCommentsResp> {
  const res = await api.get<ListCommentsResp>("/api/v1/comments", { params })
  return res.data
}

/** 创建评论 */
export async function createComment(data: CreateCommentReq): Promise<Comment> {
  const res = await api.post<Comment>("/api/v1/comments", data)
  return res.data
}

/** 解决评论 */
export async function resolveComment(commentId: string): Promise<void> {
  await api.put(`/api/v1/comments/${commentId}/resolve`)
}

/** 删除评论 */
export async function deleteComment(commentId: string): Promise<void> {
  await api.delete(`/api/v1/comments/${commentId}`)
}
