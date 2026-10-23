// 通用工具纯函数单测（node 环境，无 DOM 依赖）
import { describe, it, expect } from 'vitest'
import {
  pickColor,
  fmtBps,
  fmtBytes,
  fmtDuration,
  fmtTime,
  validatePasswordStrength,
  PALETTE
} from '../index'

describe('pickColor', () => {
  it('负数索引按周期回绕', () => {
    expect(pickColor(-1)).toBe(PALETTE[PALETTE.length - 1])
  })
  it('索引取模不越界', () => {
    expect(pickColor(0)).toBe(PALETTE[0])
    expect(pickColor(PALETTE.length)).toBe(PALETTE[0])
  })
})

describe('fmtBps', () => {
  it('0/负数 → 0 bps', () => {
    expect(fmtBps(0)).toBe('0 bps')
    expect(fmtBps(-5)).toBe('0 bps')
    expect(fmtBps(NaN)).toBe('0 bps')
  })
  it('边界单位换算', () => {
    expect(fmtBps(999)).toBe('999 bps')
    expect(fmtBps(1000)).toBe('1.00 Kbps')
    expect(fmtBps(1_000_000)).toBe('1.00 Mbps')
    expect(fmtBps(1_500_000)).toBe('1.50 Mbps')
    expect(fmtBps(1_000_000_000)).toBe('1.00 Gbps')
  })
})

describe('fmtBytes', () => {
  it('0/负数 → 0 B', () => {
    expect(fmtBytes(0)).toBe('0 B')
    expect(fmtBytes(-1)).toBe('0 B')
  })
  it('1024 进制换算', () => {
    expect(fmtBytes(1024)).toBe('1.00 KB')
    expect(fmtBytes(1024 * 1024)).toBe('1.00 MB')
    expect(fmtBytes(1024 * 1024 * 1024)).toBe('1.00 GB')
  })
})

describe('fmtDuration', () => {
  it('边界与分层', () => {
    expect(fmtDuration(0)).toBe('-')
    expect(fmtDuration(30)).toBe('0分')
    expect(fmtDuration(3600)).toBe('1小时0分')
    expect(fmtDuration(86400 + 7200)).toBe('1天2小时')
  })
})

describe('fmtTime', () => {
  it('空值 → -', () => {
    expect(fmtTime(null)).toBe('-')
    expect(fmtTime('')).toBe('-')
    expect(fmtTime(undefined)).toBe('-')
  })
  it('时间戳 → 本地字符串', () => {
    const ts = new Date(2026, 9, 24, 8, 5, 9).getTime() // 2026-10-24 08:05:09
    expect(fmtTime(ts)).toBe('2026-10-24 08:05:09')
  })
  it('非法值原样返回', () => {
    expect(fmtTime('abc')).toBe('abc')
  })
})

describe('validatePasswordStrength', () => {
  it('弱口令逐项拦截', () => {
    expect(validatePasswordStrength('short')).not.toBeNull()
    expect(validatePasswordStrength('abcdefghijklm')).toContain('大写')
    expect(validatePasswordStrength('ABCDEFGHIJKLM')).toContain('小写')
    expect(validatePasswordStrength('Abcdefghijklm')).toContain('数字')
    expect(validatePasswordStrength('Abcdefghijkl1')).toContain('符号')
  })
  it('强口令通过', () => {
    expect(validatePasswordStrength('Abcdefghijkl1!')).toBeNull()
  })
})
