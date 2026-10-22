// 认证与国密会话状态
import { defineStore } from 'pinia'
import type { SessionKey } from '@/utils/crypto'

export interface UserInfo {
  id: number
  username: string
  email: string
  employee_no: string
  role: string
  status: string
  must_change_pwd: boolean
  last_login_at: string
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: sessionStorage.getItem('np-token') || '',
    sessionKey: (sessionStorage.getItem('np-session-key') || '') as SessionKey,
    sm2PublicKey: sessionStorage.getItem('np-sm2-pub') || '',
    user: null as UserInfo | null
  }),
  getters: {
    isLoggedIn: (s) => !!s.token,
    mustChangePwd: (s) => !!s.user?.must_change_pwd
  },
  actions: {
    saveCrypto(sessionKey: SessionKey, sm2PublicKey: string) {
      this.sessionKey = sessionKey
      this.sm2PublicKey = sm2PublicKey
      sessionStorage.setItem('np-session-key', sessionKey)
      sessionStorage.setItem('np-sm2-pub', sm2PublicKey)
    },
    setLogin(token: string, sessionKey: SessionKey, sm2PublicKey: string) {
      this.token = token
      this.saveCrypto(sessionKey, sm2PublicKey)
      sessionStorage.setItem('np-token', token)
    },
    setUser(user: UserInfo) {
      this.user = user
    },
    logout() {
      this.token = ''
      this.user = null
      this.sessionKey = ''
      sessionStorage.removeItem('np-token')
      sessionStorage.removeItem('np-session-key')
      sessionStorage.removeItem('np-sm2-pub')
    }
  }
})
