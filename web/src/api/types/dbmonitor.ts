export interface DBInstance {
  id: number
  name: string
  type: string
  host: string
  port: number
  user: string
  password: string
  interval: number
  enable: boolean
  remark: string
  status?: string
}

export interface DBSnap {
  id: number
  name: string
  type: string
  status: string
  conns: number
  cpu: number
  mem: number
  ts: string
}
