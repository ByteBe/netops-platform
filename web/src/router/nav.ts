// 左侧导航元数据（对应 views/<模块>/ 目录，图标来自 @element-plus/icons-vue）
export interface NavMeta {
  title: string // i18n key
  icon: string
  hidden?: boolean
}

export const navMeta: Record<string, NavMeta> = {
  dashboard: { title: 'nav.dashboard', icon: 'DataBoard', hidden: true },
  linkdetect: { title: 'nav.linkdetect', icon: 'Connection' },
  topology: { title: 'nav.topology', icon: 'Share' },
  monitor: { title: 'nav.monitor', icon: 'Cpu' },
  resource: { title: 'nav.resource', icon: 'FolderOpened' },
  traffic: { title: 'nav.traffic', icon: 'TrendCharts' },
  container: { title: 'nav.container', icon: 'Box' },
  k8s: { title: 'nav.k8s', icon: 'Ship' },
  dbmonitor: { title: 'nav.dbmonitor', icon: 'Coin' },
  report: { title: 'nav.report', icon: 'Document' },
  scriptgen: { title: 'nav.scriptgen', icon: 'MagicStick' },
  ipam: { title: 'nav.ipam', icon: 'MapLocation' },
  subnet: { title: '子网计算', icon: 'Grid' },
  system: { title: 'nav.system', icon: 'Setting' }
}
