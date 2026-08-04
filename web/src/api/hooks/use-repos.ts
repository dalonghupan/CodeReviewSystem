"use client"

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { listRepos, getRepo, bindRepo, deleteRepo } from "@/api/services/repo"
import type { ListReposReq } from "@/api/types/repo"

export const repoKeys = {
  all: ["repos"] as const,
  list: (params?: ListReposReq) => ["repos", "list", params] as const,
  detail: (id: string) => ["repos", id] as const,
}

/** 仓库列表查询 */
export function useRepos(params?: ListReposReq) {
  return useQuery({
    queryKey: repoKeys.list(params),
    queryFn: () => listRepos(params),
  })
}

/** 仓库详情查询 */
export function useRepo(repoId: string) {
  return useQuery({
    queryKey: repoKeys.detail(repoId),
    queryFn: () => getRepo(repoId),
    enabled: !!repoId,
  })
}

/** 绑定仓库 */
export function useBindRepo() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: bindRepo,
    onSuccess: () => qc.invalidateQueries({ queryKey: repoKeys.all }),
  })
}

/** 删除仓库 */
export function useDeleteRepo() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteRepo(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: repoKeys.all }),
  })
}
