import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import type { TrafficRule } from './types/traffic'

export function listRules() {
  return getEnc<TrafficRule[]>('/traffic/rules')
}

export function createRule(data: Partial<TrafficRule>) {
  return postEnc('/traffic/rules', data)
}

export function updateRule(id: number, data: Partial<TrafficRule>) {
  return putEnc(`/traffic/rules/${id}`, data)
}

export function deleteRule(id: number) {
  return delEnc(`/traffic/rules/${id}`)
}

export function getTraffic(ruleId: number, hours = 1) {
  return getEnc(`/traffic/rules/${ruleId}/data?hours=${hours}`)
}
