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

function syncHydroGlobals(nextUser: User | null) {
  ensureHydroGlobals(nextUser?.id ?? 0, {
    viewLang: nextUser?.viewLang,
    fontFamily: nextUser?.fontFamily,
    codeFontFamily: nextUser?.codeFontFamily,
    codeLang: nextUser?.codeLang,
    codeTemplate: nextUser?.codeTemplate,
    skipAnimate: nextUser?.skipAnimate,
  })
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    getMe()
      .then((nextUser) => {
        syncHydroGlobals(nextUser)
        setUser(nextUser)
      })
      .catch(() => {
        syncHydroGlobals(null)
        setUser(null)
      })
      .finally(() => setLoading(false))
  }, [])

  async function login(username: string, password: string, tfa = '') {
    const nextUser = await loginRequest(username, password, tfa)
    syncHydroGlobals(nextUser)
    setUser(nextUser)
    return nextUser
  }

  async function passkeyLogin(username: string) {
    const begin = await beginWebAuthnLogin(username)
    const credential = await startAuthentication({ optionsJSON: begin.options.publicKey })
    const nextUser = await finishWebAuthnLogin(begin.token, credential)
    syncHydroGlobals(nextUser)
    setUser(nextUser)
    return nextUser
  }

  async function logout() {
    await logoutRequest()
    syncHydroGlobals(null)
    setUser(null)
  }

  async function refresh() {
    try {
      const nextUser = await getMe()
      syncHydroGlobals(nextUser)
      setUser(nextUser)
      return nextUser
    } catch {
      syncHydroGlobals(null)
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
