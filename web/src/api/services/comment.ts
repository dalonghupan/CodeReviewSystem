import api from "@/api/client"
import type {
  CommentInfo,
  GetCommentsReq,
  GetCommentsResp,
  AddCommentReq,
  ReplyCommentReq,
  UpdateCommentReq,
} from "@/api/types/comment"

// 后端 HTTP 绑定见 proto/crsystem/v1/review.proto
// 评论为评审单的嵌套资源：/api/v1/reviews/{review_id}/comments
// 注意：后端暂无「解决评论」接口（LLD 未定义 is_resolved 字段），如需请先在 proto 层补充

/** 获取评审单评论（按文件分组） */
export async function getComments(params: GetCommentsReq): Promise<GetCommentsResp> {
  const { review_id, ...query } = params
  const res = await api.get<GetCommentsResp>(`/api/v1/reviews/${review_id}/comments`, {
    params: query,
  })
  return res.data
}

/** 创建行级评论 */
export async function addComment(data: AddCommentReq): Promise<CommentInfo> {
  const { review_id, ...body } = data
  const res = await api.post<CommentInfo>(`/api/v1/reviews/${review_id}/comments`, body)
  return res.data
}

/** 回复评论 */
export async function replyComment(data: ReplyCommentReq): Promise<CommentInfo> {
  const { review_id, comment_id, content } = data
  const res = await api.post<CommentInfo>(
    `/api/v1/reviews/${review_id}/comments/${comment_id}/reply`,
    { content },
  )
  return res.data
}

/** 更新评论（内容 / 缺陷等级） */
export async function updateComment(data: UpdateCommentReq): Promise<CommentInfo> {
  const { review_id, comment_id, ...body } = data
  const res = await api.put<CommentInfo>(
    `/api/v1/reviews/${review_id}/comments/${comment_id}`,
    body,
  )
  return res.data
}

/** 删除评论 */
export async function deleteComment(reviewId: string, commentId: string): Promise<void> {
  await api.delete(`/api/v1/reviews/${reviewId}/comments/${commentId}`)
}
