"use client"

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { listComments, createComment, resolveComment, deleteComment } from "@/api/services/comment"
import type { ListCommentsReq, CreateCommentReq } from "@/api/types/comment"

export const commentKeys = {
  all: ["comments"] as const,
  list: (params?: ListCommentsReq) => ["comments", "list", params] as const,
}

/** 评论列表查询 */
export function useComments(params?: ListCommentsReq) {
  return useQuery({
    queryKey: commentKeys.list(params),
    queryFn: () => listComments(params),
  })
}

/** 创建评论 */
export function useCreateComment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateCommentReq) => createComment(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: commentKeys.all }),
  })
}

/** 解决评论 */
export function useResolveComment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (commentId: string) => resolveComment(commentId),
    onSuccess: () => qc.invalidateQueries({ queryKey: commentKeys.all }),
  })
}

/** 删除评论 */
export function useDeleteComment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (commentId: string) => deleteComment(commentId),
    onSuccess: () => qc.invalidateQueries({ queryKey: commentKeys.all }),
  })
}
