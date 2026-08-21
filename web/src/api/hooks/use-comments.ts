"use client"

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  getComments,
  addComment,
  replyComment,
  updateComment,
  deleteComment,
} from "@/api/services/comment"
import type {
  GetCommentsReq,
  AddCommentReq,
  ReplyCommentReq,
  UpdateCommentReq,
} from "@/api/types/comment"

export const commentKeys = {
  all: ["comments"] as const,
  list: (params: GetCommentsReq) => ["comments", "list", params] as const,
}

/** 评审单评论查询（按文件分组） */
export function useComments(params: GetCommentsReq) {
  return useQuery({
    queryKey: commentKeys.list(params),
    queryFn: () => getComments(params),
    enabled: !!params.review_id,
  })
}

/** 创建行级评论 */
export function useAddComment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: AddCommentReq) => addComment(data),
    onSuccess: (_r, vars) =>
      qc.invalidateQueries({ queryKey: ["comments", "list"] }),
  })
}

/** 回复评论 */
export function useReplyComment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: ReplyCommentReq) => replyComment(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["comments", "list"] }),
  })
}

/** 更新评论 */
export function useUpdateComment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: UpdateCommentReq) => updateComment(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["comments", "list"] }),
  })
}

/** 删除评论 */
export function useDeleteComment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (vars: { reviewId: string; commentId: string }) =>
      deleteComment(vars.reviewId, vars.commentId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["comments", "list"] }),
  })
}
