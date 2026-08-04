import api from "@/api/client"
import type { Repo, ListReposReq, ListReposResp } from "@/api/types/repo"

/** 获取仓库列表 */
export async function listRepos(params?: ListReposReq): Promise<ListReposResp> {
  const res = await api.get<ListReposResp>("/api/v1/repos", { params })
  return res.data
}

/** 获取仓库详情 */
export async function getRepo(repoId: string): Promise<Repo> {
  const res = await api.get<Repo>(`/api/v1/repos/${repoId}`)
  return res.data
}

/** 绑定仓库 */
export async function bindRepo(data: {
  tenant_id: string
  name: string
  platform: string
  repo_url: string
  auth_id: string
}): Promise<Repo> {
  const res = await api.post<Repo>("/api/v1/repos", data)
  return res.data
}

/** 删除仓库 */
export async function deleteRepo(repoId: string): Promise<void> {
  await api.delete(`/api/v1/repos/${repoId}`)
}
