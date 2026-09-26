import { getEnc, putEnc } from '@/utils/request'
import type { ProfileInfo, ChangePwdReq } from './types/profile'

export function getProfile() {
  return getEnc<ProfileInfo>('/auth/profile')
}

export function changePassword(data: ChangePwdReq) {
  return putEnc('/auth/change-password', data)
}

export function updateProfile(data: Partial<ProfileInfo>) {
  return putEnc('/auth/profile', data)
}
