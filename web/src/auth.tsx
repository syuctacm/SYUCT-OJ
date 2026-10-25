import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { startAuthentication } from '@simplewebauthn/browser'
import {
  beginWebAuthnLogin, finishWebAuthnLogin, getMe, login as loginRequest, logout as logoutRequest, type User,
} from './api.ts'
import { ensureHydroGlobals } from './upstream/hydro-globals.ts'

type AuthContextValue = {
  user: User | null
  loading: boolean
  login: (username: string, password: string, tfa?: string) => Promise<User>
  passkeyLogin: (username: string) => Promise<User>
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

  async function login(username: string, password: string, tfa = '') {
    const nextUser = await loginRequest(username, password, tfa)
    ensureHydroGlobals(nextUser.id)
    setUser(nextUser)
    return nextUser
  }

  async function passkeyLogin(username: string) {
    const begin = await beginWebAuthnLogin(username)
    const credential = await startAuthentication({ optionsJSON: begin.options.publicKey })
    const nextUser = await finishWebAuthnLogin(begin.token, credential)
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

  return <AuthContext.Provider value={{ user, loading, login, passkeyLogin, logout, refresh }}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}
