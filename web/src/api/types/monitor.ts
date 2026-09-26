export interface MonitorDevice {
  id: number
  name: string
  ip: string
  type: string
  group_id: number
  community: string
  version: string
  status: string
  cpu: number
  mem: number
  uptime: string
  last_seen: string
}

export interface DeviceSnapshot {
  id: number
  name: string
  ip: string
  cpu: number
  mem_used: number
  uptime: string
  status: string
}
