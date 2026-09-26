// 网络设备 SVG 图标库 - 专业 iconfont 风格（2D 内联 SVG / 3D Canvas 纹理通用）
// viewBox=0 0 64 64，白色线条风格

export interface IconDef {
  viewBox: string
  body: string
  bg: string
}

// 路由器：圆形+四向箭头（参考您截图的样式）
const routerIcon: IconDef = {
  viewBox: '0 0 64 64',
  bg: '#2f6bff',
  body: `
    <circle cx="32" cy="32" r="22" fill="none" stroke="#ffffff" stroke-width="2.5"/>
    <path d="M32 20 L32 44 M32 20 L27 25 M32 20 L37 25" stroke="#ffffff" stroke-width="2.2" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
    <path d="M32 44 L32 20 M32 44 L27 39 M32 44 L37 39" stroke="#ffffff" stroke-width="2.2" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
    <path d="M20 32 L44 32 M20 32 L25 27 M20 32 L25 37" stroke="#ffffff" stroke-width="2.2" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
    <path d="M44 32 L20 32 M44 32 L39 27 M44 32 L39 37" stroke="#ffffff" stroke-width="2.2" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
  `
}

// 交换机：机架设备+网口
const switchIcon: IconDef = {
  viewBox: '0 0 64 64',
  bg: '#22c55e',
  body: `
    <rect x="10" y="22" width="44" height="20" rx="2" fill="none" stroke="#ffffff" stroke-width="2.5"/>
    <rect x="16" y="28" width="6" height="6" rx="1" fill="#ffffff"/>
    <rect x="25" y="28" width="6" height="6" rx="1" fill="#ffffff"/>
    <rect x="34" y="28" width="6" height="6" rx="1" fill="#ffffff"/>
    <rect x="43" y="28" width="6" height="6" rx="1" fill="#ffffff"/>
  `
}

// 防火墙：盾牌+叉号
const firewallIcon: IconDef = {
  viewBox: '0 0 64 64',
  bg: '#f59e0b',
  body: `
    <path d="M32 8 L50 15 V30 C50 42 41 50 32 56 C23 50 14 42 14 30 V15 Z"
      fill="none" stroke="#ffffff" stroke-width="2.5" stroke-linejoin="round"/>
    <line x1="25" y1="25" x2="39" y2="39" stroke="#ffffff" stroke-width="2.5" stroke-linecap="round"/>
    <line x1="39" y1="25" x2="25" y2="39" stroke="#ffffff" stroke-width="2.5" stroke-linecap="round"/>
  `
}

// 服务器：机架堆叠
const serverIcon: IconDef = {
  viewBox: '0 0 64 64',
  bg: '#8b5cf6',
  body: `
    <rect x="14" y="10" width="36" height="13" rx="2" fill="none" stroke="#ffffff" stroke-width="2.2"/>
    <rect x="14" y="26" width="36" height="13" rx="2" fill="none" stroke="#ffffff" stroke-width="2.2"/>
    <rect x="14" y="42" width="36" height="13" rx="2" fill="none" stroke="#ffffff" stroke-width="2.2"/>
    <circle cx="20" cy="16.5" r="1.5" fill="#ffffff"/>
    <circle cx="20" cy="32.5" r="1.5" fill="#ffffff"/>
    <circle cx="20" cy="48.5" r="1.5" fill="#ffffff"/>
    <line x1="25" y1="16.5" x2="44" y2="16.5" stroke="#ffffff" stroke-width="1.5"/>
    <line x1="25" y1="32.5" x2="44" y2="32.5" stroke="#ffffff" stroke-width="1.5"/>
    <line x1="25" y1="48.5" x2="44" y2="48.5" stroke="#ffffff" stroke-width="1.5"/>
  `
}

// 其他设备：地球
const otherIcon: IconDef = {
  viewBox: '0 0 64 64',
  bg: '#64748b',
  body: `
    <circle cx="32" cy="32" r="22" fill="none" stroke="#ffffff" stroke-width="2.5"/>
    <ellipse cx="32" cy="32" rx="10" ry="22" fill="none" stroke="#ffffff" stroke-width="1.8"/>
    <line x1="10" y1="32" x2="54" y2="32" stroke="#ffffff" stroke-width="1.8"/>
  `
}

export const deviceIcons: Record<string, IconDef> = {
  router: routerIcon,
  switch: switchIcon,
  firewall: firewallIcon,
  server: serverIcon,
  other: otherIcon
}

export function getDeviceSvg(type: string, size = 64): string {
  const icon = deviceIcons[type] || otherIcon
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${icon.viewBox}" width="${size}" height="${size}">${icon.body}</svg>`
}

export function getDeviceSvgDataUrl(type: string): string {
  const icon = deviceIcons[type] || otherIcon
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${icon.viewBox}" width="64" height="64">${icon.body}</svg>`
  return 'data:image/svg+xml;base64,' + btoa(unescape(encodeURIComponent(svg)))
}
