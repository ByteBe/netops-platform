import { postEnc, getEnc } from '@/utils/request'

export async function checkSetup() {
  return getEnc<{ need_setup: boolean }>('/setup/status')
}

export async function initDB(data: Record<string, unknown>) {
  return postEnc('/setup/db', data)
}

export async function initTSDB(data: Record<string, unknown>) {
  return postEnc('/setup/tsdb', data)
}

export async function initAdmin(data: Record<string, unknown>) {
  return postEnc('/setup/admin', data)
}
