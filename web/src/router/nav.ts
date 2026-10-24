// 左侧导航元数据
// 自动扫描 views/ 目录，新增页面无需手动注册菜单
// 如需自定义标题/图标，在下方 overrides 中补充即可
// 图标直接引用组件对象（按需打包 + 动态组件可用），勿用字符串名
import type { Component } from 'vue'
import {
  Bell,
  Box,
  Coin,
  Connection,
  Cpu,
  DataBoard,
  Document,
  Folder,
  FolderOpened,
  Grid,
  List,
  MagicStick,
  MapLocation,
  Menu,
  Monitor,
  Service,
  Setting,
  Share,
  Ship,
  Star,
  Tools,
  TrendCharts
} from '@element-plus/icons-vue'

export interface NavMeta {
  title: string
  icon: Component
  hidden?: boolean
}

// 手动覆盖（可选）：key 为 views 下的目录名
const overrides: Record<string, NavMeta> = {
  dashboard: { title: 'nav.dashboard', icon: DataBoard, hidden: true },
  linkdetect: { title: 'nav.linkdetect', icon: Connection },
  topology: { title: 'nav.topology', icon: Share },
  monitor: { title: 'nav.monitor', icon: Cpu },
  resource: { title: 'nav.resource', icon: FolderOpened },
  traffic: { title: 'nav.traffic', icon: TrendCharts },
  container: { title: 'nav.container', icon: Box },
  k8s: { title: 'nav.k8s', icon: Ship },
  dbmonitor: { title: 'nav.dbmonitor', icon: Coin },
  nginxmon: { title: 'nav.nginx', icon: Service },
  report: { title: 'nav.report', icon: Document },
  scriptgen: { title: 'nav.scriptgen', icon: MagicStick },
  ipam: { title: 'nav.ipam', icon: MapLocation },
  subnet: { title: 'nav.subnet', icon: Grid },
  system: { title: 'nav.system', icon: Setting },
  dbmigrate: { title: 'nav.dbmigrate', icon: Coin },
  distributed: { title: 'nav.distributed', icon: Share },
  alert: { title: 'nav.alert', icon: Bell },
  bgpmon: { title: 'nav.bgp', icon: Connection },
  tasklog: { title: 'nav.tasklog', icon: List },
  devicelog: { title: 'nav.devicelog', icon: Document },
  aiinspect: { title: 'nav.aiinspect', icon: MagicStick }
}

// 自动扫描 views 目录，收集所有模块名
const viewModules = import.meta.glob('../views/**/index.vue', { eager: false })
const autoNames: string[] = []
for (const p in viewModules) {
  const rel = p.replace('../views/', '').replace('/index.vue', '')
  const seg = rel.split('/')
  autoNames.push(seg[seg.length - 1])
}

// 默认图标池（按顺序分配）
const defaultIcons: Component[] = [Menu, List, Star, Bell, Monitor, Folder, Setting, Tools]

export const navMeta: Record<string, NavMeta> = {}
let iconIdx = 0
for (const name of autoNames) {
  if (name === 'login' || name === 'setup' || name === 'profile') continue
  if (overrides[name]) {
    navMeta[name] = overrides[name]
  } else {
    navMeta[name] = { title: name, icon: defaultIcons[iconIdx++ % defaultIcons.length] }
  }
}
