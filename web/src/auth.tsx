import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { getMe, login as loginRequest, logout as logoutRequest, type User } from './api.ts'
import { ensureHydroGlobals } from './upstream/hydro-globals.ts'

type AuthContextValue = {
  user: User | null
  loading: boolean
  login: (username: string, password: string) => Promise<User>
  logout: () => Promise<void>
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

  return <AuthContext.Provider value={{ user, loading, login, logout }}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}
