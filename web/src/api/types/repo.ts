// 仓库相关类型定义

export interface Repo {
  repo_id: string
  tenant_id: string
  name: string
  platform: "gitlab" | "gitee" | "github"
  repo_url: string
  default_branch: string
  description: string
  auth_id: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface ListReposReq {
  tenant_id?: string
  platform?: string
  page?: number
  page_size?: number
}

export interface ListReposResp {
  items: Repo[]
  pagination: PaginationResp
}

export interface PaginationResp {
  total: number
  page: number
  page_size: number
  total_pages: number
}
