// 仓库相关类型定义（对齐 proto/crsystem/v1/git_adapter.proto 的 RepoInfo，snake_case）

export interface Repo {
  repo_id: string
  tenant_id: string
  /** protojson 枚举字符串：GIT_PLATFORM_GITLAB / GIT_PLATFORM_GITEE / GIT_PLATFORM_GITHUB */
  platform: string
  full_name: string
  default_branch: string
  web_url: string
  sync_status: string
  last_synced_at?: string
  created_at: string
}

export interface ListReposReq {
  tenant_id: string
  /** 平台筛选：1=GitLab 2=Gitee 3=GitHub */
  platform?: number
  keyword?: string
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
