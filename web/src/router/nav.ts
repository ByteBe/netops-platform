// 左侧导航元数据
// 自动扫描 views/ 目录，新增页面无需手动注册菜单
// 如需自定义标题/图标，在下方 overrides 中补充即可
export interface NavMeta {
  title: string
  icon: string
  hidden?: boolean
}

// 手动覆盖（可选）：key 为 views 下的目录名
const overrides: Record<string, NavMeta> = {
  dashboard: { title: '数据大屏', icon: 'DataBoard', hidden: true },
  linkdetect: { title: '链路检测', icon: 'Connection' },
  topology: { title: '网络拓扑', icon: 'Share' },
  monitor: { title: '设备监控', icon: 'Cpu' },
  resource: { title: '资源管理', icon: 'FolderOpened' },
  traffic: { title: '流量分析', icon: 'TrendCharts' },
  container: { title: '容器监控', icon: 'Box' },
  k8s: { title: 'K8S集群', icon: 'Ship' },
  dbmonitor: { title: '数据库监控', icon: 'Coin' },
  report: { title: '巡检报告', icon: 'Document' },
  scriptgen: { title: '脚本生成', icon: 'MagicStick' },
  ipam: { title: 'IP地址管理', icon: 'MapLocation' },
  subnet: { title: '子网计算', icon: 'Grid' },
  system: { title: '系统管理', icon: 'Setting' },
  dbmigrate: { title: '数据库迁移', icon: 'Coin' }
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
const defaultIcons = ['Menu', 'List', 'Star', 'Bell', 'Monitor', 'Folder', 'Setting', 'Tools']

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
