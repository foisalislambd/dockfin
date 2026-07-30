import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { api, type Team, type User } from '../lib/api'

type AuthState = {
  user: User | null
  team: Team | null
  teams: Team[]
  loading: boolean
  refresh: () => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [team, setTeam] = useState<Team | null>(null)
  const [teams, setTeams] = useState<Team[]>([])
  const [loading, setLoading] = useState(true)

  const refresh = async () => {
    try {
      const me = await api.me()
      setUser(me.user)
      setTeam(me.team)
      setTeams(me.teams || [])
    } catch {
      setUser(null)
      setTeam(null)
      setTeams([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
    const onUnauth = () => {
      setUser(null)
      setTeam(null)
      setTeams([])
      setLoading(false)
    }
    window.addEventListener('goolify:unauthorized', onUnauth)
    return () => window.removeEventListener('goolify:unauthorized', onUnauth)
  }, [])

  const logout = async () => {
    try {
      await api.logout()
    } catch {
      /* clear local session even if API fails */
    }
    setUser(null)
    setTeam(null)
    setTeams([])
  }

  return (
    <AuthContext.Provider value={{ user, team, teams, loading, refresh, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth outside provider')
  return ctx
}
