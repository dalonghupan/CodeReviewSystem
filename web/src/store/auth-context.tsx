"use client"

import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  type ReactNode,
} from "react"
import api from "@/api/client"
import {
  getToken,
  setToken,
  removeToken,
  getStoredUser,
  setStoredUser,
  type UserInfo,
} from "@/lib/utils"

// ==================== 类型定义 ====================

interface LoginParams {
  username: string
  password: string
  tenant_id: string
}

interface AuthContextValue {
  user: UserInfo | null
  token: string | null
  isLoading: boolean
  login: (params: LoginParams) => Promise<void>
  logout: () => void
  isAuthenticated: boolean
}

// ==================== Context ====================

const AuthContext = createContext<AuthContextValue | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<UserInfo | null>(null)
  const [token, setTokenState] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  // 初始化：从 localStorage 恢复登录态
  useEffect(() => {
    const savedToken = getToken()
    const savedUser = getStoredUser()
    if (savedToken && savedUser) {
      setTokenState(savedToken)
      setUser(savedUser)
    }
    setIsLoading(false)
  }, [])

  const login = useCallback(async (params: LoginParams) => {
    const res = await api.post<{ token: string; user: UserInfo }>(
      "/api/v1/auth/login",
      params,
    )
    const { token: newToken, user: userInfo } = res.data
    setToken(newToken)
    setTokenState(newToken)
    setUser(userInfo)
    setStoredUser(userInfo)
  }, [])

  const logout = useCallback(() => {
    removeToken()
    setTokenState(null)
    setUser(null)
  }, [])

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        isLoading,
        login,
        logout,
        isAuthenticated: !!token && !!user,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth 必须在 AuthProvider 内使用")
  return ctx
}
