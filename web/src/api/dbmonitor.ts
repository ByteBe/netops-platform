import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import type { DBInstance } from './types/dbmonitor'

export function getTypes() {
  return getEnc<Array<{ label: string; value: string }>>('/dbmonitor/types')
}

export function list() {
  return getEnc<DBInstance[]>('/dbmonitor/instances')
}

export function create(data: Partial<DBInstance>) {
  return postEnc('/dbmonitor/instances', data)
}

export function update(id: number, data: Partial<DBInstance>) {
  return putEnc(`/dbmonitor/instances/${id}`, data)
}

export function remove(id: number) {
  return delEnc(`/dbmonitor/instances/${id}`)
}

export function snapshots() {
  return getEnc('/dbmonitor/snapshots')
}
