export interface Subnet {
  id: number
  name: string
  cidr: string
  gateway: string
  vlan: number
  group_id: number
  binding_enabled: boolean
  description: string
  used: number
  total: number
  created_at: string
}

export interface IPItem {
  ip: string
  status: 'used' | 'unused'
  owner_name: string
  mac: string
  office: string
  record_id: number
  bind_device_id: number
  bind_status: string
  bind_log: string
  remark: string
}

export interface BindDevice {
  id: number
  name: string
  ip: string
  ssh_user: string
  ssh_port: number
  auth_type: string
  credential: string
  vendor: string
  enable: boolean
  remark: string
}
