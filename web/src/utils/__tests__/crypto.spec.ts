// 国密工具纯函数单测（SM4 往返 / base64 往返 / 字节转换）
import { describe, it, expect } from 'vitest'
import {
  sessionKeyBytes,
  utf8ToBytes,
  bytesToUtf8,
  bytesToBase64,
  base64ToBytes,
  sm4Encrypt,
  sm4Decrypt
} from '../crypto'

describe('sessionKeyBytes', () => {
  it('32位hex → 16字节', () => {
    const key = '00112233445566778899aabbccddeeff'
    const b = sessionKeyBytes(key)
    expect(b).toHaveLength(16)
    expect(b[0]).toBe(0x00)
    expect(b[15]).toBe(0xff)
  })
  it('越界 hex 自动截断', () => {
    const b = sessionKeyBytes('gg') // 非法 hex
    expect(b[0]).toBeNaN
  })
})

describe('utf8/base64 往返', () => {
  it('UTF-8 编解码往返（含中文与 emoji）', () => {
    const s = '网络运维平台 NetOps 巡检 ✅'
    expect(bytesToUtf8(utf8ToBytes(s))).toBe(s)
  })
  it('base64 往返', () => {
    const raw = utf8ToBytes('NetOps SM4-CBC')
    expect(bytesToUtf8(base64ToBytes(bytesToBase64(raw)))).toBe('NetOps SM4-CBC')
  })
})

describe('SM4-CBC 加解密', () => {
  it('加密解密往返还原明文', () => {
    const key = '00112233445566778899aabbccddeeff'
    const plain = '告警阈值 80% 已触发'
    const env = sm4Encrypt(key, plain)
    expect(env.length).toBeGreaterThan(0)
    expect(sm4Decrypt(key, env)).toBe(plain)
  })
  it('同一明文两次加密密文不同（随机 IV）', () => {
    const key = '00112233445566778899aabbccddeeff'
    const a = sm4Encrypt(key, 'hello')
    const b = sm4Encrypt(key, 'hello')
    expect(a).not.toBe(b)
  })
  it('错误密钥解密必须失败（抛异常而非返回明文）', () => {
    const key = '00112233445566778899aabbccddeeff'
    const other = 'ffeeddccbbaa99887766554433221100'
    const env = sm4Encrypt(key, '机密内容')
    expect(() => sm4Decrypt(other, env)).toThrow()
  })
})
