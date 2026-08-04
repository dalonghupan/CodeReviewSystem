"use client"

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { listReviews, getReview, createReview } from "@/api/services/review"
import type { ListReviewsReq } from "@/api/types/review"

export const reviewKeys = {
  all: ["reviews"] as const,
  list: (params?: ListReviewsReq) => ["reviews", "list", params] as const,
  detail: (id: string) => ["reviews", id] as const,
}

/** 评审列表查询 */
export function useReviews(params?: ListReviewsReq) {
  return useQuery({
    queryKey: reviewKeys.list(params),
    queryFn: () => listReviews(params),
  })
}

/** 评审详情查询 */
export function useReview(reviewId: string) {
  return useQuery({
    queryKey: reviewKeys.detail(reviewId),
    queryFn: () => getReview(reviewId),
    enabled: !!reviewId,
  })
}

/** 创建评审 */
export function useCreateReview() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: createReview,
    onSuccess: () => qc.invalidateQueries({ queryKey: reviewKeys.all }),
  })
}
