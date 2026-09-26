import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import type { TopoDevice, TopoLink } from './types/topology'

export function getDevices() {
  return getEnc<TopoDevice[]>('/topology/devices')
}

export function createDevice(data: Partial<TopoDevice>) {
  return postEnc('/topology/devices', data)
}

export function updateDevice(id: number, data: Partial<TopoDevice>) {
  return putEnc(`/topology/devices/${id}`, data)
}

export function deleteDevice(id: number) {
  return delEnc(`/topology/devices/${id}`)
}

export function getLinks() {
  return getEnc<TopoLink[]>('/topology/links')
}

export function uploadIcon(formData: FormData) {
  return postEnc('/topology/upload', formData)
}
