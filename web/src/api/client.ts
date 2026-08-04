// API 请求封装 — axios 实例 + Token 拦截 + 统一错误处理
import axios, {
  type AxiosError,
  type InternalAxiosRequestConfig,
} from "axios"
import { getToken, removeToken } from "@/lib/utils"

const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8000"

const api = axios.create({
  baseURL: API_BASE,
  timeout: 15_000,
  headers: { "Content-Type": "application/json" },
})

// 请求拦截：注入 Token
api.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = getToken()
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截：统一错误处理
api.interceptors.response.use(
  (res) => res,
  (err: AxiosError<{ message?: string; detail?: string }>) => {
    if (err.response?.status === 401) {
      removeToken()
      if (typeof window !== "undefined") {
        window.location.href = "/login"
      }
    }
    const msg =
      err.response?.data?.message ||
      err.response?.data?.detail ||
      err.message ||
      "请求失败"
    return Promise.reject(new Error(msg))
  },
)

export default api
