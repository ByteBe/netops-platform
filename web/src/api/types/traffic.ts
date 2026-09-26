export interface TrafficRule {
  id: number
  name: string
  device_id: number
  interface: string
  bandwidth: number
  direction: string
}

export interface TrafficSample {
  ts: number
  in_bps: number
  out_bps: number
}
