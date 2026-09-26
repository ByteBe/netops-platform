import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import type { ResourceGroup, ResourceItem } from './types/resource'

export function listGroups() {
  return getEnc<ResourceGroup[]>('/resource/groups')
}

export function createGroup(data: Partial<ResourceGroup>) {
  return postEnc('/resource/groups', data)
}

export function updateGroup(id: number, data: Partial<ResourceGroup>) {
  return putEnc(`/resource/groups/${id}`, data)
}

export function deleteGroup(id: number) {
  return delEnc(`/resource/groups/${id}`)
}

export function listItems(groupId?: number) {
  const url = groupId ? `/resource/items?group_id=${groupId}` : '/resource/items'
  return getEnc<ResourceItem[]>(url)
}
