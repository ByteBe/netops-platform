export interface OverviewKPI {
  link_total: number
  link_up: number
  link_down: number
  device_total: number
  device_online: number
  device_offline: number
  db_total: number
  db_up: number
  db_down: number
  user_total: number
  ip_used: number
}

export interface LinkTrend {
  name: string
  color: string
  points: [number, number][]
}

export interface DeviceHealth {
  name: string
  cpu: number
  mem_used: number
}
