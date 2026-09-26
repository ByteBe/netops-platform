export interface LinkTask {
  id: number
  name: string
  target: string
  method: 'cmd' | 'tcp' | 'icmp'
  port: number
  interval: number
  timeout: number
  color: string
  enabled: boolean
  status: string
  rt: number
  loss: number
  last_ts: number
  message: string
  remark: string
  group_id: number
}

export interface LinkHistory {
  task: LinkTask
  rt: [number, number][]
  loss: number
  status: string
}
