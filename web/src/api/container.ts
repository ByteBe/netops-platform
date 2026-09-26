import { getEnc, putEnc, postEnc } from '@/utils/request'

export function getConfig() {
  return getEnc('/container/config')
}

export function saveConfig(data: Record<string, unknown>) {
  return putEnc('/container/config', data)
}

export function collect() {
  return postEnc('/container/collect', {})
}

export function list() {
  return getEnc('/container/list')
}
