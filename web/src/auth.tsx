import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { getMe, login as loginRequest, logout as logoutRequest, type User } from './api.ts'
import { ensureHydroGlobals } from './upstream/hydro-globals.ts'

type AuthContextValue = {
  user: User | null
  loading: boolean
  login: (username: string, password: string) => Promise<User>
  logout: () => Promise<void>
  refresh: () => Promise<User | null>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    getMe()
      .then((nextUser) => {
        ensureHydroGlobals(nextUser.id)
        setUser(nextUser)
      })
      .catch(() => setUser(null))
      .finally(() => setLoading(false))
  }, [])

  async function login(username: string, password: string) {
    const nextUser = await loginRequest(username, password)
    ensureHydroGlobals(nextUser.id)
    setUser(nextUser)
    return nextUser
  }

  async function logout() {
    await logoutRequest()
    setUser(null)
  }

  async function refresh() {
    try {
      const nextUser = await getMe()
      ensureHydroGlobals(nextUser.id)
      setUser(nextUser)
      return nextUser
    } catch {
      setUser(null)
      return null
    }
  }

  return <AuthContext.Provider value={{ user, loading, login, logout, refresh }}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}
