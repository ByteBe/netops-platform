import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import type { LinkTask } from './types/linkdetect'

export function getTasks() {
  return getEnc<LinkTask[]>('/linkdetect/tasks')
}

export function createTask(data: Partial<LinkTask>) {
  return postEnc('/linkdetect/tasks', data)
}

export function updateTask(id: number, data: Partial<LinkTask>) {
  return putEnc(`/linkdetect/tasks/${id}`, data)
}

export function deleteTask(id: number) {
  return delEnc(`/linkdetect/tasks/${id}`)
}

export function getTaskHistory(id: number, hours = 1) {
  return getEnc<{ rt: [number, number][] }>(`/linkdetect/tasks/${id}/history?hours=${hours}`)
}
