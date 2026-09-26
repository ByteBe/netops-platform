// 国密工具：SM2 非对称 + SM4-CBC 对称（与后端 common/crypto 协议对齐）
import { sm2, sm4 } from 'sm-crypto'

export type SessionKey = string // 32位16进制（16字节）

// 生成 SM4 会话密钥（16字节 → 32位hex）
export function generateSessionKey(): SessionKey {
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
}

// 会话密钥 → 字节数组
export function sessionKeyBytes(key: SessionKey): number[] {
  const out: number[] = []
  for (let i = 0; i < 32; i += 2) {
    out.push(parseInt(key.slice(i, i + 2), 16))
  }
  return out
}

// 使用后端 SM2 公钥加密会话密钥（C1C3C2，与后端兼容；明文为16字节原始密钥）
export function sm2EncryptSessionKey(publicKeyHex: string, sessionKey: SessionKey): string {
  return sm2.doEncrypt(sessionKeyBytes(sessionKey), publicKeyHex, 1)
}

// SM4-CBC 加密：返回 base64(IV(16) + 密文)
export function sm4Encrypt(key: SessionKey, plaintext: string | Uint8Array): string {
  const keyArr = sessionKeyBytes(key)
  const iv = new Uint8Array(16)
  crypto.getRandomValues(iv)
  const data = typeof plaintext === 'string' ? utf8ToBytes(plaintext) : plaintext
  const cipher = sm4.encrypt(Array.from(data), keyArr, {
    mode: 'cbc',
    iv: Array.from(iv),
    output: 'array'
  }) as number[]
  const merged = new Uint8Array(16 + cipher.length)
  merged.set(iv, 0)
  merged.set(new Uint8Array(cipher), 16)
  return bytesToBase64(merged)
}

// SM4-CBC 解密 base64(IV + 密文)
export function sm4Decrypt(key: SessionKey, envelope: string): string {
  const keyArr = sessionKeyBytes(key)
  const raw = base64ToBytes(envelope)
  const iv = Array.from(raw.slice(0, 16))
  const ct = Array.from(raw.slice(16))
  const plain = sm4.decrypt(ct, keyArr, { mode: 'cbc', iv, output: 'array' }) as number[]
  return bytesToUtf8(new Uint8Array(plain))
}

// 工具函数
export function utf8ToBytes(s: string): Uint8Array {
  return new TextEncoder().encode(s)
}

export function bytesToUtf8(b: Uint8Array): string {
  return new TextDecoder().decode(b)
}

export function bytesToBase64(b: Uint8Array): string {
  let bin = ''
  for (let i = 0; i < b.length; i++) {
    bin += String.fromCharCode(b[i])
  }
  return btoa(bin)
}

export function base64ToBytes(b64: string): Uint8Array {
  const bin = atob(b64)
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) {
    out[i] = bin.charCodeAt(i)
  }
  return out
}
