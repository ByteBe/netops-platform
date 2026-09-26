import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'
import type { UserItem, AIConfig, MCPAgent, EmailConfig } from './types/system'

// 用户
export function listUsers() {
  return getEnc<UserItem[]>('/system/users')
}

export function createUser(data: Partial<UserItem>) {
  return postEnc('/system/users', data)
}

export function updateUser(id: number, data: Partial<UserItem>) {
  return putEnc(`/system/users/${id}`, data)
}

export function deleteUser(id: number) {
  return delEnc(`/system/users/${id}`)
}

// AI 配置
export function listAI() {
  return getEnc<AIConfig[]>('/system/ai/configs')
}

export function saveAI(data: Partial<AIConfig>) {
  return data.id ? putEnc(`/system/ai/configs/${data.id}`, data) : postEnc('/system/ai/configs', data)
}

export function deleteAI(id: number) {
  return delEnc(`/system/ai/configs/${id}`)
}

// MCP Agent
export function listMCP() {
  return getEnc<MCPAgent[]>('/system/mcp/agents')
}

export function saveMCP(data: Partial<MCPAgent>) {
  return data.id ? putEnc(`/system/mcp/agents/${data.id}`, data) : postEnc('/system/mcp/agents', data)
}

export function deleteMCP(id: number) {
  return delEnc(`/system/mcp/agents/${id}`)
}

export function testMCP(id: number) {
  return postEnc<{ ok: boolean; output: string }>(`/system/mcp/agents/${id}/test`, {})
}

export function getMCPInfo() {
  return getEnc<{ server: string; tools_endpoint: string; call_endpoint: string; tools: string[] }>('/system/mcp/info')
}

// 邮箱
export function getEmail() {
  return getEnc<EmailConfig>('/system/email')
}

export function saveEmail(data: Partial<EmailConfig>) {
  return putEnc('/system/email', data)
}

// 审计日志
export function listAudit(limit = 50) {
  return getEnc(`/system/audit?limit=${limit}`)
}
