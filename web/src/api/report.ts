import { getEnc, postEnc, delEnc } from '@/utils/request'
import type { ReportItem } from './types/report'

export function list() {
  return getEnc<ReportItem[]>('/report/list')
}

export function generate(data: { title?: string }) {
  return postEnc('/report/generate', data)
}

export function remove(id: number) {
  return delEnc(`/report/${id}`)
}

export function sendEmail(id: number, email: string) {
  return postEnc(`/report/${id}/send`, { email })
}
