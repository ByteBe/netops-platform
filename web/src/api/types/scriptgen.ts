export interface ScriptField {
  key: string
  label: string
  type: string
  required: boolean
  placeholder?: string
  options?: string[]
}

export interface ScriptTemplate {
  code: string
  vendor: string
  device_type: string
  category: string
  name: string
  description: string
  fields: ScriptField[]
}

export interface ScriptHistory {
  id: number
  template_code: string
  vendor: string
  device_type: string
  output: string
  created_at: string
}
