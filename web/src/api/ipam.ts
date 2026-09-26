import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import type { Subnet, IPItem, BindDevice } from './types/ipam'

export function listSubnets() {
  return getEnc<Subnet[]>('/ipam/subnets')
}

export function createSubnet(data: Partial<Subnet>) {
  return postEnc('/ipam/subnets', data)
}

export function updateSubnet(id: number, data: Partial<Subnet>) {
  return putEnc(`/ipam/subnets/${id}`, data)
}

export function deleteSubnet(id: number) {
  return delEnc(`/ipam/subnets/${id}`)
}

export function listIPs(subnetId: number, params: { status: string; page: number; size: number; keyword: string }) {
  return getEnc<{ total: number; list: IPItem[] }>(
    `/ipam/subnets/${subnetId}/ips?status=${params.status}&page=${params.page}&size=${params.size}&keyword=${encodeURIComponent(params.keyword)}`
  )
}

export function createRecord(data: Record<string, unknown>) {
  return postEnc('/ipam/records', data)
}

export function updateRecord(id: number, data: Record<string, unknown>) {
  return putEnc(`/ipam/records/${id}`, data)
}

export function deleteRecord(id: number) {
  return delEnc(`/ipam/records/${id}`)
}

export function bindRecord(id: number, bindDeviceId: number) {
  return postEnc(`/ipam/records/${id}/bind`, { bind_device_id: bindDeviceId })
}

export function listBindDevices() {
  return getEnc<BindDevice[]>('/ipam/bind-devices')
}

export function createBindDevice(data: Partial<BindDevice>) {
  return postEnc('/ipam/bind-devices', data)
}

export function updateBindDevice(id: number, data: Partial<BindDevice>) {
  return putEnc(`/ipam/bind-devices/${id}`, data)
}

export function deleteBindDevice(id: number) {
  return delEnc(`/ipam/bind-devices/${id}`)
}

export function testBindDevice(id: number) {
  return postEnc<{ ok: boolean; output: string }>(`/ipam/bind-devices/${id}/test`, {})
}

export function importARP(subnetId: number, bindDeviceId: number) {
  return postEnc<{ imported: number; updated: number; arp_count: number }>(
    `/ipam/subnets/${subnetId}/import-arp`, { bind_device_id: bindDeviceId }
  )
}

export function getSettings() {
  return getEnc<{ ipam_binding_enable: boolean }>('/ipam/settings')
}

export function saveSettings(data: Record<string, unknown>) {
  return putEnc('/ipam/settings', data)
}
