export interface TopoDevice {
  id: number
  name: string
  ip: string
  type: string
  x: number
  y: number
  icon: string
  status: string
  group_id: number
}

export interface TopoLink {
  id: number
  source_id: number
  target_id: number
  label: string
}

export interface TopoLinkIP {
  id: number
  link_id: number
  local_ip: string
  remote_ip: string
}
