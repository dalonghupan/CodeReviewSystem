import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// ==================== 认证工具 ====================

export const TOKEN_KEY = "cr_system_token"
export const USER_KEY = "cr_system_user"

export interface UserInfo {
  uid: string
  tenant_id: string
  username: string
  avatar?: string
  roles: string[]
}

export function getToken(): string | null {
  if (typeof window === "undefined") return null
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function removeToken() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

export function getStoredUser(): UserInfo | null {
  if (typeof window === "undefined") return null
  try {
    const raw = localStorage.getItem(USER_KEY)
    return raw ? JSON.parse(raw) : null
  } catch {
    return null
  }
}

export function setStoredUser(user: UserInfo) {
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

// ==================== 分页工具 ====================

export interface DataTablePagination {
  page: number
  pageSize: number
  total: number
  totalPages: number
}

/** 将 API 返回的 snake_case 分页转换为 DataTable 所需格式 */
export function toDataTablePagination(p: {
  page: number
  page_size: number
  total: number
  total_pages: number
} | undefined): DataTablePagination | undefined {
  if (!p) return undefined
  return {
    page: p.page,
    pageSize: p.page_size,
    total: p.total,
    totalPages: p.total_pages,
  }
}
