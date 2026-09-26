import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import type { MonitorDevice } from './types/monitor'

export function getDevices() {
  return getEnc<MonitorDevice[]>('/monitor/devices')
}

export function createDevice(data: Partial<MonitorDevice>) {
  return postEnc('/monitor/devices', data)
}

export function updateDevice(id: number, data: Partial<MonitorDevice>) {
  return putEnc(`/monitor/devices/${id}`, data)
}

export function deleteDevice(id: number) {
  return delEnc(`/monitor/devices/${id}`)
}

export function getSnapshot() {
  return getEnc('/monitor/snapshot')
}
