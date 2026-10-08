// Axios 封装：国密 SM4 请求体加密 / 响应解密 / 统一错误处理
import axios, { type AxiosRequestConfig, type AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'
import { sm4Encrypt, sm4Decrypt, generateSessionKey, sm2EncryptSessionKey, type SessionKey } from '@/utils/crypto'
import { OK } from '@/utils'

export const http = axios.create({ baseURL: '/api/v1', timeout: 30000 })

// 业务响应结构
export interface ApiResult<T = unknown> {
  code: number
  message: string
  data: T
}

// 请求加密参数
interface EncOptions {
  encrypt?: boolean // 是否需要加密请求体（默认有体即加密）
}

// 加密请求
export async function postEnc<T = unknown>(url: string, data: unknown, config: AxiosRequestConfig = {}): Promise<T> {
  const res = await http.post<ApiResult<T>>(url, data, config)
  return res.data.data
}

export async function putEnc<T = unknown>(url: string, data: unknown, config: AxiosRequestConfig = {}): Promise<T> {
  const res = await http.put<ApiResult<T>>(url, data, config)
  return res.data.data
}

export async function getEnc<T = unknown>(url: string, config: AxiosRequestConfig = {}): Promise<T> {
  const res = await http.get<ApiResult<T>>(url, config)
  return res.data.data
}

export async function delEnc<T = unknown>(url: string, config: AxiosRequestConfig = {}): Promise<T> {
  const res = await http.delete<ApiResult<T>>(url, config)
  return res.data.data
}

// ---- 国密登录（特殊流程，不走通用拦截器加密） ----
export interface LoginResult {
  token: string
  username: string
  role: string
  email: string
  employee_no: string
  must_change_pwd: boolean
  expires_in: number
}

export async function gmLogin(username: string, password: string): Promise<{ result: LoginResult; sessionKey: SessionKey }> {
  // 1. 获取 SM2 公钥
  const pub = await getEnc<{ public_key: string }>('/crypto/public-key')
  const sm2PublicKey = pub.public_key
  // 2. 生成 SM4 会话密钥并 SM2 加密
  const sessionKey = generateSessionKey()
  const key = sm2EncryptSessionKey(sm2PublicKey, sessionKey)
  const data = sm4Encrypt(sessionKey, JSON.stringify({ username, password }))
  // 3. 提交登录（返回 SM4 加密信封）
  const raw = await axios.post('/api/v1/auth/login', { key, data }, {
    timeout: 30000,
    headers: {
      'X-Timestamp': String(Math.floor(Date.now() / 1000)),
      'X-Nonce': Math.random().toString(36).substring(2, 15) + Date.now().toString(36)
    }
  })
  const result = parseEnvelope<ApiResult<LoginResult>>(raw.data, sessionKey)
  if (result.code !== OK) {
    throw new Error(result.message || '登录失败')
  }
  return { result: result.data, sessionKey }
}

// 解析 {enc} 信封
export function parseEnvelope<T>(body: unknown, sessionKey: SessionKey): T {
  if (body && typeof body === 'object' && 'enc' in body) {
    const plain = sm4Decrypt(sessionKey, (body as { enc: string }).enc)
    return JSON.parse(plain) as T
  }
  return body as T
}

// ---- 请求拦截：SM4 加密请求体 + 防重放头 ----
http.interceptors.request.use((config) => {
  const auth = useAuthStore()
  if (auth.token) {
    config.headers.Authorization = `Bearer ${auth.token}`
  }
  // 防重放：时间戳 + 随机Nonce
  config.headers['X-Timestamp'] = String(Math.floor(Date.now() / 1000))
  config.headers['X-Nonce'] = Math.random().toString(36).substring(2, 15) + Date.now().toString(36)
  const hasBody = !!config.data && typeof config.data !== 'string'
  if (auth.sessionKey && hasBody) {
    const plain = typeof config.data === 'string' ? config.data : JSON.stringify(config.data)
    config.data = sm4Encrypt(auth.sessionKey, plain)
    config.headers['X-Enc'] = 'sm4'
  }
  return config
})

// ---- 响应拦截：SM4 解密 + 统一处理 ----
http.interceptors.response.use(
  (resp: AxiosResponse) => {
    const auth = useAuthStore()
    let body: ApiResult
    // 响应只要是加密信封（{enc: base64}）就用会话密钥解密，与请求头 X-Enc 无关
    // （后端对所有有会话的响应统一加密，包括 GET 无 body 请求）
    if (auth.sessionKey && resp.data && typeof resp.data === 'object' && 'enc' in resp.data) {
      try {
        const plain = sm4Decrypt(auth.sessionKey, (resp.data as { enc: string }).enc)
        body = JSON.parse(plain) as ApiResult
      } catch {
        body = { code: -1, message: '解密失败', data: null }
      }
    } else {
      body = resp.data as ApiResult
    }
    if (body.code !== OK) {
      if (body.code === 40100) {
        auth.logout()
      } else if (body.code === 40310) {
        // 强制改密：跳转改密页
        /* 强制改密由路由守卫处理 */
      } else {
        // 错误由调用方 catch 处理，不全局弹窗
      }
      return Promise.reject(new Error(body.message))
    }
    // 包装为 ApiResult，便于 getEnc/postEnc 直接取 data
    return { ...resp, data: body } as AxiosResponse
  },
  (error) => {
    const status = error?.response?.status
    if (status === 401) {
      try { useAuthStore().logout() } catch {}
      return Promise.resolve({ data: { code: 40100, data: null } })
    }
    const auth = useAuthStore()
    if (status && error.response?.data) {
      let msg = error.response.data.message || '请求失败'
      // 解密错误响应（若为加密信封）
      if (auth.sessionKey && error.response.data && typeof error.response.data === 'object' && 'enc' in error.response.data) {
        try {
          msg = sm4Decrypt(auth.sessionKey, (error.response.data as { enc: string }).enc)
        } catch {
          /* 忽略 */
        }
      }
      return Promise.reject(new Error(msg))
    }
    return Promise.reject(error)
  }
)

// 供 ElementPlus 表格等使用的原始请求（不额外处理）
export type { EncOptions }
