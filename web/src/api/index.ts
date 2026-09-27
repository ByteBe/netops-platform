// 通用 API 客户端：按模块名自动构造 URL
// 新增模块时无需创建 api/*.ts 文件，直接 useApi('模块名') 即可
import { getEnc, postEnc, putEnc, delEnc } from '@/utils/request'

export function useApi(module: string) {
  const base = `/${module}`
  return {
    list: <T = unknown>(params?: Record<string, unknown>) =>
      getEnc<T>(`${base}/list`, { params }),
    get: <T = unknown>(id: number | string) =>
      getEnc<T>(`${base}/${id}`),
    create: <T = unknown>(data: unknown) =>
      postEnc<T>(`${base}`, data),
    update: <T = unknown>(id: number | string, data: unknown) =>
      putEnc<T>(`${base}/${id}`, data),
    remove: <T = unknown>(id: number | string) =>
      delEnc<T>(`${base}/${id}`),
    custom: <T = unknown>(action: string, data?: unknown) =>
      postEnc<T>(`${base}/${action}`, data ?? {}),
    getCustom: <T = unknown>(action: string, params?: Record<string, unknown>) =>
      getEnc<T>(`${base}/${action}`, { params }),
    raw: { getEnc, postEnc, putEnc, delEnc }
  }
}
