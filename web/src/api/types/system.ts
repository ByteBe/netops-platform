export interface UserItem {
  id: number
  username: string
  employee_no: string
  email: string
  role: string
  status: string
  created_at: string
}

export interface AIConfig {
  id: number
  name: string
  provider: string
  api_key: string
  base_url: string
  model: string
  enable: boolean
}

export interface MCPAgent {
  id: number
  name: string
  server_url: string
  endpoint: string
  auth_token: string
  enable: boolean
}

export interface EmailConfig {
  smtp_host: string
  smtp_port: number
  smtp_user: string
  smtp_pass: string
  from: string
  enable: boolean
}
