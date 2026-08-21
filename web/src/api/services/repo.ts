import api from "@/api/client"
import type { Repo, ListReposReq, ListReposResp } from "@/api/types/repo"

// 后端 HTTP 绑定见 proto/crsystem/v1/git_adapter.proto（前缀 /api/v1/git）
// 注意：分页参数为嵌套 message Pagination，query 绑定须用点号路径 pagination.page

/** 获取仓库列表 */
export async function listRepos(params?: ListReposReq): Promise<ListReposResp> {
  const { page, page_size, ...rest } = params ?? {}
  const res = await api.get<ListReposResp>("/api/v1/git/repos", {
    params: {
      ...rest,
      ...(page != null ? { "pagination.page": page } : {}),
      ...(page_size != null ? { "pagination.page_size": page_size } : {}),
    },
  })
  return res.data
}

/** 绑定仓库（platform 传数值：1=GitLab 2=Gitee 3=GitHub） */
export async function bindRepo(data: {
  tenant_id: string
  auth_id: string
  platform_repo_id: string
  full_name: string
  platform: number
}): Promise<Repo> {
  const res = await api.post<Repo>("/api/v1/git/repos", data)
  return res.data
}

/** 解绑仓库 */
export async function deleteRepo(repoId: string): Promise<void> {
  await api.delete(`/api/v1/git/repos/${repoId}`)
}
