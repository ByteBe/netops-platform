import { getEnc } from '@/utils/request'
import type { OverviewKPI, LinkTrend, DeviceHealth } from './types/dashboard'

export function getOverview() {
  return getEnc<{
    kpi: OverviewKPI
    links: Array<{ name: string; up: boolean; rtt_ms: number; loss_pct: number }>
    devices: DeviceHealth[]
    dbs: Array<{ name: string; conns: number }>
  }>('/dashboard/overview')
}

export function getLinkTrends(hours = 6) {
  return getEnc<{ series: LinkTrend[] }>(`/dashboard/link-trends?hours=${hours}`)
}

export function getTraffic() {
  return getEnc<{ name: string; in_bps: number; out_bps: number }[]>('/dashboard/traffic')
}
