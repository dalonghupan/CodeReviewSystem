import api from "@/api/client"
import type { Review, ListReviewsReq, ListReviewsResp } from "@/api/types/review"

/** 获取评审列表 */
export async function listReviews(params?: ListReviewsReq): Promise<ListReviewsResp> {
  const res = await api.get<ListReviewsResp>("/api/v1/reviews", { params })
  return res.data
}

/** 获取评审详情 */
export async function getReview(reviewId: string): Promise<Review> {
  const res = await api.get<Review>(`/api/v1/reviews/${reviewId}`)
  return res.data
}

/** 创建评审 */
export async function createReview(data: {
  tenant_id: string
  repo_id: string
  mr_id: string
  title: string
  reviewer_uids: string[]
}): Promise<Review> {
  const res = await api.post<Review>("/api/v1/reviews", data)
  return res.data
}
