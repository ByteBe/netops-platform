import { gmLogin, postEnc, getEnc } from '@/utils/request'
import type { LoginForm, ChangePwdForm, LoginResult } from './types/login'

export async function login(form: LoginForm) {
  return gmLogin(form.username, form.password)
}

export async function changePassword(data: { old_password: string; new_password: string }) {
  return postEnc('/auth/change-password', data)
}

export async function fetchPublicKey() {
  return getEnc<{ public_key: string }>('/crypto/public-key')
}

export type { LoginForm, ChangePwdForm, LoginResult }
