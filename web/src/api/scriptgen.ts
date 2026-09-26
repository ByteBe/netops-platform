import { getEnc, postEnc } from '@/utils/request'
import type { ScriptTemplate, ScriptHistory } from './types/scriptgen'

export function listTemplates() {
  return getEnc<ScriptTemplate[]>('/scriptgen/templates')
}

export function generate(data: { template_code: string; vendor: string; params: Record<string, string> }) {
  return postEnc<{ output: string }>('/scriptgen/generate', data)
}

export function history() {
  return getEnc<ScriptHistory[]>('/scriptgen/history')
}
