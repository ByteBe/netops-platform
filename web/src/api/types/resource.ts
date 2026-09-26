export interface ResourceGroup {
  id: number
  name: string
  description: string
  count: number
}

export interface ResourceItem {
  id: number
  name: string
  ip: string
  type: string
  group_id: number
  status: string
}
